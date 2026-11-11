package util

import (
	"github.com/pkg/errors"
)

// decodeLZVN appends the decoded LZVN stream src to dst, which also serves as match history, and
// stops at the end of stream opcode or once len(dst) reaches limit.
// Opcode layout follows lzvn_decode_base.c from Apple's reference LZFSE implementation.
func decodeLZVN(dst, src []byte, limit int) ([]byte, error) {
	pos, dist := 0, 0
	need := func(n int) error {
		if pos+n > len(src) {
			return errors.New("lzvn: truncated input")
		}
		return nil
	}
	for {
		if err := need(1); err != nil {
			return nil, err
		}
		op := src[pos]
		var literals, match, size int
		switch {
		case op == 0x06: // end of stream
			return dst, nil
		case op == 0x0e || op == 0x16: // nop
			pos++
			continue
		case op >= 0xa0 && op <= 0xbf: // medium distance: 101LLMMM DDDDDDMM DDDDDDDD
			if err := need(3); err != nil {
				return nil, err
			}
			b1, b2 := int(src[pos+1]), int(src[pos+2])
			literals = int(op>>3) & 3
			match = (int(op&7)<<2 | b1&3) + 3
			dist = b2<<6 | b1>>2
			size = 3
		case op == 0xe0: // large literal: 11100000 LLLLLLLL
			if err := need(2); err != nil {
				return nil, err
			}
			literals, size = int(src[pos+1])+16, 2
		case op > 0xe0 && op <= 0xef: // small literal: 1110LLLL
			literals, size = int(op&0x0f), 1
		case op == 0xf0: // large match with the previous distance: 11110000 MMMMMMMM
			if err := need(2); err != nil {
				return nil, err
			}
			match, size = int(src[pos+1])+16, 2
		case op > 0xf0: // small match with the previous distance: 1111MMMM
			match, size = int(op&0x0f), 1
		case op >= 0x70 && op <= 0x7f, op >= 0xd0 && op <= 0xdf:
			return nil, errors.Errorf("lzvn: undefined opcode %#x", op)
		default: // LLMMMDDD, distance encoding depends on the low bits
			literals, match = int(op>>6), int(op>>3&7)+3
			switch op & 7 {
			case 6: // previous distance
				if op < 0x40 {
					return nil, errors.Errorf("lzvn: undefined opcode %#x", op)
				}
				size = 1
			case 7: // large distance: LLMMM111 DDDDDDDD DDDDDDDD
				if err := need(3); err != nil {
					return nil, err
				}
				dist, size = int(src[pos+1])|int(src[pos+2])<<8, 3
			default: // small distance: LLMMMDDD DDDDDDDD
				if err := need(2); err != nil {
					return nil, err
				}
				dist, size = int(op&7)<<8|int(src[pos+1]), 2
			}
		}
		pos += size
		if err := need(literals); err != nil {
			return nil, err
		}
		if len(dst)+literals+match > limit {
			return nil, errors.New("lzvn: output exceeds expected size")
		}
		dst = append(dst, src[pos:pos+literals]...)
		pos += literals
		if match > 0 {
			if dist <= 0 || dist > len(dst) {
				return nil, errors.Errorf("lzvn: bad match distance %d", dist)
			}
			// byte by byte, since a match may overlap its own output
			for range match {
				dst = append(dst, dst[len(dst)-dist])
			}
		}
	}
}
