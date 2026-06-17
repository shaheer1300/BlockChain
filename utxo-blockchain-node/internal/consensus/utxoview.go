// Package consensus implements pure validation rules for transactions and
// blocks. It has no database writes, no HTTP calls, and no global state.
// All functions accept explicit dependencies and return errors.
//
// Rule: nothing in this package may import internal/storage, internal/api,
// internal/p2p, or internal/node.
package consensus

import (
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// UTXOView is the read-only interface that transaction validation uses to
// look up unspent outputs. The concrete implementation may be:
//   - MapUTXOView   — an in-memory map used by consensus and tests.
//   - OverlayUTXOView — a two-layer view used during block validation to
//     reflect intra-block spends without writing to the database.
//
// GetUTXO must return (nil, nil) when the outpoint is not found. Any
// database or I/O error is returned as a non-nil error.
type UTXOView interface {
	GetUTXO(op types.OutPoint) (*types.UTXO, error)
}

// MapUTXOView is a purely in-memory UTXOView backed by a Go map. It is
// safe to read from multiple goroutines as long as no goroutine is
// writing concurrently.
type MapUTXOView struct {
	utxos map[types.OutPoint]*types.UTXO
}

// NewMapUTXOView returns an empty MapUTXOView.
func NewMapUTXOView() *MapUTXOView {
	return &MapUTXOView{utxos: make(map[types.OutPoint]*types.UTXO)}
}

// AddUTXO inserts or replaces a UTXO entry.
func (v *MapUTXOView) AddUTXO(utxo *types.UTXO) {
	v.utxos[utxo.OutPoint] = utxo
}

// SpendUTXO removes an entry. It is a no-op if the outpoint is absent.
func (v *MapUTXOView) SpendUTXO(op types.OutPoint) {
	delete(v.utxos, op)
}

// GetUTXO returns the UTXO for op, or (nil, nil) if not found.
func (v *MapUTXOView) GetUTXO(op types.OutPoint) (*types.UTXO, error) {
	return v.utxos[op], nil
}

// OverlayUTXOView is a two-layer view: writes go into a local diff map
// (spent entries stored as nil); reads check the diff first, then fall
// through to the parent. This is used during block validation to reflect
// the UTXOs created and spent within the block being validated without
// touching the persistent database.
type OverlayUTXOView struct {
	parent UTXOView
	diff   map[types.OutPoint]*types.UTXO // nil value = spent
}

// NewOverlayUTXOView wraps parent with a clean diff layer.
func NewOverlayUTXOView(parent UTXOView) *OverlayUTXOView {
	return &OverlayUTXOView{
		parent: parent,
		diff:   make(map[types.OutPoint]*types.UTXO),
	}
}

// AddUTXO records a new UTXO in the diff layer.
func (o *OverlayUTXOView) AddUTXO(utxo *types.UTXO) {
	o.diff[utxo.OutPoint] = utxo
}

// SpendUTXO marks an outpoint as spent in the diff layer.
func (o *OverlayUTXOView) SpendUTXO(op types.OutPoint) {
	o.diff[op] = nil
}

// GetUTXO checks the diff layer first. A nil entry in the diff means the
// outpoint was spent and is intentionally absent. If no diff entry exists,
// the query falls through to the parent view.
func (o *OverlayUTXOView) GetUTXO(op types.OutPoint) (*types.UTXO, error) {
	if entry, ok := o.diff[op]; ok {
		return entry, nil // nil entry means spent
	}
	return o.parent.GetUTXO(op)
}
