package util

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
	"howett.net/plist"
)

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

const (
	// cards show icons at 48px, this leaves room for high density screens
	iconOutputSize = 192
	maxCarSize     = 1 << 30
)

// AppInfo is what's shown about an app before it's signed.
type AppInfo struct {
	Version string
	Icon    []byte // PNG of at most iconOutputSize pixels per side
	IconErr error  // why there's no icon
}

// ExtractIcon returns the app icon of an IPA as a PNG of at most iconOutputSize pixels per side.
func ExtractIcon(r io.ReaderAt, size int64) ([]byte, error) {
	info, err := ReadAppInfo(r, size)
	if err != nil {
		return nil, err
	}
	return info.Icon, info.IconErr
}

// ReadAppInfo reads an IPA's version and icon. For the icon, loose files are tried first, then the
// compiled asset catalog. It only fails if the file isn't an app at all.
func ReadAppInfo(r io.ReaderAt, size int64) (*AppInfo, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, errors.WithMessage(err, "open zip")
	}
	bundle, err := findBundle(zr.File)
	if err != nil {
		return nil, err
	}

	info := &AppInfo{}
	var names iconNames
	if bundle.plist != nil {
		var plistInfo *infoPlist
		if plistInfo, err = readInfoPlist(bundle.plist); err != nil {
			return nil, err
		}
		info.Version = plistInfo.version()
		names = plistInfo.iconNames()
	}
	info.Icon, info.IconErr = extractIcon(r, bundle, names)
	return info, nil
}

// appBundle is an iOS app (Payload/X.app/Info.plist) or a native Mac app (X.app/Contents/Info.plist).
type appBundle struct {
	mac       bool
	plist     *zip.File
	resources map[string]*zip.File // files directly in the folder holding the icon files
}

func findBundle(files []*zip.File) (*appBundle, error) {
	type candidate struct {
		dir, resourcesDir string
		mac               bool
		plist             *zip.File
	}
	var best *candidate
	for _, f := range files {
		parts := strings.Split(f.Name, "/")
		if parts[0] == "Payload" {
			parts = parts[1:]
		}
		var c candidate
		switch {
		case len(parts) == 2 && strings.HasSuffix(parts[0], ".app") && parts[1] == "Info.plist":
			c = candidate{dir: strings.TrimSuffix(f.Name, "Info.plist")}
			c.resourcesDir = c.dir
		case len(parts) == 3 && strings.HasSuffix(parts[0], ".app") && parts[1] == "Contents" && parts[2] == "Info.plist":
			c = candidate{dir: strings.TrimSuffix(f.Name, "Contents/Info.plist"), mac: true}
			c.resourcesDir = c.dir + "Contents/Resources/"
		default:
			continue
		}
		c.plist = f
		// prefer iOS apps, then the first bundle
		if best == nil || best.mac && !c.mac {
			best = &c
		}
	}
	if best == nil {
		// an IPA without Info.plist still has the usual layout
		for _, f := range files {
			parts := strings.Split(f.Name, "/")
			if len(parts) >= 2 && parts[0] == "Payload" && strings.HasSuffix(parts[1], ".app") {
				best = &candidate{dir: "Payload/" + parts[1] + "/"}
				best.resourcesDir = best.dir
				break
			}
		}
	}
	if best == nil {
		return nil, errors.New("no app bundle found")
	}
	bundle := &appBundle{mac: best.mac, plist: best.plist, resources: map[string]*zip.File{}}
	for _, f := range files {
		if name, ok := strings.CutPrefix(f.Name, best.resourcesDir); ok && name != "" && !strings.Contains(name, "/") {
			bundle.resources[name] = f
		}
	}
	return bundle, nil
}

