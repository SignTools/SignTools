package util

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bomBuilder writes a minimal BOM store. Block 0 is the null block, as in real files.
type bomBuilder struct {
	blocks [][]byte
	vars   []string
	varIdx []uint32
}

func (b *bomBuilder) add(data []byte) uint32 {
	if len(b.blocks) == 0 {
		b.blocks = append(b.blocks, nil)
	}
	b.blocks = append(b.blocks, data)
	return uint32(len(b.blocks) - 1)
}

func (b *bomBuilder) setVar(name string, idx uint32) {
	b.vars = append(b.vars, name)
	b.varIdx = append(b.varIdx, idx)
}

type bomEntry struct{ key, value []byte }

func pathsNode(leaf bool, forward uint32, pairs [][2]uint32) []byte {
	be := binary.BigEndian
	node := make([]byte, 12)
	if leaf {
		be.PutUint16(node, 1)
	}
	be.PutUint16(node[2:], uint16(len(pairs)))
	be.PutUint32(node[4:], forward)
	for _, p := range pairs {
		node = be.AppendUint32(node, p[0])
		node = be.AppendUint32(node, p[1])
	}
	return node
}

// addTree splits entries over two linked leaves under a branch node, to exercise both.
func (b *bomBuilder) addTree(name string, entries []bomEntry) {
	var pairs [][2]uint32
	for _, e := range entries {
		pairs = append(pairs, [2]uint32{b.add(e.value), b.add(e.key)})
	}
	half := len(pairs) / 2
	second := b.add(pathsNode(true, 0, pairs[half:]))
	first := b.add(pathsNode(true, second, pairs[:half]))
	root := b.add(pathsNode(false, 0, [][2]uint32{{first, 0}, {second, 0}}))
	tree := append([]byte("tree"), make([]byte, 17)...)
	binary.BigEndian.PutUint32(tree[4:], 1)
	binary.BigEndian.PutUint32(tree[8:], root)
	b.setVar(name, b.add(tree))
}

func (b *bomBuilder) bytes() []byte {
	be := binary.BigEndian
	out := make([]byte, 512)
	copy(out, "BOMStore")
	be.PutUint32(out[8:], 1)
	be.PutUint32(out[12:], uint32(len(b.blocks)))
	index := be.AppendUint32(nil, uint32(len(b.blocks)))
	for _, block := range b.blocks {
		if block == nil {
			index = be.AppendUint32(be.AppendUint32(index, 0), 0)
			continue
		}
		index = be.AppendUint32(index, uint32(len(out)))
		index = be.AppendUint32(index, uint32(len(block)))
		out = append(out, block...)
	}
	vars := be.AppendUint32(nil, uint32(len(b.vars)))
	for i, name := range b.vars {
		vars = be.AppendUint32(vars, b.varIdx[i])
		vars = append(vars, byte(len(name)))
		vars = append(vars, name...)
	}
	be.PutUint32(out[16:], uint32(len(out)))
	be.PutUint32(out[20:], uint32(len(index)))
	out = append(out, index...)
	be.PutUint32(out[24:], uint32(len(out)))
	be.PutUint32(out[28:], uint32(len(vars)))
	return append(out, vars...)
}

// uncompressed LZFSE blocks, so the real decoder runs without needing an encoder
func lzfseRaw(data []byte) []byte {
	out := append([]byte("bvx-"), binary.LittleEndian.AppendUint32(nil, uint32(len(data)))...)
	out = append(out, data...)
	return append(out, "bvx$"...)
}

type testRendition struct {
	appearance, identifier uint16
	idiom                  uint16
	layout                 uint16
	width, height          int
	color                  color.NRGBA
	pad                    int    // alignment bytes after each row
	jpeg                   []byte // stored as a JPEG file instead of a bitmap
}

