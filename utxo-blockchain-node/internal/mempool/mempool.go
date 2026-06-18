// Package mempool implements the node's unconfirmed-transaction pool.
// The mempool is pure policy logic: it enforces local acceptance rules
// (fee rate, duplicates, double-spends) that are stricter than consensus.
// A transaction may pass consensus.ValidateTx and still be rejected by the
// mempool (e.g. fee too low). Conversely, nothing in this package relaxes
// consensus rules — every admitted transaction must first pass
// consensus.ValidateTx.
//
// Rule: this package must not import internal/storage, internal/api, or
// internal/p2p. It accepts a UTXOView interface so the caller decides
// which backing store to use.
package mempool

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/consensus"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// Sentinel errors returned by Mempool methods.
var (
	// ErrDuplicateTx is returned when a transaction with the same TxID is
	// already in the mempool.
	ErrDuplicateTx = errors.New("mempool: transaction already in mempool")

	// ErrDoubleSpend is returned when any input of the submitted transaction
	// spends an OutPoint already spent by another in-mempool transaction.
	ErrDoubleSpend = errors.New("mempool: transaction double-spends a mempool output")

	// ErrFeeTooLow is returned when the computed fee rate is below the
	// mempool's minimum.
	ErrFeeTooLow = errors.New("mempool: transaction fee rate below minimum")
)

// UTXOView is the read-only interface the mempool uses to look up unspent
// outputs from the committed chain state. It has the same contract as
// consensus.UTXOView: GetUTXO returns (nil, nil) when not found.
type UTXOView interface {
	GetUTXO(op types.OutPoint) (*types.UTXO, error)
}

// Mempool holds unconfirmed transactions. All exported methods are safe for
// concurrent use.
type Mempool struct {
	mu sync.RWMutex

	// entries is the primary transaction map (TxID → entry).
	entries map[types.Hash32]*types.MempoolEntry

	// spends maps each OutPoint being spent by a mempool transaction to
	// the TxID of the spending transaction. This lets Add detect double-
	// spends in O(inputs) time and lets RemoveForBlock purge conflicting
	// transactions in O(1) per input.
	spends map[types.OutPoint]types.Hash32

	// minFeeRateSatPerByte is the minimum fee rate for acceptance. 0 means
	// no minimum (useful for tests and local development).
	minFeeRateSatPerByte uint64
}

// New creates an empty Mempool. minFeeRate is the minimum fee rate in
// satoshis-per-byte; pass 0 to disable the minimum-fee policy.
func New(minFeeRate uint64) *Mempool {
	return &Mempool{
		entries:              make(map[types.Hash32]*types.MempoolEntry),
		spends:               make(map[types.OutPoint]types.Hash32),
		minFeeRateSatPerByte: minFeeRate,
	}
}

// Add validates tx against view and the current mempool state, then adds it
// to the pool. Steps performed in order:
//
//  1. Consensus validation via consensus.ValidateTx.
//  2. Duplicate check (same TxID already in pool).
//  3. Double-spend check (any input already claimed by a pooled tx).
//  4. Minimum fee-rate check.
//
// On success the entry is stored and the spend index is updated. On any
// failure the pool is unchanged.
func (m *Mempool) Add(tx *types.Transaction, view UTXOView) error {
	// ── 1. Consensus validation ──────────────────────────────────────
	result, err := consensus.ValidateTx(tx, view)
	if err != nil {
		return fmt.Errorf("mempool: consensus: %w", err)
	}

	txID := tx.TxID()

	// ── 2. Compute canonical size and fee rate ───────────────────────
	encoded, encErr := tx.CanonicalEncode()
	if encErr != nil {
		return fmt.Errorf("mempool: encode: %w", encErr)
	}
	size := uint32(len(encoded))

	var feeRate uint64
	if size > 0 {
		feeRate = uint64(result.Fee) / uint64(size)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// ── 3. Duplicate check ───────────────────────────────────────────
	if _, exists := m.entries[txID]; exists {
		return ErrDuplicateTx
	}

	// ── 4. Double-spend check ────────────────────────────────────────
	for _, in := range tx.Inputs {
		if spender, conflict := m.spends[in.PrevOut]; conflict {
			return fmt.Errorf("%w: outpoint %s:%d already spent by %s",
				ErrDoubleSpend, in.PrevOut.TxID, in.PrevOut.Index, spender)
		}
	}

	// ── 5. Minimum fee-rate policy ───────────────────────────────────
	if m.minFeeRateSatPerByte > 0 && feeRate < m.minFeeRateSatPerByte {
		return fmt.Errorf("%w: got %d sat/byte, minimum %d sat/byte",
			ErrFeeTooLow, feeRate, m.minFeeRateSatPerByte)
	}

	// ── 6. Admit ─────────────────────────────────────────────────────
	entry := &types.MempoolEntry{
		Tx:      *tx,
		TxID:    txID,
		Fee:     result.Fee,
		Size:    size,
		FeeRate: feeRate,
		AddedAt: time.Now().Unix(),
	}
	m.entries[txID] = entry
	for _, in := range tx.Inputs {
		m.spends[in.PrevOut] = txID
	}
	return nil
}

// Get returns the mempool entry for txID, or nil when not present.
func (m *Mempool) Get(txID types.Hash32) *types.MempoolEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.entries[txID]
}

// Size returns the number of transactions currently in the mempool.
func (m *Mempool) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// Entries returns a snapshot of all entries, sorted by descending fee rate so
// the miner can greedily select the most profitable transactions first.
func (m *Mempool) Entries() []*types.MempoolEntry {
	m.mu.RLock()
	entries := make([]*types.MempoolEntry, 0, len(m.entries))
	for _, e := range m.entries {
		entries = append(entries, e)
	}
	m.mu.RUnlock()

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].FeeRate != entries[j].FeeRate {
			return entries[i].FeeRate > entries[j].FeeRate
		}
		// Tie-break: earlier transactions first (lower AddedAt).
		return entries[i].AddedAt < entries[j].AddedAt
	})
	return entries
}

// RemoveMined removes every transaction whose TxID appears in block from the
// mempool and its spend index. Call this after a block has been connected to
// the chain so already-mined transactions are evicted cleanly.
func (m *Mempool) RemoveMined(block *types.Block) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := 1; i < len(block.Transactions); i++ {
		txID := block.Transactions[i].TxID()
		m.remove(txID)
	}
}

// RemoveConflicting removes transactions that conflict with the inputs of any
// non-coinbase transaction in block — i.e. transactions that attempted to
// spend the same outpoints as transactions that were just confirmed. This
// keeps the mempool consistent after each block connection.
//
// It is safe to call RemoveConflicting on the same block as RemoveMined; the
// two operations are idempotent and may be called in either order.
func (m *Mempool) RemoveConflicting(block *types.Block) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := 1; i < len(block.Transactions); i++ {
		for _, in := range block.Transactions[i].Inputs {
			if spender, ok := m.spends[in.PrevOut]; ok {
				m.remove(spender)
			}
		}
	}
}

// remove is the unsynchronised single-entry deletion. The caller must hold
// m.mu.
func (m *Mempool) remove(txID types.Hash32) {
	entry, ok := m.entries[txID]
	if !ok {
		return
	}
	for _, in := range entry.Tx.Inputs {
		delete(m.spends, in.PrevOut)
	}
	delete(m.entries, txID)
}

// HasSpend reports whether any transaction in the mempool spends op.
func (m *Mempool) HasSpend(op types.OutPoint) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.spends[op]
	return ok
}
