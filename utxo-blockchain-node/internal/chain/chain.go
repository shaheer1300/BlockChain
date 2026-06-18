// Package chain implements the chain state manager. It wires together
// consensus validation, UTXO-set management, and persistent storage into a
// single serialised Manager. Both linear-chain extension and full
// fork-choice reorg (Milestone 11) are handled here.
//
// Rule: nothing in this package may import internal/api, internal/p2p, or
// internal/node. It may import internal/consensus and internal/storage.
package chain

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/consensus"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/storage"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// Supply schedule constants — simple Bitcoin-style halving.
const (
	// InitialSubsidy is the coinbase reward at height 0 in satoshi-like units.
	InitialSubsidy  types.Amount = 50_000_000
	halvingInterval uint32       = 210_000
)

// BlockSubsidy returns the coinbase reward at height. Subsidy halves every
// halvingInterval blocks; returns 0 after 64 halvings to avoid shift overflow.
func BlockSubsidy(height uint32) types.Amount {
	halvings := height / halvingInterval
	if halvings >= 64 {
		return 0
	}
	return InitialSubsidy >> halvings
}

// computeBlockWork returns the proof-of-work contribution of a single block
// for our simplified difficulty model. When powNibbles == 0 (test mode) each
// block contributes 1 unit so TotalWork still increases monotonically.
func computeBlockWork(powNibbles int) *big.Int {
	if powNibbles == 0 {
		return big.NewInt(1)
	}
	return new(big.Int).Lsh(big.NewInt(1), uint(powNibbles*4))
}

// Sentinel errors.
var (
	// ErrOrphanBlock is returned by ImportBlock when the block's parent is
	// not present in the block index.
	ErrOrphanBlock = errors.New("chain: parent block not found in block index")
	// ErrTipNotInitialized is returned by ImportBlock before InitGenesis has
	// been called.
	ErrTipNotInitialized = errors.New("chain: chain tip not initialized; call InitGenesis first")
)

// ReorgResult is returned by ImportBlock when a side-chain block overtook the
// active chain. It is nil when the block simply extended the active chain. The
// caller should use it to revalidate the mempool: remove mined/conflicting txs
// for each Connected block and re-attempt adding txs from Disconnected blocks.
type ReorgResult struct {
	// OldTip is the active-chain tip hash before the reorg.
	OldTip types.Hash32
	// NewTip is the active-chain tip hash after the reorg.
	NewTip types.Hash32
	// ForkPoint is the hash of the last common ancestor block.
	ForkPoint types.Hash32
	// Disconnected contains the blocks removed from the active chain,
	// ordered oldest-first (fork+1 … old-tip).
	Disconnected []*types.Block
	// Connected contains the blocks added to the new active chain,
	// ordered oldest-first (fork+1 … new-tip).
	Connected []*types.Block
}

// Manager is the chain state manager. It serialises all chain-state mutations
// through a single mutex and delegates persistence to *storage.DB.
// All exported methods are safe for concurrent use.
type Manager struct {
	db         *storage.DB
	powNibbles int
	mu         sync.Mutex
	tip        *types.ChainTip // nil until InitGenesis succeeds
}

// NewManager creates a Manager backed by db. It loads the persisted best tip
// so that a restarted node resumes from where it left off. If the chain has
// not been initialised (fresh database) tip will be nil until InitGenesis is
// called.
func NewManager(db *storage.DB, powNibbles int) (*Manager, error) {
	tip, err := db.GetBestTip()
	if err != nil {
		return nil, fmt.Errorf("chain: load best tip: %w", err)
	}
	return &Manager{db: db, powNibbles: powNibbles, tip: tip}, nil
}

// Tip returns a snapshot of the current best chain tip, or nil when the chain
// has not been initialised.
func (m *Manager) Tip() *types.ChainTip {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tip == nil {
		return nil
	}
	c := *m.tip
	return &c
}

