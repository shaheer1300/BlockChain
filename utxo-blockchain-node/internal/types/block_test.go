package types

import "testing"

func sampleHeader() BlockHeader {
	var prev, merkle Hash32
	for i := range prev {
		prev[i] = byte(i + 1)
		merkle[i] = byte(i + 100)
	}
	return BlockHeader{
		Version:    1,
		PrevHash:   prev,
		MerkleRoot: merkle,
		Timestamp:  1_700_000_000,
		Bits:       0x1d00ffff,
		Nonce:      42,
	}
}

func TestBlockHeader_HashDeterministic(t *testing.T) {
	a := sampleHeader()
	b := sampleHeader()
	if a.BlockHash() != b.BlockHash() {
		t.Fatal("identical headers produced different block hashes")
	}
}

func TestBlockHeader_HashChangesWithFields(t *testing.T) {
	base := sampleHeader()
	baseHash := base.BlockHash()

	mutations := map[string]func(*BlockHeader){
		"version":     func(h *BlockHeader) { h.Version = 2 },
		"prev hash":   func(h *BlockHeader) { h.PrevHash[0] ^= 0xFF },
		"merkle root": func(h *BlockHeader) { h.MerkleRoot[0] ^= 0xFF },
		"timestamp":   func(h *BlockHeader) { h.Timestamp++ },
		"bits":        func(h *BlockHeader) { h.Bits++ },
		"nonce":       func(h *BlockHeader) { h.Nonce++ },
	}

	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			h := sampleHeader()
			mutate(&h)
			if h.BlockHash() == baseHash {
				t.Fatalf("mutation %q did not change block hash", name)
			}
		})
	}
}

func TestBlockHeader_CanonicalSize(t *testing.T) {
	h := sampleHeader()
	encoded := h.CanonicalEncode()
	if len(encoded) != BlockHeaderSize {
		t.Fatalf("canonical header size = %d, want %d", len(encoded), BlockHeaderSize)
	}
}

func TestBlock_BlockHashDelegates(t *testing.T) {
	b := Block{Header: sampleHeader()}
	if b.BlockHash() != b.Header.BlockHash() {
		t.Fatal("Block.BlockHash() did not match Header.BlockHash()")
	}
}
