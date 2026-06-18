package node

import (
	"context"
	"errors"
	"fmt"
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
type nodeServices struct {
	cfg        *config.Config
	db         *storage.DB
	chain      *chain.Manager
	mp         *mempool.Mempool
	gossiper   *p2p.Gossiper // nil until wired in
	minerAddr  types.Address // zero if not configured
	powNibbles int
	wallets    *walletStore // nil unless DemoMode is enabled
}

// Status returns a lightweight snapshot of the current chain state.
func (s *nodeServices) Status() api.StatusResult {
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
	return s.db.GetBlock(hash)
}

// GetUTXOsByAddress scans the UTXO set for outputs belonging to addr.
func (s *nodeServices) GetUTXOsByAddress(addr types.Address) ([]*types.UTXO, error) {
	return s.db.GetUTXOsByAddress(addr)
}

// SubmitTx validates tx and admits it to the mempool. The storage.DB is
// used as the UTXOView because it satisfies the mempool.UTXOView interface
// (GetUTXO returns the committed chain state). On success the transaction
// is broadcast to peers.
func (s *nodeServices) SubmitTx(tx *types.Transaction) error {
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

	// Collect mempool transactions in fee-rate order and accumulate fees.
	entries := s.mp.Entries()
	extraTxs := make([]types.Transaction, 0, len(entries))
	var totalFees types.Amount
	for _, e := range entries {
		next, feeErr := totalFees.SafeAdd(e.Fee)
		if feeErr != nil {
			// Fee total overflowed — stop including transactions.
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
		// A reorg occurred (unexpected during mining, but handle it correctly).
		for _, connBlock := range reorgResult.Connected {
			s.mp.RemoveMined(connBlock)
			s.mp.RemoveConflicting(connBlock)
		}
		for _, discBlock := range reorgResult.Disconnected {
			for i := 1; i < len(discBlock.Transactions); i++ {
				tx := discBlock.Transactions[i]
				_ = s.mp.Add(&tx, s.db) // best-effort; ignore errors
			}
		}
	} else {
		s.mp.RemoveMined(block)
		s.mp.RemoveConflicting(block)
	}

	newTip := s.chain.Tip()
	// Broadcast the newly mined block to all peers.
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
// It admits the transaction to the local mempool, then re-broadcasts it.
func (s *nodeServices) ReceiveTx(ctx context.Context, tx *types.Transaction) error {
	if s.gossiper != nil {
		return s.gossiper.ReceiveTx(ctx, tx)
	}
	// Gossiper not yet wired (should not happen in production).
	return s.mp.Add(tx, s.db)
}

// ReceiveBlock handles an inbound block from a peer (via POST /p2p/block).
// It connects the block locally, then re-broadcasts it.
func (s *nodeServices) ReceiveBlock(ctx context.Context, block *types.Block) error {
	if s.gossiper != nil {
		return s.gossiper.ReceiveBlock(ctx, block)
	}
	// Gossiper not yet wired (should not happen in production).
	_, err := s.chain.ImportBlock(block, time.Now().Unix())
	return err
}

// ── p2p.Callbacks implementation ─────────────────────────────────────────────

// HandleReceivedTx is called by the Gossiper after deduplication. It admits
// the transaction to the mempool. Errors are policy-level and not fatal.
func (s *nodeServices) HandleReceivedTx(_ context.Context, tx *types.Transaction) error {
	return s.mp.Add(tx, s.db)
}

// HandleReceivedBlock is called by the Gossiper after deduplication. It
// connects the block and revalidates the mempool.
func (s *nodeServices) HandleReceivedBlock(_ context.Context, block *types.Block) error {
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
				_ = s.mp.Add(&tx, s.db) // best-effort
			}
		}
	} else {
		s.mp.RemoveMined(block)
		s.mp.RemoveConflicting(block)
	}
	return nil
}
