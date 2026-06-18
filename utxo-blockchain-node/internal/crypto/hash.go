// Package crypto provides blockchain-specific cryptographic primitives:
// hashing, Merkle root construction, secp256k1 key management, address
// derivation, transaction signing, and signature verification.
//
// All functions are stateless and deterministic. No global state, no
// randomness except where explicitly required (key generation).
package crypto

import (
	"crypto/sha256"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// SHA256 returns the single SHA-256 digest of data.
func SHA256(data []byte) types.Hash32 {
	return types.Hash32(sha256.Sum256(data))
}

// DoubleSHA256 returns SHA-256(SHA-256(data)). This is the standard
// Bitcoin-family hash used for transaction IDs and block hashes.
func DoubleSHA256(data []byte) types.Hash32 {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return types.Hash32(second)
}

// Hash160 returns RIPEMD-160(SHA-256(data)), the standard 20-byte
// address hash used in Bitcoin-style pay-to-pubkey-hash outputs.
func Hash160(data []byte) types.Address {
	h := sha256.Sum256(data)
	return types.Address(ripemd160Hash(h[:]))
}

// MerkleRoot computes the Merkle root of a list of leaf hashes using the
// Bitcoin double-SHA-256 Merkle tree algorithm:
//   - Empty list: returns ZeroHash.
//   - Single leaf: returns that leaf unchanged.
//   - Odd number of nodes at any level: the last node is duplicated.
//
// The returned hash is the canonical Merkle root to place in BlockHeader.
func MerkleRoot(leaves []types.Hash32) types.Hash32 {
	if len(leaves) == 0 {
		return types.ZeroHash
	}

	// Work on a mutable copy so callers are not surprised by mutation.
	level := make([]types.Hash32, len(leaves))
	copy(level, leaves)

	for len(level) > 1 {
		// Duplicate last element when the level has odd count.
		if len(level)%2 == 1 {
			level = append(level, level[len(level)-1])
		}
		next := make([]types.Hash32, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			combined := append(level[i][:], level[i+1][:]...)
			next[i/2] = DoubleSHA256(combined)
		}
		level = next
	}
	return level[0]
}