func extractIcon(r io.ReaderAt, bundle *appBundle, names iconNames) ([]byte, error) {
	var img image.Image
	var errs []string
	if !bundle.mac {
		var err error
		if img, err = looseIcon(bundle.resources, names.files); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if img == nil || img.Bounds().Dx() < iconOutputSize {
		if car, ok := bundle.resources["Assets.car"]; ok {
			carImg, err := carIcon(r, car, append(names.assets, "AppIcon"))
			if err != nil {
				errs = append(errs, "Assets.car: "+err.Error())
			} else if img == nil || carImg.Bounds().Dx() > img.Bounds().Dx() {
				img = carImg
			}
		}
	}
	if img == nil && bundle.mac {
		if f, ok := bundle.resources[names.icns]; ok && names.icns != "" {
			var err error
			if img, err = icnsIcon(f); err != nil {
				errs = append(errs, names.icns+": "+err.Error())
			}
		}
	}
	if img == nil {
		if len(errs) == 0 {
			return nil, errors.New("no icon found")
		}
		return nil, errors.New(strings.Join(errs, "; "))
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, shrinkImage(img, iconOutputSize)); err != nil {
		return nil, errors.WithMessage(err, "encode png")
	}
	return buf.Bytes(), nil
}

// icnsIcon returns the largest PNG in a Mac .icns file. Modern files store every size from
// 128px up as PNG; older formats aren't supported.
func icnsIcon(f *zip.File) (image.Image, error) {
	data, err := readZipFile(f)
	if err != nil {
		return nil, err
	}
	if len(data) < 8 || string(data[:4]) != "icns" {
		return nil, errors.New("not an icns file")
	}
	var best []byte
	bestArea := 0
	for pos := 8; pos+8 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[pos+4:]))
		if n < 8 || pos+n > len(data) {
			return nil, errors.New("icns entry out of bounds")
		}
		entry := data[pos+8 : pos+n]
		pos += n
		if !bytes.HasPrefix(entry, pngSignature) {
			continue
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(entry))
		if err != nil || cfg.Width > maxIconSide || cfg.Height > maxIconSide {
			continue
		}
		if area := cfg.Width * cfg.Height; area > bestArea {
			best, bestArea = entry, area
		}
	}
	if best == nil {
		return nil, errors.New("no png in icns")
	}
	return png.Decode(bytes.NewReader(best))
}

func looseIcon(files map[string]*zip.File, prefixes []string) (image.Image, error) {
	best, bestArea := pickIcon(files, prefixes)
	if best == nil {
		best, bestArea = pickIcon(files, []string{"AppIcon", "Icon"})
	}
	if best == nil || bestArea == 0 {
		return nil, errors.New("no icon file found")
	}
	data, err := readZipFile(best)
	if err != nil {
		return nil, err
	}
	return DecodeIconPNG(data)
}

func carIcon(r io.ReaderAt, f *zip.File, names []string) (image.Image, error) {
	if f.UncompressedSize64 > maxCarSize {
		return nil, errors.New("asset catalog too large")
	}
	if f.Method == zip.Store {
		off, err := f.DataOffset()
		if err != nil {
			return nil, err
		}
		size := int64(f.UncompressedSize64)
		return ExtractCarIcon(io.NewSectionReader(r, off, size), size, names)
	}
	// the catalog is read at random offsets, and can be tens of megabytes
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	tmp, err := os.CreateTemp("", "signtools-*.car")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	size, err := io.Copy(tmp, io.LimitReader(rc, maxCarSize+1))
	if err != nil {
		return nil, errors.WithMessage(err, "extract asset catalog")
	}
	if size > maxCarSize {
		return nil, errors.New("asset catalog too large")
	}
	return ExtractCarIcon(tmp, size, names)
}

type iconNames struct {
	files  []string // prefixes of loose icon files
	assets []string // asset catalog names
	icns   string   // Mac icon file
}

type infoPlist struct {
	ShortVersion string         `plist:"CFBundleShortVersionString"`
	BuildVersion string         `plist:"CFBundleVersion"`
	Icons        map[string]any `plist:"CFBundleIcons"`
	IconsIpad    map[string]any `plist:"CFBundleIcons~ipad"`
	IconFiles    []string       `plist:"CFBundleIconFiles"`
	IconFile     string         `plist:"CFBundleIconFile"`
}

func readInfoPlist(f *zip.File) (*infoPlist, error) {
	data, err := readZipFile(f)
	if err != nil {
		return nil, err
	}
	var info infoPlist
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return nil, errors.WithMessage(err, "parse Info.plist")
	}
	return &info, nil
}

// maxVersionLength keeps a bogus version from stretching the card
const maxVersionLength = 32

// version is the user facing version, or the build number if an app has none.
func (info *infoPlist) version() string {
	v := strings.TrimSpace(info.ShortVersion)
	if v == "" {
		v = strings.TrimSpace(info.BuildVersion)
	}
	if r := []rune(v); len(r) > maxVersionLength {
		v = string(r[:maxVersionLength])
	}
	return v
}

