package node

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/api"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/chain"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/config"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/mempool"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/p2p"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/storage"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// nodeServices implements api.Services and p2p.Callbacks. It bridges the
// HTTP layer to the chain manager, mempool, storage, and gossiper without
// putting any consensus logic in the handlers.
//
// mu protects the two fields that may be swapped atomically during a hard
// reset: db and chain. All public methods that touch db or chain must
// acquire mu.RLock(); DemoHardReset acquires the exclusive mu.Lock().
// Internal "noLock" variants are called by composite methods that already
// hold the lock, to avoid reentrant deadlocks.
type nodeServices struct {
	mu         sync.RWMutex
	cfg        *config.Config
	db         *storage.DB
	chain      *chain.Manager
	mp         *mempool.Mempool
	gossiper   *p2p.Gossiper // nil until wired in
	minerAddr  types.Address // zero if not configured
	powNibbles int
	wallets    *walletStore // nil unless DemoMode is enabled
}

// ── public api.Services methods ───────────────────────────────────────────────

// Status returns a lightweight snapshot of the current chain state.
func (s *nodeServices) Status() api.StatusResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusNoLock()
}

// statusNoLock is the lock-free body of Status. Call only while holding mu.
func (s *nodeServices) statusNoLock() api.StatusResult {
	res := api.StatusResult{
		NodeID:  s.cfg.NodeID,
		Network: s.cfg.NetworkID,
	}
	if tip := s.chain.Tip(); tip != nil {
		h := tip.Height
		res.Height = &h
		res.TipHash = &tip.Hash
	}
	return res
}

// GetBlock delegates to persistent storage.
func (s *nodeServices) GetBlock(hash types.Hash32) (*types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.GetBlock(hash)
}

// GetUTXOsByAddress scans the UTXO set for outputs belonging to addr.
func (s *nodeServices) GetUTXOsByAddress(addr types.Address) ([]*types.UTXO, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.GetUTXOsByAddress(addr)
}

// SubmitTx validates tx and admits it to the mempool. The storage.DB is
// used as the UTXOView because it satisfies the mempool.UTXOView interface
// (GetUTXO returns the committed chain state). On success the transaction
// is broadcast to peers.
func (s *nodeServices) SubmitTx(tx *types.Transaction) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.submitTxNoLock(tx)
}

// submitTxNoLock is the lock-free body of SubmitTx. Call only while holding mu.
func (s *nodeServices) submitTxNoLock(tx *types.Transaction) error {
	if err := s.mp.Add(tx, s.db); err != nil {
		return err
	}
	if s.gossiper != nil {
		s.gossiper.BroadcastTx(context.Background(), tx)
	}
	return nil
}

// Mine builds a candidate block from the current mempool, finds a PoW
// nonce, connects the block, and cleans up the mempool.
func (s *nodeServices) Mine(ctx context.Context) (*api.MineResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mineNoLock(ctx)
}

// mineNoLock is the lock-free body of Mine. Call only while holding mu.
func (s *nodeServices) mineNoLock(ctx context.Context) (*api.MineResult, error) {
	if s.minerAddr.IsZero() {
		return nil, errors.New("MINER_ADDRESS is not configured; cannot mine")
	}

	tip := s.chain.Tip()
	if tip == nil {
		return nil, errors.New("chain not initialized; call InitGenesis first")
	}

	parent, err := s.db.GetHeader(tip.Hash)
	if err != nil {
		return nil, fmt.Errorf("mine: load parent header: %w", err)
	}

	height := tip.Height + 1
	subsidy := chain.BlockSubsidy(height)

	entries := s.mp.Entries()
	extraTxs := make([]types.Transaction, 0, len(entries))
	var totalFees types.Amount
	for _, e := range entries {
		next, feeErr := totalFees.SafeAdd(e.Fee)
		if feeErr != nil {
			break
		}
		totalFees = next
		extraTxs = append(extraTxs, e.Tx)
	}

	coinbaseValue, err := subsidy.SafeAdd(totalFees)
	if err != nil {
		return nil, errors.New("mine: coinbase value overflow (subsidy + fees)")
	}

	block, err := chain.MineBlock(
		ctx, parent, s.minerAddr, coinbaseValue,
		extraTxs, s.powNibbles, time.Now().Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("mine: PoW search: %w", err)
	}

	reorgResult, err := s.chain.ImportBlock(block, time.Now().Unix())
	if err != nil {
		return nil, fmt.Errorf("mine: import block: %w", err)
	}

	if reorgResult != nil {
		for _, connBlock := range reorgResult.Connected {
			s.mp.RemoveMined(connBlock)
			s.mp.RemoveConflicting(connBlock)
		}
		for _, discBlock := range reorgResult.Disconnected {
			for i := 1; i < len(discBlock.Transactions); i++ {
				tx := discBlock.Transactions[i]
				_ = s.mp.Add(&tx, s.db)
			}
		}
	} else {
		s.mp.RemoveMined(block)
		s.mp.RemoveConflicting(block)
	}

	newTip := s.chain.Tip()
	if s.gossiper != nil {
		s.gossiper.BroadcastBlock(ctx, block)
	}
	return &api.MineResult{Hash: newTip.Hash, Height: newTip.Height}, nil
}

// GetPeers returns the configured peer list.
func (s *nodeServices) GetPeers() []string {
	if s.cfg.Peers == nil {
		return []string{}
	}
	return s.cfg.Peers
}

// GetMempool returns a snapshot of current mempool entries sorted by
// descending fee rate.
func (s *nodeServices) GetMempool() []*types.MempoolEntry {
	return s.mp.Entries()
}

// ── api.Services gossip methods ───────────────────────────────────────────────

// ReceiveTx handles an inbound transaction from a peer (via POST /p2p/tx).
func (s *nodeServices) ReceiveTx(ctx context.Context, tx *types.Transaction) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.gossiper != nil {
		return s.gossiper.ReceiveTx(ctx, tx)
	}
	return s.mp.Add(tx, s.db)
}

// ReceiveBlock handles an inbound block from a peer (via POST /p2p/block).
func (s *nodeServices) ReceiveBlock(ctx context.Context, block *types.Block) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.gossiper != nil {
		return s.gossiper.ReceiveBlock(ctx, block)
	}
	_, err := s.chain.ImportBlock(block, time.Now().Unix())
	return err
}

// ── p2p.Callbacks implementation ─────────────────────────────────────────────

// HandleReceivedTx is called by the Gossiper after deduplication.
func (s *nodeServices) HandleReceivedTx(_ context.Context, tx *types.Transaction) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mp.Add(tx, s.db)
}

// HandleReceivedBlock is called by the Gossiper after deduplication.
func (s *nodeServices) HandleReceivedBlock(_ context.Context, block *types.Block) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	reorgResult, err := s.chain.ImportBlock(block, time.Now().Unix())
	if err != nil {
		return err
	}
	if reorgResult != nil {
		for _, connBlock := range reorgResult.Connected {
			s.mp.RemoveMined(connBlock)
			s.mp.RemoveConflicting(connBlock)
		}
		for _, discBlock := range reorgResult.Disconnected {
			for i := 1; i < len(discBlock.Transactions); i++ {
				tx := discBlock.Transactions[i]
				_ = s.mp.Add(&tx, s.db)
			}
		}
	} else {
		s.mp.RemoveMined(block)
		s.mp.RemoveConflicting(block)
	}
	return nil
}

