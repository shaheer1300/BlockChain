package crypto

// ripemd160Hash is isolated in its own file so that replacing the RIPEMD-160
// implementation (e.g. switching to golang.org/x/crypto/ripemd160) requires
// changing only this one function.

// ripemd160Hash computes RIPEMD-160 of data using a pure-Go implementation.
// RIPEMD-160 is a 20-round, 160-bit Merkle-Damgård hash function used in
// Bitcoin's HASH160 (address derivation) construction.
func ripemd160Hash(data []byte) [20]byte {
	var digest [20]byte
	r := newRipemd160()
	r.Write(data)
	copy(digest[:], r.Sum(nil))
	return digest
}

// ----- Pure-Go RIPEMD-160 (RFC 2286 / ISO/IEC 10118-3) -----
// This avoids a dependency on golang.org/x/crypto for a single 20-byte hash.
// The implementation follows the reference specification exactly.

const (
	rBlockSize  = 64
	rDigestSize = 20
)

type ripemd160State struct {
	s   [5]uint32
	x   [rBlockSize]byte
	nx  int
	len uint64
}

func newRipemd160() *ripemd160State {
	r := new(ripemd160State)
	r.Reset()
	return r
}

func (r *ripemd160State) Reset() {
	r.s[0] = 0x67452301
	r.s[1] = 0xEFCDAB89
	r.s[2] = 0x98BADCFE
	r.s[3] = 0x10325476
	r.s[4] = 0xC3D2E1F0
	r.nx = 0
	r.len = 0
}

func (r *ripemd160State) Write(p []byte) (int, error) {
	nn := len(p)
	r.len += uint64(nn)
	if r.nx > 0 {
		n := copy(r.x[r.nx:], p)
		r.nx += n
		if r.nx == rBlockSize {
			rBlock(r, r.x[:])
			r.nx = 0
		}
		p = p[n:]
	}
	for len(p) >= rBlockSize {
		rBlock(r, p[:rBlockSize])
		p = p[rBlockSize:]
	}
	if len(p) > 0 {
		r.nx = copy(r.x[:], p)
	}
	return nn, nil
}

func (r *ripemd160State) Sum(in []byte) []byte {
	r0 := *r
	var digest [rDigestSize]byte
	r0.checkSum(digest[:])
	return append(in, digest[:]...)
}

func (r *ripemd160State) checkSum(out []byte) {
	tc := r.len
	var tmp [64]byte
	tmp[0] = 0x80
	if tc%64 < 56 {
		r.Write(tmp[0 : 56-tc%64])
	} else {
		r.Write(tmp[0 : 64+56-tc%64])
	}
	tc <<= 3
	for i := uint(0); i < 8; i++ {
		tmp[i] = byte(tc >> (8 * i))
	}
	r.Write(tmp[0:8])
	for i, s := range r.s {
		out[4*i] = byte(s)
		out[4*i+1] = byte(s >> 8)
		out[4*i+2] = byte(s >> 16)
		out[4*i+3] = byte(s >> 24)
	}
}

// RIPEMD-160 round constants and permutations (see ISO/IEC 10118-3).
var (
	rK  = [80]uint32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x5A827999, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x6ED9EBA1, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0x8F1BBCDC, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E, 0xA953FD4E}
	rKK = [80]uint32{0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x50A28BE6, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x5C4DD124, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x6D703EF3, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0x7A6D76E9, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	rR  = [80]uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 7, 4, 13, 1, 10, 6, 15, 3, 12, 0, 9, 5, 2, 14, 11, 8, 3, 10, 14, 4, 9, 15, 8, 1, 2, 7, 0, 6, 13, 11, 5, 12, 1, 9, 11, 10, 0, 8, 12, 4, 13, 3, 7, 15, 14, 5, 6, 2, 4, 0, 5, 9, 7, 12, 2, 10, 14, 1, 3, 8, 11, 6, 15, 13}
	rRR = [80]uint8{5, 14, 7, 0, 9, 2, 11, 4, 13, 6, 15, 8, 1, 10, 3, 12, 6, 11, 3, 7, 0, 13, 5, 10, 14, 15, 8, 12, 4, 9, 1, 2, 15, 5, 1, 3, 7, 14, 6, 9, 11, 8, 12, 2, 10, 0, 4, 13, 8, 6, 4, 1, 3, 11, 15, 0, 5, 12, 2, 13, 9, 7, 10, 14, 12, 15, 10, 4, 1, 5, 8, 7, 6, 2, 13, 14, 0, 3, 9, 11}
	rS  = [80]uint8{11, 14, 15, 12, 5, 8, 7, 9, 11, 13, 14, 15, 6, 7, 9, 8, 7, 6, 8, 13, 11, 9, 7, 15, 7, 12, 15, 9, 11, 7, 13, 12, 11, 13, 6, 7, 14, 9, 13, 15, 14, 8, 13, 6, 5, 12, 7, 5, 11, 12, 14, 15, 14, 15, 9, 8, 9, 14, 5, 6, 8, 6, 5, 12, 9, 15, 5, 11, 6, 8, 13, 12, 5, 12, 13, 14, 11, 8, 5, 6}
	rSS = [80]uint8{8, 9, 9, 11, 13, 15, 15, 5, 7, 7, 8, 11, 14, 14, 12, 6, 9, 13, 15, 7, 12, 8, 9, 11, 7, 7, 12, 7, 6, 15, 13, 11, 9, 7, 15, 11, 8, 6, 6, 14, 12, 13, 5, 14, 13, 13, 7, 5, 15, 5, 8, 11, 14, 14, 6, 14, 6, 9, 12, 9, 12, 5, 15, 8, 8, 5, 12, 9, 12, 5, 14, 6, 8, 13, 6, 5, 15, 13, 11, 11}
)

func rol(x uint32, n uint8) uint32 { return (x << n) | (x >> (32 - n)) }

func rBlock(r *ripemd160State, p []byte) {
	var X [16]uint32
	for i := 0; i < 16; i++ {
		j := i * 4
		X[i] = uint32(p[j]) | uint32(p[j+1])<<8 | uint32(p[j+2])<<16 | uint32(p[j+3])<<24
	}
	a, b, c, d, e := r.s[0], r.s[1], r.s[2], r.s[3], r.s[4]
	aa, bb, cc, dd, ee := a, b, c, d, e

	for j := 0; j < 80; j++ {
		var f, ff uint32
		switch {
		case j < 16:
			f = b ^ c ^ d
			ff = (bb & dd) | (cc &^ dd)
		case j < 32:
			f = (b & c) | (^b & d)
			ff = (bb | ^cc) ^ dd
		case j < 48:
			f = (b | ^c) ^ d
			ff = (bb & cc) | (^bb & dd)
		case j < 64:
			f = (b & d) | (c &^ d)
			ff = (bb | ^cc) ^ dd
		default:
			f = b ^ (c | ^d)
			ff = bb ^ (cc | ^dd)
		}
		t := rol(a+f+X[rR[j]]+rK[j], rS[j]) + e
		a, b, c, d, e = e, t, b, rol(c, 10), d
		t = rol(aa+ff+X[rRR[j]]+rKK[j], rSS[j]) + ee
		aa, bb, cc, dd, ee = ee, t, bb, rol(cc, 10), dd
	}
	t := r.s[1] + c + dd
	r.s[1] = r.s[2] + d + ee
	r.s[2] = r.s[3] + e + aa
	r.s[3] = r.s[4] + a + bb
	r.s[4] = r.s[0] + b + cc
	r.s[0] = t
}