// InitGenesis mines and connects the genesis block if the chain is not yet
// initialised. It is safe to call on every node startup — it is a no-op when
// genesis already exists. ctx cancellation aborts the PoW search loop.
func (m *Manager) InitGenesis(ctx context.Context, miner types.Address, subsidy types.Amount, now int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.tip != nil {
		return nil // already initialised; idempotent
	}

	block, err := MineBlock(ctx, nil, miner, subsidy, nil, m.powNibbles, now)
	if err != nil {
		return fmt.Errorf("chain: mine genesis: %w", err)
	}
	if err := m.connectBlock(block, 0, big.NewInt(0)); err != nil {
		return fmt.Errorf("chain: connect genesis: %w", err)
	}
	return nil
}

// ImportBlock validates block against its parent's chain state and, when
// valid, either extends the active chain or stores it as a side-chain
// candidate. When a side-chain block accumulates more cumulative work than
// the active chain, a full reorg is executed atomically and the returned
// *ReorgResult describes what changed. A nil *ReorgResult means the block
// simply extended the active chain.
//
// Returns ErrTipNotInitialized before InitGenesis, ErrOrphanBlock when the
// parent is unknown, and a consensus error when the block is invalid.
func (m *Manager) ImportBlock(block *types.Block, now int64) (*ReorgResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.tip == nil {
		return nil, ErrTipNotInitialized
	}

	// Look up parent in the persistent block index.
	parentIdx, err := m.db.GetBlockIndex(block.Header.PrevHash)
	if err != nil {
		return nil, fmt.Errorf("chain: look up parent block index: %w", err)
	}
	if parentIdx == nil {
		return nil, fmt.Errorf("%w: %s", ErrOrphanBlock, block.Header.PrevHash)
	}

	height := parentIdx.Height + 1
	subsidy := BlockSubsidy(height)

	// Full consensus validation against the committed UTXO set.
	// Side-chain blocks that spend only their own coinbase outputs (or
	// UTXOs that are also present in the active chain) will pass. For more
	// general side-chain validation the caller should submit blocks in
	// chronological order from the fork point.
	if _, err := consensus.ValidateBlock(
		block,
		&parentIdx.Header,
		&dbUTXOView{m.db},
		height,
		subsidy,
		m.powNibbles,
		now,
	); err != nil {
		return nil, fmt.Errorf("chain: block validation: %w", err)
	}

	// ── Simple extend: this block builds directly on the current tip ──
	if block.Header.PrevHash == m.tip.Hash {
		if err := m.connectBlock(block, height, parentIdx.TotalWork); err != nil {
			return nil, err
		}
		return nil, nil // no reorg
	}

	// ── Side-chain block: store it without touching the UTXO set ──────
	if err := m.storeSideChainBlock(block, height, parentIdx.TotalWork); err != nil {
		return nil, err
	}

	// Check whether the side chain now has more cumulative work than the
	// active chain. If not, just return — we stored the block for later.
	blockWork := computeBlockWork(m.powNibbles)
	newTotalWork := new(big.Int).Add(parentIdx.TotalWork, blockWork)
	if newTotalWork.Cmp(m.tip.TotalWork) <= 0 {
		return nil, nil
	}

	// Side chain overtook active chain — execute a reorg.
	newHash := block.BlockHash()
	newTipIdx, err := m.db.GetBlockIndex(newHash)
	if err != nil {
		return nil, fmt.Errorf("chain: reload block index after side-chain store: %w", err)
	}
	if newTipIdx == nil {
		return nil, fmt.Errorf("chain: block index missing immediately after store: %s", newHash)
	}
	return m.reorg(newTipIdx)
}

// ── Internal chain-state helpers (callers must hold m.mu) ────────────────────

// connectBlock opens a bbolt write transaction, calls connectBlockTx to
// apply all mutations, then updates the in-memory tip on success.
func (m *Manager) connectBlock(block *types.Block, height uint32, parentTotalWork *big.Int) error {
	var newTip *types.ChainTip
	if err := m.db.Update(func(tx *storage.WriteTx) error {
		var err error
		newTip, err = m.connectBlockTx(tx, block, height, parentTotalWork)
		return err
	}); err != nil {
		return err
	}
	m.tip = newTip
	return nil
}

