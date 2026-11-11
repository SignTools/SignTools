package util

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"image"
	"image/png"
	"io"
	"path"
	"strings"

	"github.com/pkg/errors"
	"howett.net/plist"
)

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// ExtractIcon returns the largest app icon in an IPA as a standard PNG.
func ExtractIcon(r io.ReaderAt, size int64) ([]byte, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, errors.WithMessage(err, "open zip")
	}
	var appDir string
	var plistFile *zip.File
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
		if parts[2] == "Info.plist" {
			plistFile = f
		}
	}
	if appDir == "" {
		return nil, errors.New("no app bundle in Payload")
	}

	var prefixes []string
	if plistFile != nil {
		prefixes, err = iconNamesFromPlist(plistFile)
		if err != nil {
			return nil, err
		}
	}
	best, bestArea := pickIcon(rootFiles, prefixes)
	if best == nil {
		best, bestArea = pickIcon(rootFiles, []string{"AppIcon", "Icon"})
	}
	if best == nil || bestArea == 0 {
		return nil, errors.New("no icon found")
	}
	data, err := readZipFile(best)
	if err != nil {
		return nil, err
	}
	return NormalizePNG(data)
}

func iconNamesFromPlist(f *zip.File) ([]string, error) {
	data, err := readZipFile(f)
	if err != nil {
		return nil, err
	}
	var info struct {
		Icons     map[string]any `plist:"CFBundleIcons"`
		IconsIpad map[string]any `plist:"CFBundleIcons~ipad"`
		IconFiles []string       `plist:"CFBundleIconFiles"`
		IconFile  string         `plist:"CFBundleIconFile"`
	}
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return nil, errors.WithMessage(err, "parse Info.plist")
	}
	var names []string
	for _, icons := range []map[string]any{info.Icons, info.IconsIpad} {
		primary, _ := icons["CFBundlePrimaryIcon"].(map[string]any)
		files, _ := primary["CFBundleIconFiles"].([]any)
		for _, file := range files {
			if s, ok := file.(string); ok {
				names = append(names, s)
			}
		}
		if s, ok := primary["CFBundleIconName"].(string); ok {
			names = append(names, s)
		}
	}
	names = append(names, info.IconFiles...)
	if info.IconFile != "" {
		names = append(names, info.IconFile)
	}
	var prefixes []string
	for _, name := range names {
		name = strings.TrimSuffix(name, ".png")
		if name != "" {
			prefixes = append(prefixes, name)
		}
	}
	return prefixes, nil
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

// NormalizePNG converts an Xcode "CgBI" crushed PNG to a standard one, and passes other PNGs through.
func NormalizePNG(data []byte) ([]byte, error) {
	chunks, err := readChunks(data)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 || chunks[0].typ != "CgBI" {
		if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
			return nil, errors.WithMessage(err, "decode png")
		}
		return data, nil
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
	if width <= 0 || height <= 0 || width > 4096 || height > 4096 {
		return nil, errors.Errorf("unsupported CgBI size %dx%d", width, height)
	}
	stride := width * 4
	raw := make([]byte, (stride+1)*height)
	if _, err := io.ReadFull(flate.NewReader(&idat), raw); err != nil {
		return nil, errors.WithMessage(err, "inflate CgBI data")
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	prev := make([]byte, stride)
	for y := range height {
		line := raw[y*(stride+1) : (y+1)*(stride+1)]
		cur := line[1:]
		if err := unfilter(line[0], cur, prev, 4); err != nil {
			return nil, err
		}
		out := img.Pix[y*img.Stride : y*img.Stride+stride]
		for x := 0; x < stride; x += 4 {
			b, g, r, a := cur[x], cur[x+1], cur[x+2], cur[x+3]
			if a != 0 && a != 255 {
				r = unpremultiply(r, a)
				g = unpremultiply(g, a)
				b = unpremultiply(b, a)
			}
			out[x], out[x+1], out[x+2], out[x+3] = r, g, b, a
		}
		prev = cur
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, errors.WithMessage(err, "encode png")
	}
	return buf.Bytes(), nil
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
