// Package node is the top-level orchestrator. It wires together storage,
// chain manager, mempool, and the HTTP API. On startup it opens the
// database, initialises the chain (genesis creation if needed), and starts
// accepting HTTP requests.
package node

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/api"
	chainfmt "github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/chain"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/config"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/mempool"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/p2p"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/storage"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// shutdownGrace is the maximum time allowed for a graceful API shutdown
// before connections are forcibly closed.
const shutdownGrace = 10 * time.Second

// Node owns all subsystems and manages their lifecycle.
type Node struct {
	cfg       *config.Config
	log       *slog.Logger
	db        *storage.DB
	chain     *chainfmt.Manager
	mp        *mempool.Mempool
	gossiper  *p2p.Gossiper
	api       *api.Server
	minerAddr types.Address // zero if MINER_ADDRESS not set
}

// New initialises all subsystems. It creates DataDir if it does not exist,
// opens the bbolt database, creates the chain manager and mempool, and
// constructs the API server — but does not start listening yet.
func New(cfg *config.Config, log *slog.Logger) (*Node, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("node: create data dir %q: %w", cfg.DataDir, err)
	}

	dbPath := filepath.Join(cfg.DataDir, "chain.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("node: open storage: %w", err)
	}

	cm, err := chainfmt.NewManager(db, cfg.PowTargetPrefixZeroes)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("node: create chain manager: %w", err)
	}

	mp := mempool.New(0) // no minimum fee rate for now

	var minerAddr types.Address
	if cfg.MinerAddress != "" {
		b, parseErr := hex.DecodeString(cfg.MinerAddress)
		if parseErr != nil || len(b) != types.AddressSize {
			_ = db.Close()
			return nil, fmt.Errorf("node: invalid MINER_ADDRESS %q: must be %d hex chars",
				cfg.MinerAddress, types.AddressSize*2)
		}
		copy(minerAddr[:], b)
	}

	svc := &nodeServices{
		cfg:        cfg,
		db:         db,
		chain:      cm,
		mp:         mp,
		minerAddr:  minerAddr,
		powNibbles: cfg.PowTargetPrefixZeroes,
	}

	// The Gossiper is created after nodeServices so we can pass svc as the
	// Callbacks implementation. We then set svc.gossiper to close the loop.
	g := p2p.New(cfg.Peers, log, svc)
	svc.gossiper = g

	// When DemoMode is enabled, attach an in-memory wallet store (persisted
	// to <data-dir>/wallets.json) and wire the diagnostic + demo HTTP
	// endpoints with a permissive CORS handler.
	var apiOpts *api.Options
	if cfg.DemoMode {
		walletsPath := filepath.Join(cfg.DataDir, "wallets.json")
		ws, wErr := newWalletStore(walletsPath)
		if wErr != nil {
			_ = db.Close()
			return nil, fmt.Errorf("node: init demo wallets: %w", wErr)
		}
		svc.wallets = ws
		apiOpts = &api.Options{
			Diagnostic: svc,
			Demo:       svc,
			EnableCORS: true,
		}
		log.Info("demo mode enabled — extra /demo/* and /utxos /blocks routes registered")
	}

	srv := api.New(cfg, log, svc, apiOpts)

	return &Node{
		cfg:       cfg,
		log:       log,
		db:        db,
		chain:     cm,
		mp:        mp,
		gossiper:  g,
		api:       srv,
		minerAddr: minerAddr,
	}, nil
}

// Run starts all subsystems and blocks until ctx is cancelled (e.g. on
// Ctrl+C or SIGTERM). It performs an ordered graceful shutdown before
// returning. A nil error indicates a clean exit.
func (n *Node) Run(ctx context.Context) error {
	n.log.Info("node starting",
		"node_id", n.cfg.NodeID,
		"network", n.cfg.NetworkID,
		"http_addr", n.cfg.HTTPAddr,
		"data_dir", n.cfg.DataDir,
	)

	// Initialise the genesis block if a miner address is configured and the
	// chain has not been started yet. InitGenesis is idempotent — safe to call
	// on every restart.
	if !n.minerAddr.IsZero() {
		if err := n.chain.InitGenesis(ctx, n.minerAddr, chainfmt.InitialSubsidy, time.Now().Unix()); err != nil {
			n.log.Warn("genesis initialization failed (chain may already exist)", "err", err)
		} else {
			n.log.Info("genesis ready", "tip", n.chain.Tip().Hash)
		}
	}

	if err := n.api.Start(); err != nil {
		_ = n.db.Close()
		return fmt.Errorf("node: start API: %w", err)
	}

	<-ctx.Done()
	return n.shutdown()
}

// shutdown performs an ordered teardown of all subsystems. It attempts
// every step regardless of earlier failures so that resources are always
// released, logging rather than propagating individual errors.
func (n *Node) shutdown() error {
	n.log.Info("node shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()

	if err := n.api.Shutdown(ctx); err != nil {
		n.log.Error("API shutdown error", "err", err)
	}
	if err := n.db.Close(); err != nil {
		n.log.Error("storage close error", "err", err)
	}

	n.log.Info("node stopped")
	return nil
}
