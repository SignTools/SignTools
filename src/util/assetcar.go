package util

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"image"
	"io"

	"github.com/aixiansheng/lzfse"
	"github.com/pkg/errors"
)

// Compiled asset catalogs (Assets.car) are BOM files. Their RENDITIONS tree maps attribute keys to
// CSI-wrapped images, and FACETKEYS maps asset names to the identifier attribute of their renditions.

const (
	carAttrAppearance = 7
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

type carRendition struct {
	value      uint32
	appearance uint16
	width      int
	height     int
}

// betterThan prefers the default appearance over dark and tinted variants, then the bigger image.
func (c *carRendition) betterThan(o *carRendition) bool {
	if (c.appearance == 0) != (o.appearance == 0) {
		return c.appearance == 0
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
	identifierPos, appearancePos := -1, -1
	for i := range attrCount {
		switch le.Uint32(keyFormat[12+4*i:]) {
		case carAttrIdentifier:
			identifierPos = i
		case carAttrAppearance:
			appearancePos = i
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
		if layout < carLayoutImageMin || layout > carLayoutImageMax || string(head[24:28]) != "BGRA" {
			continue
		}
		c.width, c.height = int(le.Uint32(head[12:])), int(le.Uint32(head[16:]))
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
	csi, err := bom.block(best.value)
	if err != nil {
		return nil, err
	}
	return decodeCsiBitmap(csi, best.width, best.height)
}

func decodeCsiBitmap(csi []byte, width, height int) (*image.NRGBA, error) {
	le := binary.LittleEndian
	tlvLen, dataLen := int(le.Uint32(csi[168:])), int(le.Uint32(csi[180:]))
	start := csiHeaderSize + tlvLen
	if tlvLen < 0 || dataLen < 0 || start+dataLen > len(csi) {
		return nil, errors.New("csi data truncated")
	}
	data := csi[start : start+dataLen]
	if len(data) < 16 || string(data[:4]) != "MLEC" {
		return nil, errors.New("unsupported rendition data")
	}
	compression, field := le.Uint32(data[8:]), int(le.Uint32(data[12:]))
	stride := width * 4
	var pixels []byte
	if len(data) >= 20 && string(data[16:20]) == "KCBC" {
		// the image is split into bands of rows, each compressed on its own
		pos := 16
		for range field {
			if pos+20 > len(data) || string(data[pos:pos+4]) != "KCBC" {
				return nil, errors.New("bad csi chunk")
			}
			rows, n := int(le.Uint32(data[pos+12:])), int(le.Uint32(data[pos+16:]))
			if rows < 0 || n < 0 || pos+20+n > len(data) || len(pixels)/stride+rows > height {
				return nil, errors.New("csi chunk out of bounds")
			}
			chunk, err := decompressCsi(compression, data[pos+20:pos+20+n], rows*stride)
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
		var err error
		if pixels, err = decompressCsi(compression, data[16:16+field], height*stride); err != nil {
			return nil, err
		}
	}
	if len(pixels) != height*stride {
		return nil, errors.Errorf("decoded %d bytes, want %d", len(pixels), height*stride)
	}
	return premultipliedBGRAToNRGBA(pixels, width, height), nil
}

func decompressCsi(compression uint32, data []byte, want int) ([]byte, error) {
	var out []byte
	var err error
	switch compression {
	case 0:
		out = data
	case 2:
		out, err = io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(data)), int64(want)+1))
	case 3, 4:
		out, err = decompressLzfse(data, want)
	default:
		return nil, errors.Errorf("unsupported csi compression %d", compression)
	}
	if err != nil {
		return nil, errors.WithMessagef(err, "decompress csi type %d", compression)
	}
	if len(out) != want {
		return nil, errors.Errorf("decompressed %d bytes, want %d", len(out), want)
	}
	return out, nil
}

// The decoder reads compressed block headers into a fixed 752 byte struct before seeking back to
// their real, shorter end, so a block near the end of the input fails with EOF without padding.
const lzfseHeaderPadding = 1024

// decompressLzfse checks the block sizes first, since the decoder trusts them and allocates
// accordingly, and recovers its panics on malformed input.
func decompressLzfse(data []byte, want int) (out []byte, err error) {
	if err := checkLzfseBlocks(data, want); err != nil {
		return nil, err
	}
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("lzfse: %v", r)
		}
	}()
	padded := make([]byte, len(data)+lzfseHeaderPadding)
	copy(padded, data)
	return io.ReadAll(lzfse.NewReader(bytes.NewReader(padded)))
}

func checkLzfseBlocks(data []byte, want int) error {
	le := binary.LittleEndian
	raw := 0
	for pos := 0; ; {
		if pos+4 > len(data) {
			return errors.New("lzfse stream truncated")
		}
		magic := string(data[pos : pos+4])
		var size, rawSize int
		switch magic {
		case "bvx$":
			if raw != want {
				return errors.Errorf("lzfse stream has %d raw bytes, want %d", raw, want)
			}
			return nil
		case "bvx-":
			if pos+8 > len(data) {
				return errors.New("lzfse block truncated")
			}
			rawSize = int(le.Uint32(data[pos+4:]))
			size = 8 + rawSize
		case "bvxn":
			if pos+12 > len(data) {
				return errors.New("lzfse block truncated")
			}
			rawSize = int(le.Uint32(data[pos+4:]))
			size = 12 + int(le.Uint32(data[pos+8:]))
		case "bvx2":
			if pos+32 > len(data) {
				return errors.New("lzfse block truncated")
			}
			rawSize = int(le.Uint32(data[pos+4:]))
			f0, f1, f2 := le.Uint64(data[pos+8:]), le.Uint64(data[pos+16:]), le.Uint64(data[pos+24:])
			literalBytes := int(f0 >> 20 & (1<<20 - 1))
			lmdBytes := int(f1 >> 40 & (1<<20 - 1))
			size = int(uint32(f2)) + literalBytes + lmdBytes
		default:
			return errors.Errorf("unsupported lzfse block %q", magic)
		}
		raw += rawSize
		if rawSize < 0 || raw > want || size <= 0 || pos+size > len(data) {
			return errors.New("lzfse block out of bounds")
		}
		pos += size
	}
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
