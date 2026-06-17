// Package node is the top-level orchestrator. It wires together storage,
// the HTTP API, and (in later milestones) the chain manager, mempool,
// miner, and P2P gossip. Its only responsibilities at Milestone 1 are:
// create the data directory, open the database, start the HTTP server,
// and perform an ordered graceful shutdown.
package node

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/api"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/config"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/storage"
)

// shutdownGrace is the maximum time allowed for a graceful API shutdown
// before connections are forcibly closed.
const shutdownGrace = 10 * time.Second

// Node owns all subsystems and manages their lifecycle.
type Node struct {
	cfg *config.Config
	log *slog.Logger
	db  *storage.DB
	api *api.Server
}

// New initialises all subsystems. It creates DataDir if it does not
// exist, opens the bbolt database, and constructs the API server — but
// does not start listening yet.
func New(cfg *config.Config, log *slog.Logger) (*Node, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("node: create data dir %q: %w", cfg.DataDir, err)
	}

	dbPath := filepath.Join(cfg.DataDir, "chain.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("node: open storage: %w", err)
	}

	srv := api.New(cfg, log)

	return &Node{
		cfg: cfg,
		log: log,
		db:  db,
		api: srv,
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