func (r testRendition) csi() []byte {
	le := binary.LittleEndian
	head := make([]byte, csiHeaderSize)
	copy(head, "ISTC")
	le.PutUint32(head[12:], uint32(r.width))
	le.PutUint32(head[16:], uint32(r.height))
	le.PutUint32(head[20:], 100)
	copy(head[24:], "BGRA")
	le.PutUint16(head[36:], r.layout)
	copy(head[40:], "icon.png")
	if r.layout == 1010 {
		return head
	}
	if r.jpeg != nil {
		copy(head[24:], "GEPJ")
		le.PutUint32(head[12:], 0)
		le.PutUint32(head[16:], 0)
		data := append([]byte("DWAR"), le.AppendUint32(le.AppendUint32(nil, 0), uint32(len(r.jpeg)))...)
		data = append(data, r.jpeg...)
		le.PutUint32(head[180:], uint32(len(data)))
		return append(head, data...)
	}
	a := uint32(r.color.A)
	px := []byte{byte(uint32(r.color.B) * a / 255), byte(uint32(r.color.G) * a / 255), byte(uint32(r.color.R) * a / 255), r.color.A}
	// two bands of rows, like Xcode's output
	bands := []int{r.height - r.height/3, r.height / 3}
	data := append([]byte("MLEC"), make([]byte, 12)...)
	le.PutUint32(data[4:], 3)
	le.PutUint32(data[8:], 4)
	le.PutUint32(data[12:], uint32(len(bands)))
	row := append(bytes.Repeat(px, r.width), make([]byte, r.pad)...)
	for _, rows := range bands {
		chunk := lzfseRaw(bytes.Repeat(row, rows))
		hdr := append([]byte("KCBC"), make([]byte, 16)...)
		le.PutUint32(hdr[12:], uint32(rows))
		le.PutUint32(hdr[16:], uint32(len(chunk)))
		data = append(append(data, hdr...), chunk...)
	}
	le.PutUint32(head[180:], uint32(len(data)))
	return append(head, data...)
}

func makeCar(renditions []testRendition) []byte {
	le := binary.LittleEndian
	b := &bomBuilder{}
	// key format: appearance, identifier, scale, idiom
	kf := append([]byte("tmfk"), make([]byte, 8)...)
	le.PutUint32(kf[8:], 4)
	for _, attr := range []uint32{carAttrAppearance, carAttrIdentifier, 12, carAttrIdiom} {
		kf = le.AppendUint32(kf, attr)
	}
	b.setVar("KEYFORMAT", b.add(kf))
	facet := func(id uint16) []byte {
		v := le.AppendUint16(make([]byte, 4), 2)
		v = le.AppendUint16(le.AppendUint16(v, 1), 85)
		return le.AppendUint16(le.AppendUint16(v, carAttrIdentifier), id)
	}
	b.addTree("FACETKEYS", []bomEntry{
		{[]byte("AppIcon"), facet(42)},
		{[]byte("Other"), facet(7)},
	})
	var entries []bomEntry
	for _, r := range renditions {
		key := le.AppendUint16(le.AppendUint16(le.AppendUint16(le.AppendUint16(nil, r.appearance), r.identifier), 1), r.idiom)
		entries = append(entries, bomEntry{key, r.csi()})
	}
	b.addTree("RENDITIONS", entries)
	return b.bytes()
}

var (
	iconColor = color.NRGBA{R: 200, G: 100, B: 50, A: 255}
	testCar   = makeCar([]testRendition{
		{appearance: 1, identifier: 42, layout: 12, width: 400, height: 400, color: color.NRGBA{10, 10, 10, 255}},
		{appearance: 0, identifier: 42, layout: 1010},
		{appearance: 0, identifier: 42, layout: 12, width: 100, height: 100, color: color.NRGBA{1, 2, 3, 255}},
		{appearance: 0, identifier: 42, layout: 12, width: 300, height: 240, color: iconColor},
		{appearance: 0, identifier: 7, layout: 12, width: 500, height: 500, color: color.NRGBA{9, 9, 9, 255}},
	})
)

