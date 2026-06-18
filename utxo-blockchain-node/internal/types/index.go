package types

import (
	"encoding/json"
	"fmt"
	"math/big"
)

// BlockStatus enumerates the validation state of a block known to the
// node's block index.
type BlockStatus uint8

const (
	// BlockStatusUnknown is the zero value; an uninitialised entry.
	BlockStatusUnknown BlockStatus = 0
	// BlockStatusHeaderOnly means the header is known but the full block
	// has not yet been validated end-to-end.
	BlockStatusHeaderOnly BlockStatus = 1
	// BlockStatusValid means the block has been fully validated and
	// connected to the chain at some point.
	BlockStatusValid BlockStatus = 2
	// BlockStatusInvalid means the block (or one of its ancestors) failed
	// consensus validation and must not be reconsidered.
	BlockStatusInvalid BlockStatus = 3
)

// BlockIndex is the in-memory descriptor for a block known to the node.
// TotalWork accumulates proof-of-work from genesis to this block and is
// the value compared by the fork-choice rule.
type BlockIndex struct {
	Hash      Hash32      `json:"hash"`
	Header    BlockHeader `json:"header"`
	Height    uint32      `json:"height"`
	TotalWork *big.Int    `json:"total_work"`
	Status    BlockStatus `json:"status"`
}

type blockIndexJSON struct {
	Hash      Hash32      `json:"hash"`
	Header    BlockHeader `json:"header"`
	Height    uint32      `json:"height"`
	TotalWork string      `json:"total_work"`
	Status    BlockStatus `json:"status"`
}

// MarshalJSON renders TotalWork as a decimal string because *big.Int's
// default JSON form is not stable across all JSON encoders.
func (b BlockIndex) MarshalJSON() ([]byte, error) {
	return json.Marshal(blockIndexJSON{
		Hash:      b.Hash,
		Header:    b.Header,
		Height:    b.Height,
		TotalWork: bigIntString(b.TotalWork),
		Status:    b.Status,
	})
}

// UnmarshalJSON parses a decimal string into TotalWork.
func (b *BlockIndex) UnmarshalJSON(data []byte) error {
	var aux blockIndexJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	work, err := parseBigIntString(aux.TotalWork)
	if err != nil {
		return fmt.Errorf("types: BlockIndex.TotalWork: %w", err)
	}
	b.Hash = aux.Hash
	b.Header = aux.Header
	b.Height = aux.Height
	b.TotalWork = work
	b.Status = aux.Status
	return nil
}

// ChainTip identifies the current best block of the active chain.
type ChainTip struct {
	Hash      Hash32   `json:"hash"`
	Height    uint32   `json:"height"`
	TotalWork *big.Int `json:"total_work"`
}

type chainTipJSON struct {
	Hash      Hash32 `json:"hash"`
	Height    uint32 `json:"height"`
	TotalWork string `json:"total_work"`
}

// MarshalJSON renders TotalWork as a decimal string.
func (c ChainTip) MarshalJSON() ([]byte, error) {
	return json.Marshal(chainTipJSON{
		Hash:      c.Hash,
		Height:    c.Height,
		TotalWork: bigIntString(c.TotalWork),
	})
}

// UnmarshalJSON parses a decimal string into TotalWork.
func (c *ChainTip) UnmarshalJSON(data []byte) error {
	var aux chainTipJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	work, err := parseBigIntString(aux.TotalWork)
	if err != nil {
		return fmt.Errorf("types: ChainTip.TotalWork: %w", err)
	}
	c.Hash = aux.Hash
	c.Height = aux.Height
	c.TotalWork = work
	return nil
}

func bigIntString(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}

func parseBigIntString(s string) (*big.Int, error) {
	if s == "" {
		return new(big.Int), nil
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid decimal big.Int %q", s)
	}
	return v, nil
}
