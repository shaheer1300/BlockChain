// Package config loads and validates node configuration from environment
// variables, applying safe defaults for a single-node local development
// setup. No external dependencies — stdlib only.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for the node. Every field has a
// sensible default so the binary runs out of the box without any environment
// variables.
type Config struct {
	// NodeID identifies this node in logs and peer gossip messages.
	NodeID string
	// NetworkID must match across all peers in the same network.
	// Nodes with differing NetworkIDs reject each other's blocks.
	NetworkID string
	// HTTPAddr is the address:port the HTTP API server binds to.
	HTTPAddr string
	// DataDir is the directory where the bbolt database and other
	// persistent files are stored.
	DataDir string
	// MinerAddress is the pay-to address for coinbase outputs. An empty
	// string disables local block production.
	MinerAddress string
	// Peers is the list of peer HTTP base URLs used for gossip.
	Peers []string
	// LogLevel controls structured log verbosity.
	// Valid values: "debug", "info", "warn", "error".
	LogLevel string
	// PowTargetPrefixZeroes is the number of leading zero hex nibbles
	// required by proof-of-work. 4 means ~65 k hashes on average.
	PowTargetPrefixZeroes int
	// DemoMode enables the educational demo subsystem: extra HTTP routes
	// under /demo/*, the diagnostic /utxos and /blocks list endpoints,
	// in-memory demo wallets persisted to data/wallets.json, and a
	// permissive CORS handler suitable for the local web/ frontend.
	// Never enable in a production deployment.
	DemoMode bool
}

// Load reads environment variables and returns a fully-validated Config.
// It never returns a partially-populated Config alongside an error.
func Load() (*Config, error) {
	pow, err := parseNonNegativeInt("POW_TARGET_PREFIX_ZEROES", "4")
	if err != nil {
		return nil, err
	}

	return &Config{
		NodeID:                envOr("NODE_ID", "node1"),
		NetworkID:             envOr("NETWORK_ID", "localdev"),
		HTTPAddr:              envOr("HTTP_ADDR", "127.0.0.1:8001"),
		DataDir:               envOr("DATA_DIR", "./data/node1"),
		MinerAddress:          os.Getenv("MINER_ADDRESS"),
		Peers:                 parsePeers(os.Getenv("PEERS")),
		LogLevel:              envOr("LOG_LEVEL", "info"),
		PowTargetPrefixZeroes: pow,
		DemoMode:              parseBoolEnv("DEMO_MODE"),
	}, nil
}

// parseBoolEnv returns true when key is set to a truthy value
// ("1", "true", "yes", "on", case-insensitive); false otherwise.
func parseBoolEnv(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// envOr returns the value of key from the environment, or defaultVal when
// the variable is absent or empty.
func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// parsePeers splits a comma-separated peer list, trims whitespace from
// each entry, and discards empty elements. Returns nil for empty input.
func parsePeers(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	peers := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			peers = append(peers, trimmed)
		}
	}
	if len(peers) == 0 {
		return nil
	}
	return peers
}

// parseNonNegativeInt reads envKey as a non-negative integer, using
// defaultVal when the variable is absent or empty.
func parseNonNegativeInt(envKey, defaultVal string) (int, error) {
	raw := envOr(envKey, defaultVal)
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("config: %s must be a non-negative integer, got %q", envKey, raw)
	}
	return v, nil
}
