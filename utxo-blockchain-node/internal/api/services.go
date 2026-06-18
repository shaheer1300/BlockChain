package api

import (
	"context"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// StatusResult is the payload returned by Services.Status and the
// GET /status endpoint.
type StatusResult struct {
	NodeID  string        `json:"node_id"`
	Network string        `json:"network"`
	Height  *uint32       `json:"height"`
	TipHash *types.Hash32 `json:"tip_hash"`
}

// MineResult is the payload returned by Services.Mine and POST /mine.
type MineResult struct {
	Hash   types.Hash32 `json:"hash"`
	Height uint32       `json:"height"`
}

// Services is the set of node operations exposed to HTTP handlers.
// Implementations live in internal/node. Defining the interface here
// keeps the api package free of direct dependencies on chain, mempool,
// and storage.
type Services interface {
	// Status returns a lightweight summary of the current chain state.
	Status() StatusResult

	// GetBlock returns the full block for hash, or (nil, nil) if not found.
	GetBlock(hash types.Hash32) (*types.Block, error)

	// GetUTXOsByAddress returns all unspent outputs whose Recipient matches
	// addr. The result set may be empty but the error is never nil on a
	// normal "address has no UTXOs" outcome.
	GetUTXOsByAddress(addr types.Address) ([]*types.UTXO, error)

	// SubmitTx validates tx against the committed UTXO set and the current
	// mempool state, then admits it to the mempool on success.
	SubmitTx(tx *types.Transaction) error

	// Mine builds a candidate block from the current mempool, finds a valid
	// proof-of-work nonce, and imports the block into the chain.
	Mine(ctx context.Context) (*MineResult, error)

	// GetPeers returns the configured peer URLs.
	GetPeers() []string

	// GetMempool returns a snapshot of all mempool entries sorted by
	// descending fee rate.
	GetMempool() []*types.MempoolEntry
}
