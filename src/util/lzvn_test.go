package util

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lzvnTestData mirrors the generator for testdata/lzvn.bin, which was made by Apple's Compression
// framework (algorithm 0x900, raw LZVN). It hits every opcode: long and short literal runs, matches
// at small, medium and large distances, and long matches that continue at the previous distance.
func lzvnTestData() []byte {
	seed := uint32(12345)
	rnd := func(n int) []byte {
		out := make([]byte, n)
		for i := range out {
			seed = (seed*1103515245 + 12345) & 0x7fffffff
			out[i] = byte(seed >> 16)
		}
		return out
	}
	seg := rnd(300)
	var b bytes.Buffer
	b.Write(rnd(5))
	b.WriteString("abcabcabcabc")
	b.Write(rnd(40))
	b.Write(seg)
	b.Write(rnd(3000))
	b.Write(seg[:40])
	b.Write(rnd(17000))
	b.Write(seg)
	b.Write(bytes.Repeat([]byte("x"), 600))
	b.Write(rnd(3))
	b.Write(seg[10:200])
	b.Write(rnd(100))
	b.Write(seg[:12])
	b.Write(rnd(2))
	b.Write(seg[:12])
	return b.Bytes()
}

func TestDecodeLZVN(t *testing.T) {
	src, err := os.ReadFile("testdata/lzvn.bin")
	require.NoError(t, err)
	want := lzvnTestData()

	got, err := decodeLZVN(nil, src, len(want))
	require.NoError(t, err)
	assert.Equal(t, want, got)

	// output past the expected size is rejected rather than allocated
	_, err = decodeLZVN(nil, src, len(want)-1)
	assert.Error(t, err)

	// wrapped in an LZFSE block, as compression type 4 stores it
	le := binary.LittleEndian
	stream := append([]byte("bvxn"), le.AppendUint32(le.AppendUint32(nil, uint32(len(want))), uint32(len(src)))...)
	stream = append(append(stream, src...), "bvx$"...)
	got, err = decompressLzfse(stream, len(want))
	require.NoError(t, err)
	assert.Equal(t, want, got)

	got, err = decompressCsi(3, src, len(want))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestDecodeLZVNMalformed(t *testing.T) {
	src, err := os.ReadFile("testdata/lzvn.bin")
	require.NoError(t, err)
	want := len(lzvnTestData())
	for i := 0; i < len(src); i += 7 {
		broken := bytes.Clone(src)
		broken[i] ^= 0x5a
		_, _ = decodeLZVN(nil, broken, want)
	}
	for n := 0; n < len(src); n += 101 {
		_, err := decodeLZVN(nil, src[:n], want)
		assert.Error(t, err)
	}
	for _, op := range []byte{0x1e, 0x70, 0xd5} {
		_, err := decodeLZVN(nil, []byte{op, 0, 0, 0}, 100)
		assert.Error(t, err, "opcode %#x", op)
	}
	// nops, a one byte literal, then a 3 byte match at distance 1
	got, err := decodeLZVN(nil, []byte{0x0e, 0x16, 0xe1, 'a', 0x00, 0x01, 0x06}, 100)
	require.NoError(t, err)
	assert.Equal(t, []byte("aaaa"), got)

	// a match reaching before the start of the output
	_, err = decodeLZVN(nil, []byte{0x00, 0x05, 0x06}, 100)
	assert.Error(t, err)
}
