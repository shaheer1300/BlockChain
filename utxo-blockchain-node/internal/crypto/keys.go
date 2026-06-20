package crypto

import (
	"errors"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// PrivateKey wraps the secp256k1 private key. Keeping it as a named type
// prevents callers from accidentally mixing it with other byte slices.
type PrivateKey = secp256k1.PrivateKey

// PublicKey wraps the secp256k1 public key.
type PublicKey = secp256k1.PublicKey

// ErrInvalidPrivateKey is returned when an encoded private key is malformed.
var ErrInvalidPrivateKey = errors.New("crypto: invalid private key bytes")

// GenerateKey creates a new random secp256k1 private key. The caller is
// responsible for securely storing and zeroising the returned key.
func GenerateKey() (*PrivateKey, error) {
	priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("crypto: generate key: %w", err)
	}
	return priv, nil
}

// PrivateKeyFromBytes parses a 32-byte big-endian scalar into a private key.
func PrivateKeyFromBytes(b []byte) (*PrivateKey, error) {
	if len(b) != 32 {
		return nil, fmt.Errorf("%w: need 32 bytes, got %d", ErrInvalidPrivateKey, len(b))
	}
	priv := secp256k1.PrivKeyFromBytes(b)
	if priv == nil {
		return nil, ErrInvalidPrivateKey
	}
	return priv, nil
}

// PublicKeyFromPrivate derives the compressed-encoded secp256k1 public key
// from priv.
func PublicKeyFromPrivate(priv *PrivateKey) *PublicKey {
	return priv.PubKey()
}

// PubKeyToAddress derives the HASH160 (RIPEMD-160(SHA-256(pubkey))) address
// from a compressed-encoded public key. This is the standard Bitcoin P2PKH
// address construction.
func PubKeyToAddress(pub *PublicKey) types.Address {
	compressed := pub.SerializeCompressed()
	return Hash160(compressed)
}