// connectBlockTx atomically writes a validated block and all its side-effects
// within an already-open write transaction. It is used both during normal
// block import and during reorg block connection. Returns the new ChainTip.
func (m *Manager) connectBlockTx(
	tx *storage.WriteTx,
	block *types.Block,
	height uint32,
	parentTotalWork *big.Int,
) (*types.ChainTip, error) {
	hash := block.BlockHash()
	totalWork := new(big.Int).Add(parentTotalWork, computeBlockWork(m.powNibbles))

	undo := &types.BlockUndo{BlockHash: hash}

	// ── 1. Spend inputs of all non-coinbase transactions ──────────────
	for i := 1; i < len(block.Transactions); i++ {
		for _, in := range block.Transactions[i].Inputs {
			utxo, err := tx.GetUTXO(in.PrevOut)
			if err != nil {
				return nil, fmt.Errorf("chain: read UTXO for undo record: %w", err)
			}
			if utxo == nil {
				return nil, fmt.Errorf("chain: UTXO %v:%d missing during connectBlockTx",
					in.PrevOut.TxID, in.PrevOut.Index)
			}
			undo.Spent = append(undo.Spent, types.SpentOutput{
				OutPoint: utxo.OutPoint,
				Output:   utxo.Output,
				Height:   utxo.Height,
				Coinbase: utxo.Coinbase,
			})
			if err := tx.DeleteUTXO(in.PrevOut); err != nil {
				return nil, err
			}
		}
	}

	// ── 2. Add outputs of every transaction (coinbase included) ───────
	for i := range block.Transactions {
		t := &block.Transactions[i]
		txID := t.TxID()
		isCoinbase := t.IsCoinbase()
		for idx, out := range t.Outputs {
			op := types.OutPoint{TxID: txID, Index: uint32(idx)}
			if err := tx.PutUTXO(&types.UTXO{
				OutPoint: op,
				Output:   out,
				Height:   height,
				Coinbase: isCoinbase,
			}); err != nil {
				return nil, err
			}
		}
	}

	// ── 3. Persist block index ────────────────────────────────────────
	if err := tx.SaveBlockIndex(&types.BlockIndex{
		Hash:      hash,
		Header:    block.Header,
		Height:    height,
		TotalWork: totalWork,
		Status:    types.BlockStatusValid,
	}); err != nil {
		return nil, err
	}

	// ── 4. Persist full block and header ──────────────────────────────
	if err := tx.SaveBlock(block); err != nil {
		return nil, err
	}
	if err := tx.SaveHeader(&block.Header); err != nil {
		return nil, err
	}

	// ── 5. Persist undo record ────────────────────────────────────────
	if err := tx.SaveUndo(undo); err != nil {
		return nil, err
	}

	// ── 6. Update active chain height index ───────────────────────────
	if err := tx.SetActiveHash(height, hash); err != nil {
		return nil, err
	}

	// ── 7. Update best tip ────────────────────────────────────────────
	newTip := &types.ChainTip{Hash: hash, Height: height, TotalWork: totalWork}
	if err := tx.SetBestTip(newTip); err != nil {
		return nil, err
	}
	return newTip, nil
}

// storeSideChainBlock saves a validated side-chain block, its header, and its
// block-index entry. It does NOT touch the UTXO set or the active-chain index.
func (m *Manager) storeSideChainBlock(block *types.Block, height uint32, parentTotalWork *big.Int) error {
	hash := block.BlockHash()
	totalWork := new(big.Int).Add(parentTotalWork, computeBlockWork(m.powNibbles))

	return m.db.Update(func(tx *storage.WriteTx) error {
		if err := tx.SaveBlockIndex(&types.BlockIndex{
			Hash:      hash,
			Header:    block.Header,
			Height:    height,
			TotalWork: totalWork,
			Status:    types.BlockStatusValid,
		}); err != nil {
			return err
		}
		if err := tx.SaveBlock(block); err != nil {
			return err
		}
		return tx.SaveHeader(&block.Header)
	})
}

