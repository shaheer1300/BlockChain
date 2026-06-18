package types

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// MaxCanonicalSliceLen caps the length prefix of any variable-length field
// in the canonical encoding to prevent DoS via inflated length prefixes
// when a decoder is added in a later phase. 32 MiB is far larger than any
// legitimate transaction or block field but small enough to bound memory.
const MaxCanonicalSliceLen = 32 << 20

// ErrSliceTooLong is returned when a variable-length field exceeds
// MaxCanonicalSliceLen.
var ErrSliceTooLong = errors.New("types: canonical slice exceeds maximum length")

// canonicalEncoder writes the deterministic little-endian binary form of
// consensus data structures. It is the single source of truth for any
// byte sequence that feeds a consensus hash (transaction ID, block hash,
// Merkle leaf, signature preimage).
//
// Never change this encoding without updating consensus tests; a change
// constitutes a hard fork.
//
// Errors are sticky: once Err is non-nil, subsequent writes are no-ops.
// Callers check Err once at the end of an encoding sequence.
type canonicalEncoder struct {
	w     io.Writer
	err   error
	cache [8]byte
}

func newCanonicalEncoder(w io.Writer) *canonicalEncoder {
	return &canonicalEncoder{w: w}
}

// Err returns the first error encountered by the encoder, or nil.
func (e *canonicalEncoder) Err() error { return e.err }

func (e *canonicalEncoder) write(p []byte) {
	if e.err != nil {
		return
	}
	if _, err := e.w.Write(p); err != nil {
		e.err = err
	}
}

func (e *canonicalEncoder) writeUint32(v uint32) {
	if e.err != nil {
		return
	}
	binary.LittleEndian.PutUint32(e.cache[:4], v)
	e.write(e.cache[:4])
}

func (e *canonicalEncoder) writeUint64(v uint64) {
	if e.err != nil {
		return
	}
	binary.LittleEndian.PutUint64(e.cache[:8], v)
	e.write(e.cache[:8])
}

func (e *canonicalEncoder) writeInt64(v int64) {
	e.writeUint64(uint64(v))
}

func (e *canonicalEncoder) writeBytes(b []byte) {
	e.write(b)
}

func (e *canonicalEncoder) writeHash(h Hash32) {
	e.write(h[:])
}

func (e *canonicalEncoder) writeAddress(a Address) {
	e.write(a[:])
}

// writeLen writes a 4-byte little-endian length prefix used for
// slice-of-items (e.g. number of inputs, number of outputs). Returns
// silently after setting Err if n is negative or exceeds the cap.
func (e *canonicalEncoder) writeLen(n int) {
	if e.err != nil {
		return
	}
	if n < 0 || n > MaxCanonicalSliceLen {
		e.err = fmt.Errorf("%w: length %d", ErrSliceTooLong, n)
		return
	}
	e.writeUint32(uint32(n))
}

// writeVarBytes writes a 4-byte little-endian length prefix followed by
// the byte payload. Used for opaque blobs (signatures, public keys).
func (e *canonicalEncoder) writeVarBytes(b []byte) {
	e.writeLen(len(b))
	e.writeBytes(b)
}
