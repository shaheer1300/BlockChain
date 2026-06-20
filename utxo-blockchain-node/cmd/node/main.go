// Command node is the entry point for the UTXO blockchain node. It loads
// configuration from environment variables, initialises all subsystems,
// and runs until it receives SIGINT or SIGTERM, then shuts down cleanly.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/config"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/node"
)

func main() {
	// Use info-level logging until config is loaded and we know the
	// configured log level.
	log := buildLogger("info")

	cfg, err := config.Load()
	if err != nil {
		log.Error("configuration error", "err", err)
		os.Exit(1)
	}

	// Rebuild the logger at the configured level.
	log = buildLogger(cfg.LogLevel)

	n, err := node.New(cfg, log)
	if err != nil {
		log.Error("failed to initialise node", "err", err)
		os.Exit(1)
	}

	// Cancel the context on SIGINT (Ctrl+C) or SIGTERM (container stop).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := n.Run(ctx); err != nil {
		log.Error("node exited with error", "err", err)
		os.Exit(1)
	}
}

// buildLogger constructs a structured slog.Logger that writes to stderr.
// Unrecognised level strings default to Info.
func buildLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