// disconnectBlockTx reverses the UTXO-set effects of a previously-connected
// block using its undo record:
//  1. Delete every output the block created from the UTXO set.
//  2. Restore every UTXO the block spent (from undo.Spent, in reverse order).
//  3. Remove the active-chain entry at this block's height.
//
// The block and its block-index entry are intentionally left in storage so
// the data remains available for future reorg traversal.
// Returns the full block (needed by the caller to report the reorg result).
func (m *Manager) disconnectBlockTx(tx *storage.WriteTx, blockIdx *types.BlockIndex) (*types.Block, error) {
	block, err := tx.GetBlock(blockIdx.Hash)
	if err != nil {
		return nil, fmt.Errorf("chain: disconnectBlockTx: load block %s: %w", blockIdx.Hash, err)
	}
	if block == nil {
		return nil, fmt.Errorf("chain: disconnectBlockTx: block %s not found in storage", blockIdx.Hash)
	}

	undo, err := tx.GetUndo(blockIdx.Hash)
	if err != nil {
		return nil, fmt.Errorf("chain: disconnectBlockTx: load undo for %s: %w", blockIdx.Hash, err)
	}
	if undo == nil {
		return nil, fmt.Errorf("chain: disconnectBlockTx: undo record for %s not found", blockIdx.Hash)
	}

	// ── 1. Delete outputs this block created ──────────────────────────
	for i := range block.Transactions {
		t := &block.Transactions[i]
		txID := t.TxID()
		for idx := range t.Outputs {
			if err := tx.DeleteUTXO(types.OutPoint{TxID: txID, Index: uint32(idx)}); err != nil {
				return nil, fmt.Errorf("chain: disconnectBlockTx: delete UTXO: %w", err)
			}
		}
	}

	// ── 2. Restore spent UTXOs (reverse undo order) ───────────────────
	for i := len(undo.Spent) - 1; i >= 0; i-- {
		so := undo.Spent[i]
		if err := tx.PutUTXO(&types.UTXO{
			OutPoint: so.OutPoint,
			Output:   so.Output,
			Height:   so.Height,
			Coinbase: so.Coinbase,
		}); err != nil {
			return nil, fmt.Errorf("chain: disconnectBlockTx: restore UTXO: %w", err)
		}
	}

	// ── 3. Remove active chain entry at this height ───────────────────
	if err := tx.DeleteActiveHash(blockIdx.Height); err != nil {
		return nil, fmt.Errorf("chain: disconnectBlockTx: delete active hash: %w", err)
	}

	return block, nil
}

// findForkPoint locates the most recent common ancestor of block indices a
// and b by walking up both chains via PrevHash until they converge. Both
// chains must ultimately descend from a shared genesis block.
func (m *Manager) findForkPoint(a, b *types.BlockIndex) (*types.BlockIndex, error) {
	for a.Hash != b.Hash {
		switch {
		case a.Height > b.Height:
			parent, err := m.db.GetBlockIndex(a.Header.PrevHash)
			if err != nil || parent == nil {
				return nil, fmt.Errorf("chain: findForkPoint: parent of %s not found", a.Hash)
			}
			a = parent
		case b.Height > a.Height:
			parent, err := m.db.GetBlockIndex(b.Header.PrevHash)
			if err != nil || parent == nil {
				return nil, fmt.Errorf("chain: findForkPoint: parent of %s not found", b.Hash)
			}
			b = parent
		default:
			// Same height, different hashes — walk both back one step.
			parentA, err := m.db.GetBlockIndex(a.Header.PrevHash)
			if err != nil || parentA == nil {
				return nil, fmt.Errorf("chain: findForkPoint: parent of %s not found", a.Hash)
			}
			parentB, err := m.db.GetBlockIndex(b.Header.PrevHash)
			if err != nil || parentB == nil {
				return nil, fmt.Errorf("chain: findForkPoint: parent of %s not found", b.Hash)
			}
			a = parentA
			b = parentB
		}
	}
	return a, nil
}

