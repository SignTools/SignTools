package util

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/aixiansheng/lzfse"
	"github.com/pkg/errors"
)

// Compiled asset catalogs (Assets.car) are BOM files. Their RENDITIONS tree maps attribute keys to
// CSI-wrapped images, and FACETKEYS maps asset names to the identifier attribute of their renditions.

const (
	carAttrAppearance = 7
	carAttrIdiom      = 15
	carIdiomMarketing = 6 // the App Store icon, whose artwork can differ from the app's
	carAttrIdentifier = 17

	carLayoutImageMin = 10 // one-part fixed size
	carLayoutImageMax = 12 // one-part scale

	csiHeaderSize   = 184
	maxBomBlockSize = 64 << 20
	maxIconSide     = 4096
)

type bomStore struct {
	r      io.ReaderAt
	size   int64
	blocks [][2]uint32
	vars   map[string]uint32
}

func readAt(r io.ReaderAt, size int64, off int64, n int) ([]byte, error) {
	if off < 0 || n < 0 || off+int64(n) > size {
		return nil, errors.New("read out of bounds")
	}
	b := make([]byte, n)
	if _, err := r.ReadAt(b, off); err != nil {
		return nil, err
	}
	return b, nil
}

func openBom(r io.ReaderAt, size int64) (*bomStore, error) {
	head, err := readAt(r, size, 0, 32)
	if err != nil {
		return nil, errors.WithMessage(err, "read bom header")
	}
	if string(head[:8]) != "BOMStore" {
		return nil, errors.New("not a bom file")
	}
	be := binary.BigEndian
	indexOff, indexLen := be.Uint32(head[16:]), be.Uint32(head[20:])
	varsOff, varsLen := be.Uint32(head[24:]), be.Uint32(head[28:])
	if indexLen > maxBomBlockSize || varsLen > maxBomBlockSize {
		return nil, errors.New("bom index too large")
	}
	index, err := readAt(r, size, int64(indexOff), int(indexLen))
	if err != nil || len(index) < 4 {
		return nil, errors.New("read bom index")
	}
	count := int(be.Uint32(index))
	if count > (len(index)-4)/8 {
		return nil, errors.New("bom index truncated")
	}
	b := &bomStore{r: r, size: size, vars: map[string]uint32{}}
	for i := range count {
		e := index[4+8*i:]
		b.blocks = append(b.blocks, [2]uint32{be.Uint32(e), be.Uint32(e[4:])})
	}
	vars, err := readAt(r, size, int64(varsOff), int(varsLen))
	if err != nil || len(vars) < 4 {
		return nil, errors.New("read bom vars")
	}
	pos := 4
	for range be.Uint32(vars) {
		if pos+5 > len(vars) {
			return nil, errors.New("bom vars truncated")
		}
		idx, n := be.Uint32(vars[pos:]), int(vars[pos+4])
		if pos+5+n > len(vars) {
			return nil, errors.New("bom vars truncated")
		}
		b.vars[string(vars[pos+5:pos+5+n])] = idx
		pos += 5 + n
	}
	return b, nil
}

func (b *bomStore) block(i uint32) ([]byte, error) {
	if int(i) >= len(b.blocks) {
		return nil, errors.Errorf("bom block %d out of range", i)
	}
	addr, n := b.blocks[i][0], b.blocks[i][1]
	if n > maxBomBlockSize {
		return nil, errors.Errorf("bom block %d too large", i)
	}
	return readAt(b.r, b.size, int64(addr), int(n))
}

func (b *bomStore) blockPrefix(i uint32, n int) ([]byte, error) {
	if int(i) >= len(b.blocks) || b.blocks[i][1] < uint32(n) {
		return nil, errors.Errorf("bom block %d too small", i)
	}
	return readAt(b.r, b.size, int64(b.blocks[i][0]), n)
}