func TestExtractCarIcon(t *testing.T) {
	img, err := ExtractCarIcon(bytes.NewReader(testCar), int64(len(testCar)), []string{"Missing", "AppIcon"})
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 300, 240), img.Bounds())
	assert.Equal(t, iconColor, img.NRGBAAt(0, 0))
	assert.Equal(t, iconColor, img.NRGBAAt(299, 239))

	_, err = ExtractCarIcon(bytes.NewReader(testCar), int64(len(testCar)), []string{"Nope"})
	assert.Error(t, err)
}

func TestExtractCarIconSemiTransparent(t *testing.T) {
	want := color.NRGBA{R: 200, G: 100, B: 50, A: 128}
	car := makeCar([]testRendition{{identifier: 42, layout: 12, width: 4, height: 3, color: want}})
	img, err := ExtractCarIcon(bytes.NewReader(car), int64(len(car)), []string{"AppIcon"})
	require.NoError(t, err)
	got := img.NRGBAAt(1, 1)
	assert.InDelta(t, want.R, got.R, 2)
	assert.InDelta(t, want.G, got.G, 2)
	assert.InDelta(t, want.B, got.B, 2)
	assert.Equal(t, want.A, got.A)
}

func TestExtractIconFromCar(t *testing.T) {
	for _, method := range []uint16{zip.Store, zip.Deflate} {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for name, data := range map[string][]byte{
			"Payload/Test.app/Info.plist": []byte(testPlist),
			"Payload/Test.app/Assets.car": testCar,
		} {
			w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
			require.NoError(t, err)
			_, err = w.Write(data)
			require.NoError(t, err)
		}
		require.NoError(t, zw.Close())
		ipa := bytes.NewReader(buf.Bytes())

		out, err := ExtractIcon(ipa, ipa.Size())
		require.NoError(t, err, "method %d", method)
		img := decode(t, out)
		assert.Equal(t, image.Rect(0, 0, 192, 153), img.Bounds())
		assert.Equal(t, iconColor, color.NRGBAModel.Convert(img.At(50, 50)))
	}
}

func TestExtractIconPrefersBiggerCarIcon(t *testing.T) {
	ipa := makeIPA(t, map[string][]byte{
		"Payload/Test.app/Info.plist":         []byte(testPlist),
		"Payload/Test.app/MyIcon60x60@2x.png": encodePNG(t, testImage(8, 8)),
		"Payload/Test.app/Assets.car":         testCar,
	})
	out, err := ExtractIcon(ipa, ipa.Size())
	require.NoError(t, err)
	assert.Equal(t, iconColor, color.NRGBAModel.Convert(decode(t, out).At(10, 10)))
}

func TestExtractCarIconMalformed(t *testing.T) {
	car := makeCar([]testRendition{
		{appearance: 1, identifier: 42, layout: 12, width: 3, height: 3, color: iconColor},
		{identifier: 42, layout: 1010},
		{identifier: 42, layout: 12, width: 4, height: 3, color: iconColor},
	})
	_, err := ExtractCarIcon(bytes.NewReader(car), int64(len(car)), []string{"AppIcon"})
	require.NoError(t, err)
	// every corruption must fail cleanly or succeed, never panic
	for i := range car {
		for _, flip := range []byte{0x01, 0x80, 0xff} {
			broken := bytes.Clone(car)
			broken[i] ^= flip
			_, _ = ExtractCarIcon(bytes.NewReader(broken), int64(len(broken)), []string{"AppIcon"})
		}
	}
	for n := range car {
		_, _ = ExtractCarIcon(bytes.NewReader(car[:n]), int64(n), []string{"AppIcon"})
	}
}