// reorg executes a chain reorganisation that makes newTipIdx the new active
// tip. All UTXO-set changes, active-chain-index updates, and best-tip writes
// happen inside a single bbolt write transaction for atomicity. On success
// m.tip is updated and the *ReorgResult is returned. Callers must hold m.mu.
func (m *Manager) reorg(newTipIdx *types.BlockIndex) (*ReorgResult, error) {
	oldTipIdx, err := m.db.GetBlockIndex(m.tip.Hash)
	if err != nil || oldTipIdx == nil {
		return nil, fmt.Errorf("chain: reorg: load old tip block index: %w", err)
	}

	forkIdx, err := m.findForkPoint(oldTipIdx, newTipIdx)
	if err != nil {
		return nil, err
	}

	// ── Collect blocks to disconnect (active tip → fork, newest-first) ─
	var toDisconnect []*types.BlockIndex
	for cur := oldTipIdx; cur.Hash != forkIdx.Hash; {
		toDisconnect = append(toDisconnect, cur)
		parent, err := m.db.GetBlockIndex(cur.Header.PrevHash)
		if err != nil || parent == nil {
			return nil, fmt.Errorf("chain: reorg: parent of %s not found", cur.Hash)
		}
		cur = parent
	}

	// ── Collect blocks to connect (fork → new tip, newest-first then reversed) ──
	var toConnect []*types.BlockIndex
	for cur := newTipIdx; cur.Hash != forkIdx.Hash; {
		toConnect = append(toConnect, cur)
		parent, err := m.db.GetBlockIndex(cur.Header.PrevHash)
		if err != nil || parent == nil {
			return nil, fmt.Errorf("chain: reorg: parent of %s (new branch) not found", cur.Hash)
		}
		cur = parent
	}
	// Reverse toConnect to oldest-first order for sequential connection.
	for i, j := 0, len(toConnect)-1; i < j; i, j = i+1, j-1 {
		toConnect[i], toConnect[j] = toConnect[j], toConnect[i]
	}

	result := &ReorgResult{
		OldTip:    m.tip.Hash,
		NewTip:    newTipIdx.Hash,
		ForkPoint: forkIdx.Hash,
	}

	var newTip *types.ChainTip

	if err := m.db.Update(func(tx *storage.WriteTx) error {
		// ── Disconnect old chain (newest-first = tip down to fork) ────
		for _, idx := range toDisconnect {
			blk, err := m.disconnectBlockTx(tx, idx)
			if err != nil {
				return fmt.Errorf("disconnect %s: %w", idx.Hash, err)
			}
			result.Disconnected = append(result.Disconnected, blk)
		}
		// result.Disconnected is currently newest-first; reverse to oldest-first.
		for i, j := 0, len(result.Disconnected)-1; i < j; i, j = i+1, j-1 {
			result.Disconnected[i], result.Disconnected[j] = result.Disconnected[j], result.Disconnected[i]
		}

		// ── Connect new chain (oldest-first = fork+1 up to new tip) ───
		for _, idx := range toConnect {
			blk, err := tx.GetBlock(idx.Hash)
			if err != nil {
				return fmt.Errorf("chain: reorg: load block %s: %w", idx.Hash, err)
			}
			if blk == nil {
				return fmt.Errorf("chain: reorg: block %s not found in storage", idx.Hash)
			}
			// parentWork = TotalWork of this block minus one block's contribution.
			parentWork := new(big.Int).Sub(idx.TotalWork, computeBlockWork(m.powNibbles))
			tip, err := m.connectBlockTx(tx, blk, idx.Height, parentWork)
			if err != nil {
				return fmt.Errorf("connect %s: %w", idx.Hash, err)
			}
			newTip = tip
			result.Connected = append(result.Connected, blk)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("chain: reorg: %w", err)
	}

	m.tip = newTip
	return result, nil
}

// dbUTXOView adapts *storage.DB to the consensus.UTXOView interface so that
// consensus.ValidateBlock can read the committed UTXO set without knowing
// anything about bbolt.
type dbUTXOView struct{ db *storage.DB }

func (v *dbUTXOView) GetUTXO(op types.OutPoint) (*types.UTXO, error) {
	return v.db.GetUTXO(op)
}
