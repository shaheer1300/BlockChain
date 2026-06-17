// Package types defines the core blockchain data structures and the
// deterministic canonical binary encoding used to derive consensus hashes
// (transaction IDs, block hashes, Merkle leaves, signature preimages).
//
// JSON tags on consensus structs exist solely for API responses. The
// canonical encoder defined in encoding.go is the single source of truth
// for any byte sequence that feeds a consensus hash. Changing the
// canonical encoding constitutes a hard fork and must be accompanied by
// updated consensus tests.
package types

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// HashSize is the length in bytes of a Hash32.
const HashSize = 32

// AddressSize is the length in bytes of an Address (HASH160 of a public key).
const AddressSize = 20

// Hash32 is a fixed-size 32-byte digest used for transaction IDs, block
// hashes, Merkle roots, and any other consensus-relevant identifier.
type Hash32 [HashSize]byte

// ZeroHash is the all-zero Hash32 sentinel.
var ZeroHash = Hash32{}

// String returns the lowercase hex encoding of h.
func (h Hash32) String() string { return hex.EncodeToString(h[:]) }

// Bytes returns a defensive copy of h. Do not give callers direct access 
// to the underlying array, as it would allow them to mutate the hash value and break invariants. This method provides a safe way to get the byte representation of the hash without exposing the internal state.
func (h Hash32) Bytes() []byte {
	out := make([]byte, HashSize)
	copy(out, h[:])  // creating a copy here
	return out
}

// IsZero reports whether h equals ZeroHash.
func (h Hash32) IsZero() bool { return h == ZeroHash }

// SetHex parses a 64-character hex string into h. 
// Use a pointer receiver (*Hash32) to modify the original hash
// rather than a copy.
func (h *Hash32) SetHex(s string) error {
	// A 32-byte hash becomes 64 hex characters because: 1 byte = 2 hex characters
	// 32 bytes = 64 hex characters so HashSize * 2
	if len(s) != HashSize*2 {
		return fmt.Errorf("types: hash hex must be %d chars, got %d", HashSize*2, len(s))
	}
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return fmt.Errorf("types: invalid hash hex: %w", err)
	}
	copy(h[:], decoded)
	return nil
}

// MarshalJSON encodes h as a lowercase hex string.
func (h Hash32) MarshalJSON() ([]byte, error) { return json.Marshal(h.String()) }

// UnmarshalJSON decodes a hex-string JSON value into h.
func (h *Hash32) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return h.SetHex(s)
}

// HashFromBytes constructs a Hash32 from a 32-byte slice.
func HashFromBytes(b []byte) (Hash32, error) {
	var h Hash32
	if len(b) != HashSize {
		return h, fmt.Errorf("types: hash requires %d bytes, got %d", HashSize, len(b))
	}
	copy(h[:], b)
	return h, nil
}

// Address is a 20-byte payment recipient identifier (HASH160 of a pubkey).
type Address [AddressSize]byte

// ZeroAddress is the all-zero Address sentinel.
var ZeroAddress = Address{}

// String returns the lowercase hex encoding of a.
func (a Address) String() string { return hex.EncodeToString(a[:]) }

// IsZero reports whether a equals ZeroAddress.
func (a Address) IsZero() bool { return a == ZeroAddress }

// MarshalJSON encodes a as a lowercase hex string.
func (a Address) MarshalJSON() ([]byte, error) { return json.Marshal(a.String()) }

// UnmarshalJSON decodes a hex-string JSON value into a.
func (a *Address) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if len(s) != AddressSize*2 {
		return fmt.Errorf("types: address hex must be %d chars, got %d", AddressSize*2, len(s))
	}
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return fmt.Errorf("types: invalid address hex: %w", err)
	}
	copy(a[:], decoded)
	return nil
}

// Amount represents a quantity of the native coin in its smallest
// indivisible unit. uint64 matches Bitcoin's value field width.
type Amount uint64

// ErrAmountOverflow is returned by SafeAdd and SafeSub when the result
// cannot be represented in the Amount range (covers both over- and
// underflow; both indicate "arithmetic out of representable range" with
// the same caller recovery path).
var ErrAmountOverflow = errors.New("types: amount overflow")

// SafeAdd returns a + b, or ErrAmountOverflow on wrap.
func (a Amount) SafeAdd(b Amount) (Amount, error) {
	sum := a + b
	if sum < a {
		return 0, ErrAmountOverflow
	}
	return sum, nil
}

// SafeSub returns a - b, or ErrAmountOverflow on underflow.
func (a Amount) SafeSub(b Amount) (Amount, error) {
	if b > a {
		return 0, ErrAmountOverflow
	}
	return a - b, nil
}

// doubleSHA256 computes SHA-256(SHA-256(data)) and returns the result as a
// Hash32. Kept unexported to keep the public hash API in internal/crypto
// and prevent internal/crypto from importing back into types.
func doubleSHA256(data []byte) Hash32 {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return Hash32(second)
}