func (info *infoPlist) iconNames() iconNames {
	var names iconNames
	var files []string
	for _, icons := range []map[string]any{info.Icons, info.IconsIpad} {
		primary, _ := icons["CFBundlePrimaryIcon"].(map[string]any)
		list, _ := primary["CFBundleIconFiles"].([]any)
		for _, file := range list {
			if s, ok := file.(string); ok {
				files = append(files, s)
			}
		}
		if s, ok := primary["CFBundleIconName"].(string); ok && s != "" {
			files = append(files, s)
			names.assets = append(names.assets, s)
		}
	}
	files = append(files, info.IconFiles...)
	if info.IconFile != "" {
		files = append(files, info.IconFile)
		names.icns = strings.TrimSuffix(info.IconFile, ".icns") + ".icns"
	}
	for _, name := range files {
		name = strings.TrimSuffix(name, ".png")
		if name != "" {
			names.files = append(names.files, name)
		}
	}
	return names
}

func pickIcon(files map[string]*zip.File, prefixes []string) (*zip.File, int) {
	var best *zip.File
	bestArea := 0
	for name, f := range files {
		if path.Ext(name) != ".png" {
			continue
		}
		matched := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		area := pngArea(f)
		if best == nil || area > bestArea || (area == bestArea && name < best.Name) {
			best, bestArea = f, area
		}
	}
	return best, bestArea
}

// pngArea reads the dimensions from the first chunk; CgBI files put IHDR second.
func pngArea(f *zip.File) int {
	rc, err := f.Open()
	if err != nil {
		return 0
	}
	defer rc.Close()
	head := make([]byte, 8+2*(12+13))
	n, _ := io.ReadFull(rc, head)
	head = head[:n]
	if !bytes.HasPrefix(head, pngSignature) {
		return 0
	}
	pos := 8
	for pos+8 <= len(head) {
		length := int(binary.BigEndian.Uint32(head[pos:]))
		typ := string(head[pos+4 : pos+8])
		if typ == "IHDR" && pos+16 <= len(head) {
			return int(binary.BigEndian.Uint32(head[pos+8:])) * int(binary.BigEndian.Uint32(head[pos+12:]))
		}
		pos += 12 + length
	}
	return 0
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, errors.WithMessagef(err, "open %s", f.Name)
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, 32<<20))
	if err != nil {
		return nil, errors.WithMessagef(err, "read %s", f.Name)
	}
	return data, nil
}

type pngChunk struct {
	typ  string
	data []byte
}

func readChunks(data []byte) ([]pngChunk, error) {
	if !bytes.HasPrefix(data, pngSignature) {
		return nil, errors.New("not a png")
	}
	var chunks []pngChunk
	pos := len(pngSignature)
	for pos+12 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos:]))
		if length < 0 || pos+12+length > len(data) {
			return nil, errors.New("truncated png chunk")
		}
		chunks = append(chunks, pngChunk{string(data[pos+4 : pos+8]), data[pos+8 : pos+8+length]})
		pos += 12 + length
		if chunks[len(chunks)-1].typ == "IEND" {
			break
		}
	}
	return chunks, nil
}

// DecodeIconPNG decodes a PNG, including Xcode's "CgBI" crushed variant that standard decoders reject.
func DecodeIconPNG(data []byte) (image.Image, error) {
	chunks, err := readChunks(data)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 || chunks[0].typ != "CgBI" {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, errors.WithMessage(err, "decode png")
		}
		return img, nil
	}
	var ihdr []byte
	var idat bytes.Buffer
	for _, c := range chunks {
		switch c.typ {
		case "IHDR":
			ihdr = c.data
		case "IDAT":
			idat.Write(c.data)
		}
	}
	if len(ihdr) != 13 {
		return nil, errors.New("missing IHDR")
	}
	width := int(binary.BigEndian.Uint32(ihdr[0:]))
	height := int(binary.BigEndian.Uint32(ihdr[4:]))
	bitDepth, colorType, interlace := ihdr[8], ihdr[9], ihdr[12]
	if bitDepth != 8 || colorType != 6 || interlace > 1 {
		return nil, errors.Errorf("unsupported CgBI format depth=%d color=%d interlace=%d", bitDepth, colorType, interlace)
	}
	if width <= 0 || height <= 0 || width > maxIconSide || height > maxIconSide {
		return nil, errors.Errorf("unsupported CgBI size %dx%d", width, height)
	}
	passes := []adam7Pass{{0, 0, 1, 1}}
	if interlace == 1 {
		passes = adam7Passes
	}
	rawSize := 0
	for _, p := range passes {
		if w, h := p.size(width, height); w > 0 {
			rawSize += h * (1 + w*4)
		}
	}
	raw := make([]byte, rawSize)
	if _, err := io.ReadFull(flate.NewReader(&idat), raw); err != nil {
		return nil, errors.WithMessage(err, "inflate CgBI data")
	}
	stride := width * 4
	pix := make([]byte, stride*height)
	pos := 0
	for _, p := range passes {
		w, h := p.size(width, height)
		if w == 0 {
			continue
		}
		passStride := w * 4
		prev := make([]byte, passStride)
		for y := range h {
			line := raw[pos : pos+1+passStride]
			pos += 1 + passStride
			cur := line[1:]
			if err := unfilter(line[0], cur, prev, 4); err != nil {
				return nil, err
			}
			row := pix[(p.y0+y*p.dy)*stride:]
			for x := range w {
				copy(row[(p.x0+x*p.dx)*4:(p.x0+x*p.dx)*4+4], cur[x*4:x*4+4])
			}
			prev = cur
		}
	}
	return premultipliedBGRAToNRGBA(pix, width, height), nil
}

