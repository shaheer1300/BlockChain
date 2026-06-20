package config

import "testing"

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"NODE_ID", "NETWORK_ID", "HTTP_ADDR", "DATA_DIR",
		"MINER_ADDRESS", "PEERS", "LOG_LEVEL", "POW_TARGET_PREFIX_ZEROES",
	} {
		t.Setenv(key, "")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	cases := map[string]string{
		"NodeID":    cfg.NodeID,
		"NetworkID": cfg.NetworkID,
		"HTTPAddr":  cfg.HTTPAddr,
		"DataDir":   cfg.DataDir,
		"LogLevel":  cfg.LogLevel,
	}
	want := map[string]string{
		"NodeID":    "node1",
		"NetworkID": "localdev",
		"HTTPAddr":  "127.0.0.1:8001",
		"DataDir":   "./data/node1",
		"LogLevel":  "info",
	}
	for field, got := range cases {
		if got != want[field] {
			t.Errorf("%s = %q, want %q", field, got, want[field])
		}
	}
	if cfg.PowTargetPrefixZeroes != 4 {
		t.Errorf("PowTargetPrefixZeroes = %d, want 4", cfg.PowTargetPrefixZeroes)
	}
	if len(cfg.Peers) != 0 {
		t.Errorf("Peers = %v, want empty", cfg.Peers)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("NODE_ID", "node99")
	t.Setenv("HTTP_ADDR", "0.0.0.0:9090")
	t.Setenv("NETWORK_ID", "testnet")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.NodeID != "node99" {
		t.Errorf("NodeID = %q, want node99", cfg.NodeID)
	}
	if cfg.HTTPAddr != "0.0.0.0:9090" {
		t.Errorf("HTTPAddr = %q, want 0.0.0.0:9090", cfg.HTTPAddr)
	}
	if cfg.NetworkID != "testnet" {
		t.Errorf("NetworkID = %q, want testnet", cfg.NetworkID)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}

func TestLoad_PeersCommaSeparated(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("PEERS", "127.0.0.1:8002 , 127.0.0.1:8003, 127.0.0.1:8004 ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Peers) != 3 {
		t.Fatalf("len(Peers) = %d, want 3", len(cfg.Peers))
	}
	if cfg.Peers[0] != "127.0.0.1:8002" {
		t.Errorf("Peers[0] = %q, want 127.0.0.1:8002", cfg.Peers[0])
	}
	if cfg.Peers[2] != "127.0.0.1:8004" {
		t.Errorf("Peers[2] = %q, want 127.0.0.1:8004", cfg.Peers[2])
	}
}

func TestLoad_PeersEmpty(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("PEERS", "   ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Peers) != 0 {
		t.Errorf("Peers = %v, want empty for whitespace-only input", cfg.Peers)
	}
}

func TestLoad_InvalidPow(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("POW_TARGET_PREFIX_ZEROES", "notanumber")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-integer POW_TARGET_PREFIX_ZEROES")
	}
}

func TestLoad_NegativePow(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("POW_TARGET_PREFIX_ZEROES", "-1")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for negative POW_TARGET_PREFIX_ZEROES")
	}
}

func TestLoad_PowOverride(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("POW_TARGET_PREFIX_ZEROES", "6")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PowTargetPrefixZeroes != 6 {
		t.Errorf("PowTargetPrefixZeroes = %d, want 6", cfg.PowTargetPrefixZeroes)
	}
}