// walkTree calls fn with the key and value block indexes of every leaf entry in a BOM tree.
func (b *bomStore) walkTree(name string, fn func(key []byte, value uint32) error) error {
	treeIdx, ok := b.vars[name]
	if !ok {
		return errors.Errorf("no %s tree", name)
	}
	tree, err := b.block(treeIdx)
	if err != nil {
		return err
	}
	if len(tree) < 12 || string(tree[:4]) != "tree" {
		return errors.Errorf("bad %s tree", name)
	}
	be := binary.BigEndian
	node, err := b.block(be.Uint32(tree[8:]))
	if err != nil {
		return err
	}
	visited := map[uint32]bool{}
	// descend to the leftmost leaf
	for len(node) >= 12 && be.Uint16(node) == 0 {
		if be.Uint16(node[2:]) == 0 || len(node) < 16 {
			return errors.Errorf("empty %s tree node", name)
		}
		child := be.Uint32(node[12:])
		if visited[child] {
			return errors.Errorf("%s tree has a cycle", name)
		}
		visited[child] = true
		if node, err = b.block(child); err != nil {
			return err
		}
	}
	for {
		if len(node) < 12 {
			return errors.Errorf("bad %s tree leaf", name)
		}
		count, forward := int(be.Uint16(node[2:])), be.Uint32(node[4:])
		if 12+8*count > len(node) {
			return errors.Errorf("%s tree leaf truncated", name)
		}
		for i := range count {
			entry := node[12+8*i:]
			key, err := b.block(be.Uint32(entry[4:]))
			if err != nil {
				return err
			}
			if err := fn(key, be.Uint32(entry)); err != nil {
				return err
			}
		}
		if forward == 0 {
			return nil
		}
		if visited[forward] {
			return errors.Errorf("%s tree has a cycle", name)
		}
		visited[forward] = true
		if node, err = b.block(forward); err != nil {
			return err
		}
	}
}

// pixel formats, as stored on disk
const (
	carFormatARGB = "BGRA"
	carFormatGA8  = " 8AG"
	carFormatJPEG = "GEPJ"
	carFormatPNG  = " GNP"
)

type carRendition struct {
	value      uint32
	appearance uint16
	marketing  bool
	gray       bool
	format     string
	encoded    []byte // JPEG or PNG data, for renditions stored as files
	width      int
	height     int
}

// betterThan prefers the default appearance over dark and tinted variants, then device icons over
// the App Store one, then color over grayscale, then the bigger image.
func (c *carRendition) betterThan(o *carRendition) bool {
	if (c.appearance == 0) != (o.appearance == 0) {
		return c.appearance == 0
	}
	if c.marketing != o.marketing {
		return !c.marketing
	}
	if c.gray != o.gray {
		return !c.gray
	}
	return c.width*c.height > o.width*o.height
}