type adam7Pass struct{ x0, y0, dx, dy int }

var adam7Passes = []adam7Pass{{0, 0, 8, 8}, {4, 0, 8, 8}, {0, 4, 4, 8}, {2, 0, 4, 4}, {0, 2, 2, 4}, {1, 0, 2, 2}, {0, 1, 1, 2}}

// size is the pass's width and height in pixels, or zero if the image is too small for it.
func (p adam7Pass) size(width, height int) (int, int) {
	w, h := (width-p.x0+p.dx-1)/p.dx, (height-p.y0+p.dy-1)/p.dy
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	return w, h
}

func toNRGBA(src image.Image) *image.NRGBA {
	b := src.Bounds()
	if img, ok := src.(*image.NRGBA); ok && b.Min == (image.Point{}) {
		return img
	}
	img := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	return img
}

// shrinkImage scales an image down with a box filter so that no side exceeds maxSide.
func shrinkImage(src image.Image, maxSide int) *image.NRGBA {
	img := toNRGBA(src)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w <= maxSide && h <= maxSide {
		return img
	}
	dw, dh := maxSide, maxSide
	if w > h {
		dh = max(1, h*maxSide/w)
	} else {
		dw = max(1, w*maxSide/h)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := range dh {
		y0, y1 := y*h/dh, (y+1)*h/dh
		for x := range dw {
			x0, x1 := x*w/dw, (x+1)*w/dw
			// weight colors by alpha, so transparent pixels don't darken edges
			var r, g, bl, a, n uint64
			for sy := y0; sy < y1; sy++ {
				row := img.Pix[sy*img.Stride:]
				for sx := x0; sx < x1; sx++ {
					p := row[sx*4 : sx*4+4]
					pa := uint64(p[3])
					r, g, bl, a, n = r+uint64(p[0])*pa, g+uint64(p[1])*pa, bl+uint64(p[2])*pa, a+pa, n+1
				}
			}
			out := dst.Pix[y*dst.Stride+x*4 : y*dst.Stride+x*4+4]
			if a > 0 {
				out[0], out[1], out[2] = byte((r+a/2)/a), byte((g+a/2)/a), byte((bl+a/2)/a)
			}
			out[3] = byte((a + n/2) / n)
		}
	}
	return dst
}

func unpremultiply(c, a byte) byte {
	return byte(min((int(c)*255+int(a)/2)/int(a), 255))
}

func unfilter(filter byte, cur, prev []byte, bpp int) error {
	switch filter {
	case 0:
	case 1:
		for i := bpp; i < len(cur); i++ {
			cur[i] += cur[i-bpp]
		}
	case 2:
		for i := range cur {
			cur[i] += prev[i]
		}
	case 3:
		for i := range cur {
			var left byte
			if i >= bpp {
				left = cur[i-bpp]
			}
			cur[i] += byte((int(left) + int(prev[i])) / 2)
		}
	case 4:
		for i := range cur {
			var a, c byte
			if i >= bpp {
				a, c = cur[i-bpp], prev[i-bpp]
			}
			cur[i] += paeth(a, prev[i], c)
		}
	default:
		return errors.Errorf("bad png filter %d", filter)
	}
	return nil
}

func paeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa, pb, pc := abs(p-int(a)), abs(p-int(b)), abs(p-int(c))
	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
