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
	var appDir string
	rootFiles := map[string]*zip.File{}
	for _, f := range zr.File {
		parts := strings.Split(f.Name, "/")
		if len(parts) != 3 || parts[0] != "Payload" || !strings.HasSuffix(parts[1], ".app") {
			continue
		}
		if appDir == "" {
			appDir = parts[1]
		} else if appDir != parts[1] {
			continue
		}
		rootFiles[parts[2]] = f
	}
	if appDir == "" {
		return nil, errors.New("no app bundle in Payload")
	}

	info := &AppInfo{}
	var names iconNames
	if plistFile, ok := rootFiles["Info.plist"]; ok {
		var plistInfo *infoPlist
		if plistInfo, err = readInfoPlist(plistFile); err != nil {
			return nil, err
		}
		info.Version = plistInfo.version()
		names = plistInfo.iconNames()
	}
	info.Icon, info.IconErr = extractIcon(r, rootFiles, names)
	return info, nil
}

func extractIcon(r io.ReaderAt, rootFiles map[string]*zip.File, names iconNames) ([]byte, error) {
	img, looseErr := looseIcon(rootFiles, names.files)
	if img == nil || img.Bounds().Dx() < iconOutputSize {
		if car, ok := rootFiles["Assets.car"]; ok {
			carImg, carErr := carIcon(r, car, append(names.assets, "AppIcon"))
			if carErr == nil && (img == nil || carImg.Bounds().Dx() > img.Bounds().Dx()) {
				img = carImg
			} else if img == nil {
				return nil, errors.Errorf("%v; Assets.car: %v", looseErr, carErr)
			}
		}
	}
	if img == nil {
		return nil, looseErr
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, shrinkImage(img, iconOutputSize)); err != nil {
		return nil, errors.WithMessage(err, "encode png")
	}
	return buf.Bytes(), nil
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
	if bitDepth != 8 || colorType != 6 || interlace != 0 {
		return nil, errors.Errorf("unsupported CgBI format depth=%d color=%d interlace=%d", bitDepth, colorType, interlace)
	}
	if width <= 0 || height <= 0 || width > maxIconSide || height > maxIconSide {
		return nil, errors.Errorf("unsupported CgBI size %dx%d", width, height)
	}
	stride := width * 4
	raw := make([]byte, (stride+1)*height)
	if _, err := io.ReadFull(flate.NewReader(&idat), raw); err != nil {
		return nil, errors.WithMessage(err, "inflate CgBI data")
	}
	pix := make([]byte, stride*height)
	prev := make([]byte, stride)
	for y := range height {
		line := raw[y*(stride+1) : (y+1)*(stride+1)]
		cur := line[1:]
		if err := unfilter(line[0], cur, prev, 4); err != nil {
			return nil, err
		}
		copy(pix[y*stride:], cur)
		prev = cur
	}
	return premultipliedBGRAToNRGBA(pix, width, height), nil
}

// shrinkImage scales an image down with a box filter so that no side exceeds maxSide.
func shrinkImage(src image.Image, maxSide int) *image.NRGBA {
	b := src.Bounds()
	img, ok := src.(*image.NRGBA)
	if !ok || b.Min != (image.Point{}) {
		img = image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	}
	w, h := b.Dx(), b.Dy()
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
