package types

import "bytes"

// BlockHeaderSize is the locked canonical size of a serialized
// BlockHeader in bytes (4 + 32 + 32 + 8 + 4 + 8). Changing this constant
// or the encoding layout is a hard fork and must be guarded by tests.
const BlockHeaderSize = 88

// BlockHeader is the consensus header. Its canonical encoding is the
// preimage of BlockHash and is what proof-of-work hashes over.
type BlockHeader struct {
	Version    uint32 `json:"version"`
	PrevHash   Hash32 `json:"prev_hash"`
	MerkleRoot Hash32 `json:"merkle_root"`
	Timestamp  int64  `json:"timestamp"`
	Bits       uint32 `json:"bits"`
	Nonce      uint64 `json:"nonce"`
}

// CanonicalEncode returns the deterministic 88-byte representation used
// as the preimage of BlockHash. No error is returned because the
// fixed-shape header cannot fail to encode.
func (h *BlockHeader) CanonicalEncode() []byte {
	var buf bytes.Buffer
	buf.Grow(BlockHeaderSize)
	e := newCanonicalEncoder(&buf)
	e.writeUint32(h.Version)
	e.writeHash(h.PrevHash)
	e.writeHash(h.MerkleRoot)
	e.writeInt64(h.Timestamp)
	e.writeUint32(h.Bits)
	e.writeUint64(h.Nonce)
	return buf.Bytes()
}

// BlockHash returns the double-SHA-256 of the canonical header encoding.
func (h *BlockHeader) BlockHash() Hash32 {
	return doubleSHA256(h.CanonicalEncode())
}

// Block bundles a header with its ordered transaction list. The invariant
// that Transactions[0] must be the coinbase is enforced by
// internal/consensus; this layer remains a pure data container.
type Block struct {
	Header       BlockHeader   `json:"header"`
	Transactions []Transaction `json:"transactions"`
}

// BlockHash delegates to the header's hash.
func (b *Block) BlockHash() Hash32 { return b.Header.BlockHash() }