// ExtractCarIcon returns the largest bitmap of the first named asset found in an Assets.car.
func ExtractCarIcon(r io.ReaderAt, size int64, names []string) (*image.NRGBA, error) {
	bom, err := openBom(r, size)
	if err != nil {
		return nil, err
	}
	le := binary.LittleEndian

	// asset name -> identifier
	identifiers := map[string]uint16{}
	if err := bom.walkTree("FACETKEYS", func(key []byte, value uint32) error {
		facet, err := bom.block(value)
		if err != nil {
			return err
		}
		if len(facet) < 6 {
			return nil
		}
		count := int(le.Uint16(facet[4:]))
		for i := range count {
			if 6+4*i+4 > len(facet) {
				break
			}
			if le.Uint16(facet[6+4*i:]) == carAttrIdentifier {
				identifiers[string(key)] = le.Uint16(facet[8+4*i:])
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	identifier, found := uint16(0), false
	for _, name := range names {
		if identifier, found = identifiers[name]; found {
			break
		}
	}
	if !found {
		return nil, errors.New("no icon asset in catalog")
	}

	keyFormat, err := bom.block(bom.vars["KEYFORMAT"])
	if err != nil {
		return nil, errors.WithMessage(err, "read key format")
	}
	if len(keyFormat) < 12 {
		return nil, errors.New("bad key format")
	}
	attrCount := int(le.Uint32(keyFormat[8:]))
	if 12+4*attrCount > len(keyFormat) {
		return nil, errors.New("key format truncated")
	}
	identifierPos, appearancePos, idiomPos := -1, -1, -1
	for i := range attrCount {
		switch le.Uint32(keyFormat[12+4*i:]) {
		case carAttrIdentifier:
			identifierPos = i
		case carAttrAppearance:
			appearancePos = i
		case carAttrIdiom:
			idiomPos = i
		}
	}
	if identifierPos < 0 {
		return nil, errors.New("key format has no identifier")
	}

	var candidates []carRendition
	if err := bom.walkTree("RENDITIONS", func(key []byte, value uint32) error {
		if len(key) < 2*attrCount || le.Uint16(key[2*identifierPos:]) != identifier {
			return nil
		}
		c := carRendition{value: value}
		if appearancePos >= 0 {
			c.appearance = le.Uint16(key[2*appearancePos:])
		}
		if idiomPos >= 0 {
			c.marketing = le.Uint16(key[2*idiomPos:]) == carIdiomMarketing
		}
		candidates = append(candidates, c)
		return nil
	}); err != nil {
		return nil, err
	}

	var best *carRendition
	for i := range candidates {
		c := &candidates[i]
		head, err := bom.blockPrefix(c.value, csiHeaderSize)
		if err != nil || string(head[:4]) != "ISTC" {
			continue
		}
		layout := le.Uint16(head[36:])
		if layout < carLayoutImageMin || layout > carLayoutImageMax {
			continue
		}
		c.format = string(head[24:28])
		switch c.format {
		case carFormatARGB, carFormatGA8:
			c.gray = c.format == carFormatGA8
			c.width, c.height = int(le.Uint32(head[12:])), int(le.Uint32(head[16:]))
		case carFormatJPEG, carFormatPNG:
			// e.g. the App Store icon, whose header has no size
			if c.encoded, c.width, c.height, err = readEncodedRendition(bom, c.value, c.format); err != nil {
				continue
			}
		default:
			continue
		}
		if c.width <= 0 || c.height <= 0 || c.width > maxIconSide || c.height > maxIconSide {
			continue
		}
		if best == nil || c.betterThan(best) {
			best = c
		}
	}
	if best == nil {
		return nil, errors.New("no bitmap rendition for icon asset")
	}
	if best.encoded != nil {
		var img image.Image
		if best.format == carFormatJPEG {
			img, err = jpeg.Decode(bytes.NewReader(best.encoded))
		} else {
			img, err = png.Decode(bytes.NewReader(best.encoded))
		}
		if err != nil {
			return nil, errors.WithMessage(err, "decode rendition image")
		}
		return toNRGBA(img), nil
	}
	csi, err := bom.block(best.value)
	if err != nil {
		return nil, err
	}
	return decodeCsiBitmap(csi, best.width, best.height, best.gray)
}

// readEncodedRendition returns the image file in a rendition's raw data ("RAWD") and its size.
func readEncodedRendition(bom *bomStore, value uint32, format string) ([]byte, int, int, error) {
	csi, err := bom.block(value)
	if err != nil {
		return nil, 0, 0, err
	}
	data, err := csiData(csi)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(data) < 12 || string(data[:4]) != "DWAR" {
		return nil, 0, 0, errors.New("unsupported raw rendition")
	}
	n := int(binary.LittleEndian.Uint32(data[8:]))
	if n < 0 || 12+n > len(data) {
		return nil, 0, 0, errors.New("raw rendition truncated")
	}
	encoded := data[12 : 12+n]
	var cfg image.Config
	if format == carFormatJPEG {
		cfg, err = jpeg.DecodeConfig(bytes.NewReader(encoded))
	} else {
		cfg, err = png.DecodeConfig(bytes.NewReader(encoded))
	}
	if err != nil {
		return nil, 0, 0, err
	}
	return encoded, cfg.Width, cfg.Height, nil
}

// maxRowPadding bounds the alignment padding at the end of bitmap rows, e.g. 720 byte rows are
// stored as 736 bytes in some catalogs.
const maxRowPadding = 128

func decodeCsiBitmap(csi []byte, width, height int, gray bool) (*image.NRGBA, error) {
	le := binary.LittleEndian
	data, err := csiData(csi)
	if err != nil {
		return nil, err
	}
	if len(data) < 16 || string(data[:4]) != "MLEC" {
		return nil, errors.New("unsupported rendition data")
	}
	compression, field := le.Uint32(data[8:]), int(le.Uint32(data[12:]))
	bpp := 4
	if gray {
		bpp = 2
	}
	rowBytes := width * bpp
	var pixels []byte
	if len(data) >= 20 && string(data[16:20]) == "KCBC" {
		// the image is split into bands of rows, each compressed on its own
		pos := 16
		for range field {
			if pos+20 > len(data) || string(data[pos:pos+4]) != "KCBC" {
				return nil, errors.New("bad csi chunk")
			}
			rows, n := int(le.Uint32(data[pos+12:])), int(le.Uint32(data[pos+16:]))
			if rows <= 0 || n < 0 || pos+20+n > len(data) || len(pixels)/rowBytes+rows > height {
				return nil, errors.New("csi chunk out of bounds")
			}
			chunk, err := decompressRows(compression, data[pos+20:pos+20+n], rows, rowBytes)
			if err != nil {
				return nil, err
			}
			pixels = append(pixels, chunk...)
			pos += 20 + n
		}
	} else {
		if field < 0 || 16+field > len(data) {
			return nil, errors.New("csi data out of bounds")
		}
		if pixels, err = decompressRows(compression, data[16:16+field], height, rowBytes); err != nil {
			return nil, err
		}
	}
	if len(pixels) != height*rowBytes {
		return nil, errors.Errorf("decoded %d bytes, want %d", len(pixels), height*rowBytes)
	}
	if gray {
		return premultipliedGrayToNRGBA(pixels, width, height), nil
	}
	return premultipliedBGRAToNRGBA(pixels, width, height), nil
}

// csiData returns the rendition data that follows the CSI header and its TLV metadata.
func csiData(csi []byte) ([]byte, error) {
	le := binary.LittleEndian
	tlvLen, dataLen := int(le.Uint32(csi[168:])), int(le.Uint32(csi[180:]))
	start := csiHeaderSize + tlvLen
	if tlvLen < 0 || dataLen < 0 || start+dataLen > len(csi) {
		return nil, errors.New("csi data truncated")
	}
	return csi[start : start+dataLen], nil
}

// decompressRows returns rows of rowBytes each, with any row padding removed.
func decompressRows(compression uint32, data []byte, rows, rowBytes int) ([]byte, error) {
	out, err := decompressCsi(compression, data, rows*(rowBytes+maxRowPadding))
	if err != nil {
		return nil, err
	}
	stride := len(out) / rows
	if len(out)%rows != 0 || stride < rowBytes {
		return nil, errors.Errorf("decompressed %d bytes for %d rows of %d bytes", len(out), rows, rowBytes)
	}
	if stride == rowBytes {
		return out, nil
	}
	pixels := make([]byte, 0, rows*rowBytes)
	for y := range rows {
		pixels = append(pixels, out[y*stride:y*stride+rowBytes]...)
	}
	return pixels, nil
}

// decompressCsi decompresses rendition data, which must not exceed limit bytes.
func decompressCsi(compression uint32, data []byte, limit int) ([]byte, error) {
	var out []byte
	var err error
	switch compression {
	case 0:
		out = data
	case 2:
		out, err = inflate(data, limit)
	case 3:
		if bytes.HasPrefix(data, []byte("bvx")) {
			out, err = decompressLzfse(data, limit)
		} else {
			out, err = decodeLZVN(nil, data, limit)
		}
	case 4:
		out, err = decompressLzfse(data, limit)
	default:
		return nil, errors.Errorf("unsupported csi compression %d", compression)
	}
	if err != nil {
		return nil, errors.WithMessagef(err, "decompress csi type %d", compression)
	}
	if len(out) > limit {
		return nil, errors.Errorf("decompressed more than %d bytes", limit)
	}
	return out, nil
}

// inflate handles the gzip, zlib and raw deflate streams that all appear as "zip" compression.
func inflate(data []byte, limit int) ([]byte, error) {
	var r io.Reader
	switch {
	case len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b:
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		r = gz
	case len(data) >= 2 && data[0]&0x0f == 8 && (uint16(data[0])<<8|uint16(data[1]))%31 == 0:
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		r = zr
	default:
		r = flate.NewReader(bytes.NewReader(data))
	}
	return io.ReadAll(io.LimitReader(r, int64(limit)+1))
}

// The decoder reads compressed block headers into a fixed 752 byte struct before seeking back to
// their real, shorter end, so a block near the end of the input fails with EOF without padding.
const lzfseHeaderPadding = 1024

// decompressLzfse checks the block sizes first, since the library decoder trusts them and
// allocates accordingly. The library only handles bvx2 blocks correctly, so streams without them
// are decoded here. Apple's encoder doesn't mix bvx2 and LZVN blocks in one stream.
func decompressLzfse(data []byte, limit int) (out []byte, err error) {
	blocks, total, err := lzfseBlocks(data, limit)
	if err != nil {
		return nil, err
	}
	hasV2, hasLZVN := false, false
	for _, b := range blocks {
		hasV2 = hasV2 || b.magic == "bvx2"
		hasLZVN = hasLZVN || b.magic == "bvxn"
	}
	if hasV2 && hasLZVN {
		return nil, errors.New("lzfse stream mixes block types")
	}
	if !hasV2 {
		out = make([]byte, 0, total)
		for _, b := range blocks {
			if b.magic == "bvx-" {
				out = append(out, b.payload...)
				continue
			}
			start := len(out)
			if out, err = decodeLZVN(out, b.payload, start+b.rawSize); err != nil {
				return nil, err
			}
			if len(out)-start != b.rawSize {
				return nil, errors.New("lzvn block size mismatch")
			}
		}
		return out, nil
	}
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("lzfse: %v", r)
		}
	}()
	padded := make([]byte, len(data)+lzfseHeaderPadding)
	copy(padded, data)
	if out, err = io.ReadAll(lzfse.NewReader(bytes.NewReader(padded))); err != nil {
		return nil, err
	}
	if len(out) != total {
		return nil, errors.Errorf("lzfse decoded %d bytes, blocks declare %d", len(out), total)
	}
	return out, nil
}

type lzfseBlock struct {
	magic   string
	rawSize int
	payload []byte // only for uncompressed and LZVN blocks
}

// lzfseBlocks splits an LZFSE stream into blocks, checking that they fit and decode to at most
// limit bytes, and returns their total decoded size.
func lzfseBlocks(data []byte, limit int) ([]lzfseBlock, int, error) {
	le := binary.LittleEndian
	raw := 0
	var blocks []lzfseBlock
	for pos := 0; ; {
		if pos+4 > len(data) {
			return nil, 0, errors.New("lzfse stream truncated")
		}
		b := lzfseBlock{magic: string(data[pos : pos+4])}
		var size, headerSize int
		switch b.magic {
		case "bvx$":
			return blocks, raw, nil
		case "bvx-":
			headerSize = 8
			if pos+headerSize > len(data) {
				return nil, 0, errors.New("lzfse block truncated")
			}
			b.rawSize = int(le.Uint32(data[pos+4:]))
			size = headerSize + b.rawSize
		case "bvxn":
			headerSize = 12
			if pos+headerSize > len(data) {
				return nil, 0, errors.New("lzfse block truncated")
			}
			b.rawSize = int(le.Uint32(data[pos+4:]))
			size = headerSize + int(le.Uint32(data[pos+8:]))
		case "bvx2":
			if pos+32 > len(data) {
				return nil, 0, errors.New("lzfse block truncated")
			}
			b.rawSize = int(le.Uint32(data[pos+4:]))
			f0, f1, f2 := le.Uint64(data[pos+8:]), le.Uint64(data[pos+16:]), le.Uint64(data[pos+24:])
			literalBytes := int(f0 >> 20 & (1<<20 - 1))
			lmdBytes := int(f1 >> 40 & (1<<20 - 1))
			size = int(uint32(f2)) + literalBytes + lmdBytes
		default:
			return nil, 0, errors.Errorf("unsupported lzfse block %q", b.magic)
		}
		raw += b.rawSize
		if b.rawSize < 0 || raw > limit || size <= 0 || pos+size > len(data) {
			return nil, 0, errors.New("lzfse block out of bounds")
		}
		if headerSize > 0 {
			b.payload = data[pos+headerSize : pos+size]
		}
		blocks = append(blocks, b)
		pos += size
	}
}

func premultipliedGrayToNRGBA(pix []byte, width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		src := pix[y*width*2 : (y+1)*width*2]
		dst := img.Pix[y*img.Stride : y*img.Stride+width*4]
		for x := range width {
			v, a := src[x*2], src[x*2+1]
			if a != 0 && a != 255 {
				v = unpremultiply(v, a)
			}
			dst[x*4], dst[x*4+1], dst[x*4+2], dst[x*4+3] = v, v, v, a
		}
	}
	return img
}

func premultipliedBGRAToNRGBA(pix []byte, width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	stride := width * 4
	for y := range height {
		src := pix[y*stride : (y+1)*stride]
		dst := img.Pix[y*img.Stride : y*img.Stride+stride]
		for x := 0; x < stride; x += 4 {
			b, g, r, a := src[x], src[x+1], src[x+2], src[x+3]
			if a != 0 && a != 255 {
				r, g, b = unpremultiply(r, a), unpremultiply(g, a), unpremultiply(b, a)
			}
			dst[x], dst[x+1], dst[x+2], dst[x+3] = r, g, b, a
		}
	}
	return img
}
