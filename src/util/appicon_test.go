package util

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 40), G: uint8(y * 50), B: uint8(x*y*7 + 3), A: 255})
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func writeChunk(buf *bytes.Buffer, typ string, data []byte) {
	_ = binary.Write(buf, binary.BigEndian, uint32(len(data)))
	buf.WriteString(typ)
	buf.Write(data)
	_ = binary.Write(buf, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(typ), data...)))
}

// crushPNG mimics Xcode's pngcrush: premultiplied BGRA, raw deflate, CgBI chunk first.
// Each row uses a different filter so all of them get exercised.
func crushPNG(t *testing.T, img *image.NRGBA) []byte {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	stride := w * 4
	var raw bytes.Buffer
	prev := make([]byte, stride)
	for y := range h {
		cur := make([]byte, stride)
		for x := range w {
			c := img.NRGBAAt(x, y)
			premul := func(v uint8) byte { return byte(int(v) * int(c.A) / 255) }
			cur[x*4], cur[x*4+1], cur[x*4+2], cur[x*4+3] = premul(c.B), premul(c.G), premul(c.R), c.A
		}
		filter := byte(y % 5)
		out := make([]byte, stride)
		for i := range cur {
			var left, upLeft byte
			if i >= 4 {
				left, upLeft = cur[i-4], prev[i-4]
			}
			var pred byte
			switch filter {
			case 1:
				pred = left
			case 2:
				pred = prev[i]
			case 3:
				pred = byte((int(left) + int(prev[i])) / 2)
			case 4:
				pred = paeth(left, prev[i], upLeft)
			}
			out[i] = cur[i] - pred
		}
		raw.WriteByte(filter)
		raw.Write(out)
		prev = cur
	}
	var compressed bytes.Buffer
	fw, err := flate.NewWriter(&compressed, flate.BestCompression)
	require.NoError(t, err)
	_, err = fw.Write(raw.Bytes())
	require.NoError(t, err)
	require.NoError(t, fw.Close())

	var buf bytes.Buffer
	buf.Write(pngSignature)
	writeChunk(&buf, "CgBI", []byte{0x50, 0x00, 0x20, 0x06})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8], ihdr[9] = 8, 6
	writeChunk(&buf, "IHDR", ihdr)
	writeChunk(&buf, "IDAT", compressed.Bytes())
	writeChunk(&buf, "IEND", nil)
	return buf.Bytes()
}

func makeIPA(t *testing.T, files map[string][]byte) *bytes.Reader {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return bytes.NewReader(buf.Bytes())
}

func decode(t *testing.T, data []byte) image.Image {
	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	return img
}

func TestDecodeIconPNGCgBI(t *testing.T) {
	want := testImage(7, 6)
	want.SetNRGBA(2, 3, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	want.SetNRGBA(4, 1, color.NRGBA{A: 0})

	got, err := DecodeIconPNG(crushPNG(t, want))
	require.NoError(t, err)
	require.Equal(t, want.Bounds(), got.Bounds())
	for y := range 6 {
		for x := range 7 {
			w := want.NRGBAAt(x, y)
			g := color.NRGBAModel.Convert(got.At(x, y)).(color.NRGBA)
			if w.A == 0 {
				assert.Equal(t, uint8(0), g.A)
				continue
			}
			// premultiplying loses some precision at low alpha
			assert.InDelta(t, w.R, g.R, 2, "R at %d,%d", x, y)
			assert.InDelta(t, w.G, g.G, 2, "G at %d,%d", x, y)
			assert.InDelta(t, w.B, g.B, 2, "B at %d,%d", x, y)
			assert.Equal(t, w.A, g.A)
		}
	}
}

func TestDecodeIconPNGStandard(t *testing.T) {
	want := testImage(3, 3)
	got, err := DecodeIconPNG(encodePNG(t, want))
	require.NoError(t, err)
	assert.Equal(t, want.Pix, shrinkImage(got, 10).Pix)

	_, err = DecodeIconPNG([]byte("not a png"))
	assert.Error(t, err)
}

func TestShrinkImage(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 400, 200))
	for i := 0; i < len(src.Pix); i += 4 {
		copy(src.Pix[i:], []byte{10, 20, 30, 255})
	}
	// a transparent pixel with a stray color must not tint its neighbours
	copy(src.Pix[0:], []byte{255, 255, 255, 0})
	got := shrinkImage(src, 192)
	assert.Equal(t, image.Rect(0, 0, 192, 96), got.Bounds())
	assert.Equal(t, color.NRGBA{10, 20, 30, 191}, got.NRGBAAt(0, 0))
	assert.Equal(t, color.NRGBA{10, 20, 30, 255}, got.NRGBAAt(100, 50))
}

const testPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>CFBundleIcons</key><dict>
		<key>CFBundlePrimaryIcon</key><dict>
			<key>CFBundleIconFiles</key><array><string>MyIcon60x60</string></array>
		</dict>
	</dict>
</dict></plist>`

func TestExtractIconFromPlist(t *testing.T) {
	big := testImage(12, 12)
	ipa := makeIPA(t, map[string][]byte{
		"Payload/Test.app/Info.plist":                    []byte(testPlist),
		"Payload/Test.app/MyIcon60x60@2x.png":            crushPNG(t, testImage(8, 8)),
		"Payload/Test.app/MyIcon60x60@3x.png":            crushPNG(t, big),
		"Payload/Test.app/AppIcon99x99.png":              encodePNG(t, testImage(20, 20)),
		"Payload/Test.app/Frameworks/X.framework/a.png":  encodePNG(t, testImage(30, 30)),
		"Payload/Test.app/PlugIns/Ext.appex/Info.plist":  []byte(testPlist),
		"Payload/Test.app/PlugIns/Ext.appex/MyIcon1.png": encodePNG(t, testImage(40, 40)),
	})
	out, err := ExtractIcon(ipa, ipa.Size())
	require.NoError(t, err)
	assert.Equal(t, big.Pix, shrinkImage(decode(t, out), 100).Pix)
}

func TestExtractIconFallback(t *testing.T) {
	ipa := makeIPA(t, map[string][]byte{
		"Payload/Test.app/Info.plist":   []byte(`<plist version="1.0"><dict></dict></plist>`),
		"Payload/Test.app/Icon.png":     encodePNG(t, testImage(4, 4)),
		"Payload/Test.app/Icon@2x.png":  encodePNG(t, testImage(9, 9)),
		"Payload/Test.app/LaunchBg.png": encodePNG(t, testImage(50, 50)),
	})
	out, err := ExtractIcon(ipa, ipa.Size())
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 9, 9), decode(t, out).Bounds())
}

func TestExtractIconMissing(t *testing.T) {
	ipa := makeIPA(t, map[string][]byte{
		"Payload/Test.app/Info.plist": []byte(testPlist),
		"Payload/Test.app/Launch.png": encodePNG(t, testImage(4, 4)),
	})
	_, err := ExtractIcon(ipa, ipa.Size())
	assert.Error(t, err)

	notZip := bytes.NewReader([]byte("nope"))
	_, err = ExtractIcon(notZip, notZip.Size())
	assert.Error(t, err)
}

func plistWith(entries string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>` + entries + `</dict></plist>`)
}

func TestReadAppInfoVersion(t *testing.T) {
	for _, tc := range []struct{ entries, want string }{
		{`<key>CFBundleShortVersionString</key><string> 4.11.3 </string><key>CFBundleVersion</key><string>1234</string>`, "4.11.3"},
		{`<key>CFBundleVersion</key><string>1234</string>`, "1234"},
		{`<key>CFBundleShortVersionString</key><string>` + strings.Repeat("9", 50) + `</string>`, strings.Repeat("9", maxVersionLength)},
		{``, ""},
	} {
		ipa := makeIPA(t, map[string][]byte{
			"Payload/Test.app/Info.plist": plistWith(tc.entries),
			"Payload/Test.app/Icon.png":   encodePNG(t, testImage(4, 4)),
		})
		info, err := ReadAppInfo(ipa, ipa.Size())
		require.NoError(t, err)
		assert.Equal(t, tc.want, info.Version)
		assert.NoError(t, info.IconErr)
		assert.NotEmpty(t, info.Icon)
	}

	// the version is still read without an icon
	ipa := makeIPA(t, map[string][]byte{
		"Payload/Test.app/Info.plist": plistWith(`<key>CFBundleShortVersionString</key><string>2.0</string>`),
	})
	info, err := ReadAppInfo(ipa, ipa.Size())
	require.NoError(t, err)
	assert.Equal(t, "2.0", info.Version)
	assert.Error(t, info.IconErr)
	assert.Empty(t, info.Icon)
}
