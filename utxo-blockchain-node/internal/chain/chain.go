// Package chain implements the chain state manager for linear-chain block
// import. It wires together consensus validation, UTXO-set management, and
// persistent storage into a single serialised Manager. Side-chain storage and
// reorg logic are added in Milestone 11.
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

// Sentinel errors.
var (
	// ErrOrphanBlock is returned by ImportBlock when the block's parent is
	// not present in the block index.
	ErrOrphanBlock = errors.New("chain: parent block not found in block index")
	// ErrTipNotInitialized is returned by ImportBlock before InitGenesis has
	// been called.
	ErrTipNotInitialized = errors.New("chain: chain tip not initialized; call InitGenesis first")
)

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

// ImportBlock validates block against the current chain state and, when valid,
// atomically connects it. Returns ErrTipNotInitialized when InitGenesis has not
// yet been called, and ErrOrphanBlock when the block's parent is unknown.
// now must be a reasonable Unix timestamp (e.g. time.Now().Unix()).
func (m *Manager) ImportBlock(block *types.Block, now int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.tip == nil {
		return ErrTipNotInitialized
	}

	// Look up parent in the persistent block index.
	parentIdx, err := m.db.GetBlockIndex(block.Header.PrevHash)
	if err != nil {
		return fmt.Errorf("chain: look up parent block index: %w", err)
	}
	if parentIdx == nil {
		return fmt.Errorf("%w: %s", ErrOrphanBlock, block.Header.PrevHash)
	}

	height := parentIdx.Height + 1
	subsidy := BlockSubsidy(height)

	// Full consensus validation against the committed UTXO set.
	if _, err := consensus.ValidateBlock(
		block,
		&parentIdx.Header,
		&dbUTXOView{m.db},
		height,
		subsidy,
		m.powNibbles,
		now,
	); err != nil {
		return fmt.Errorf("chain: block validation: %w", err)
	}

	return m.connectBlock(block, height, parentIdx.TotalWork)
}

// connectBlock atomically writes a validated block and updates all persistent
// and in-memory state. Callers must hold m.mu. parentTotalWork is the
// cumulative chain work of the parent (pass big.NewInt(0) for genesis).
func (m *Manager) connectBlock(block *types.Block, height uint32, parentTotalWork *big.Int) error {
	hash := block.BlockHash()

	// Compute cumulative chain work. When powNibbles == 0 (development/test
	// mode) each block contributes 1 unit so TotalWork still increases.
	var blockWork *big.Int
	if m.powNibbles == 0 {
		blockWork = big.NewInt(1)
	} else {
		blockWork = new(big.Int).Lsh(big.NewInt(1), uint(m.powNibbles*4))
	}
	totalWork := new(big.Int).Add(parentTotalWork, blockWork)

	undo := &types.BlockUndo{BlockHash: hash}
	var newTip *types.ChainTip

	if err := m.db.Update(func(tx *storage.WriteTx) error {
		// ── 1. Spend inputs of all non-coinbase transactions ──────────
		// Read each UTXO before deleting it so we can populate the undo
		// record that DisconnectBlock will need during reorg (Milestone 11).
		for i := 1; i < len(block.Transactions); i++ {
			for _, in := range block.Transactions[i].Inputs {
				utxo, err := tx.GetUTXO(in.PrevOut)
				if err != nil {
					return fmt.Errorf("chain: read UTXO for undo record: %w", err)
				}
				if utxo == nil {
					// Unreachable if consensus.ValidateBlock ran first, but
					// guard explicitly so any future caller ordering bug is
					// caught immediately.
					return fmt.Errorf("chain: UTXO %v:%d missing during connectBlock",
						in.PrevOut.TxID, in.PrevOut.Index)
				}
				undo.Spent = append(undo.Spent, types.SpentOutput{
					OutPoint: utxo.OutPoint,
					Output:   utxo.Output,
					Height:   utxo.Height,
					Coinbase: utxo.Coinbase,
				})
				if err := tx.DeleteUTXO(in.PrevOut); err != nil {
					return err
				}
			}
		}

		// ── 2. Add outputs of every transaction (coinbase included) ───
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
					return err
				}
			}
		}

		// ── 3. Persist block index ────────────────────────────────────
		if err := tx.SaveBlockIndex(&types.BlockIndex{
			Hash:      hash,
			Header:    block.Header,
			Height:    height,
			TotalWork: totalWork,
			Status:    types.BlockStatusValid,
		}); err != nil {
			return err
		}

		// ── 4. Persist full block and header ──────────────────────────
		if err := tx.SaveBlock(block); err != nil {
			return err
		}
		if err := tx.SaveHeader(&block.Header); err != nil {
			return err
		}

		// ── 5. Persist undo record ────────────────────────────────────
		if err := tx.SaveUndo(undo); err != nil {
			return err
		}

		// ── 6. Update active chain height index ───────────────────────
		if err := tx.SetActiveHash(height, hash); err != nil {
			return err
		}

		// ── 7. Update best tip ────────────────────────────────────────
		newTip = &types.ChainTip{Hash: hash, Height: height, TotalWork: totalWork}
		return tx.SetBestTip(newTip)
	}); err != nil {
		return err
	}

	// Update in-memory tip only after bbolt has committed successfully.
	m.tip = newTip
	return nil
}

// dbUTXOView adapts *storage.DB to the consensus.UTXOView interface so that
// consensus.ValidateBlock can read the committed UTXO set without knowing
// anything about bbolt.
type dbUTXOView struct{ db *storage.DB }

func (v *dbUTXOView) GetUTXO(op types.OutPoint) (*types.UTXO, error) {
	return v.db.GetUTXO(op)
}