func TestLzfseBlocks(t *testing.T) {
	data := lzfseRaw(make([]byte, 10))
	blocks, total, err := lzfseBlocks(data, 10)
	require.NoError(t, err)
	assert.Equal(t, 10, total)
	assert.Len(t, blocks, 1)
	_, total, err = lzfseBlocks(data, 11)
	assert.NoError(t, err)
	assert.Equal(t, 10, total)
	_, _, err = lzfseBlocks(data, 9)
	assert.Error(t, err)
	_, _, err = lzfseBlocks(data[:len(data)-4], 10)
	assert.Error(t, err)

	huge := append([]byte("bvxn"), binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32(nil, 10), 0xffffffff)...)
	_, _, err = lzfseBlocks(append(huge, "bvx$"...), 10)
	assert.Error(t, err)
	_, _, err = lzfseBlocks([]byte("bvx1aaaaaaaa"), 10)
	assert.Error(t, err)

	out, err := decompressLzfse(data, 10)
	require.NoError(t, err)
	assert.Equal(t, make([]byte, 10), out)
}

// a compressed block shorter than the decoder's fixed header struct, made with
// `compression_tool -encode -a lzfse` from bytes (i%7)*30 for i in 0..8191
const shortLzfseHex = "62767832002000000c004000000500701c74b3976f1000308f0000003cbc400fef00c015570000005700000000" +
	"f01bbc20bf01f01b0000000000000000000000000000c04b02000000000000c04f02000000000000c0f70000000000" +
	"0000c0f700000000000000c04f02000000000000c04f02000000000000c04f02000000000000000000000000000000" +
	"0000000000abcf730000000000000000c08bfd7fff9fff0f62767824"

func TestDecompressLzfseShortBlock(t *testing.T) {
	data, err := hex.DecodeString(shortLzfseHex)
	require.NoError(t, err)
	want := make([]byte, 8192)
	for i := range want {
		want[i] = byte(i % 7 * 30)
	}
	out, err := decompressLzfse(data, len(want))
	require.NoError(t, err)
	assert.Equal(t, want, out)
}

func carImage(t *testing.T, car []byte) *image.NRGBA {
	img, err := ExtractCarIcon(bytes.NewReader(car), int64(len(car)), []string{"AppIcon"})
	require.NoError(t, err)
	return img
}

func TestExtractCarIconRowPadding(t *testing.T) {
	// 180px rows of 720 bytes stored as 736, as in older catalogs
	img := carImage(t, makeCar([]testRendition{{identifier: 42, layout: 12, width: 180, height: 9, color: iconColor, pad: 16}}))
	assert.Equal(t, image.Rect(0, 0, 180, 9), img.Bounds())
	assert.Equal(t, iconColor, img.NRGBAAt(179, 8))

	car := makeCar([]testRendition{{identifier: 42, layout: 12, width: 4, height: 3, color: iconColor, pad: maxRowPadding + 4}})
	_, err := ExtractCarIcon(bytes.NewReader(car), int64(len(car)), []string{"AppIcon"})
	assert.Error(t, err)
}

func TestExtractCarIconJPEG(t *testing.T) {
	var buf bytes.Buffer
	src := image.NewRGBA(image.Rect(0, 0, 64, 48))
	draw.Draw(src, src.Bounds(), image.NewUniform(color.RGBA{0, 0, 255, 255}), image.Point{}, draw.Src)
	require.NoError(t, jpeg.Encode(&buf, src, &jpeg.Options{Quality: 100}))
	device := testRendition{identifier: 42, idiom: 1, layout: 12, width: 20, height: 20, color: iconColor}
	marketing := testRendition{identifier: 42, idiom: carIdiomMarketing, layout: 12, jpeg: buf.Bytes()}
	plain := testRendition{identifier: 42, layout: 12, jpeg: buf.Bytes()}

	// a stored JPEG is used like a bitmap
	img := carImage(t, makeCar([]testRendition{device, plain}))
	assert.Equal(t, image.Rect(0, 0, 64, 48), img.Bounds())
	got := img.NRGBAAt(10, 10)
	assert.InDelta(t, 255, got.B, 3)
	assert.InDelta(t, 0, got.R, 3)

	// the App Store icon only if there's nothing else, even though it's bigger
	assert.Equal(t, image.Rect(0, 0, 20, 20), carImage(t, makeCar([]testRendition{marketing, device})).Bounds())
	assert.Equal(t, image.Rect(0, 0, 64, 48), carImage(t, makeCar([]testRendition{marketing})).Bounds())
}
