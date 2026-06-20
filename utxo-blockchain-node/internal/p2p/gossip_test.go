package p2p_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/p2p"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── stub callbacks ────────────────────────────────────────────────────────────

type stubCallbacks struct {
	mu          sync.Mutex
	txReceived  []*types.Transaction
	blkReceived []*types.Block
	txErr       error
	blkErr      error
}

func (s *stubCallbacks) HandleReceivedTx(_ context.Context, tx *types.Transaction) error {
	if s.txErr != nil {
		return s.txErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.txReceived = append(s.txReceived, tx)
	return nil
}

func (s *stubCallbacks) HandleReceivedBlock(_ context.Context, blk *types.Block) error {
	if s.blkErr != nil {
		return s.blkErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blkReceived = append(s.blkReceived, blk)
	return nil
}

func (s *stubCallbacks) txCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.txReceived)
}

func (s *stubCallbacks) blkCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.blkReceived)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// discardLog returns a logger that writes nothing.
func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// sampleTx returns a minimal coinbase-like transaction with a unique nonce
// baked into the output value so that each call produces a distinct TxID.
func sampleTx(value types.Amount) *types.Transaction {
	return &types.Transaction{
		Version: 1,
		Inputs: []types.TxInput{{
			PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex},
		}},
		Outputs: []types.TxOutput{{Value: value}},
	}
}

// sampleBlock returns a minimal block with a coinbase of the given value.
func sampleBlock(value types.Amount, prevHash types.Hash32, timestamp int64) *types.Block {
	return &types.Block{
		Header: types.BlockHeader{
			Version:   1,
			PrevHash:  prevHash,
			Timestamp: timestamp,
		},
		Transactions: []types.Transaction{
			{
				Version: 1,
				Inputs: []types.TxInput{{
					PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex},
				}},
				Outputs: []types.TxOutput{{Value: value}},
			},
		},
	}
}

// startFakePeer starts an HTTP test server that records POST bodies at
// /p2p/tx and /p2p/block and returns an always-200 response.
func startFakePeer(t *testing.T) (baseURL string, txHits *atomic.Int32, blkHits *atomic.Int32) {
	t.Helper()
	var th, bh atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /p2p/tx", func(w http.ResponseWriter, _ *http.Request) {
		th.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /p2p/block", func(w http.ResponseWriter, _ *http.Request) {
		bh.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL, &th, &bh
}

// waitForCount polls f() until it returns >= want or the timeout elapses.
func waitForCount(t *testing.T, f func() int32, want int32, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if f() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("timeout waiting for count %d (got %d)", want, f())
}

// ── Gossiper broadcast tests ──────────────────────────────────────────────────

func TestGossiper_BroadcastTx_ReachesPeers(t *testing.T) {
	url1, txHits1, _ := startFakePeer(t)
	url2, txHits2, _ := startFakePeer(t)

	g := p2p.New([]string{url1, url2}, discardLog(), nil)
	tx := sampleTx(1000)

	g.BroadcastTx(context.Background(), tx)

	waitForCount(t, txHits1.Load, 1, time.Second)
	waitForCount(t, txHits2.Load, 1, time.Second)
}

func TestGossiper_BroadcastBlock_ReachesPeers(t *testing.T) {
	url1, _, blkHits1 := startFakePeer(t)
	url2, _, blkHits2 := startFakePeer(t)

	g := p2p.New([]string{url1, url2}, discardLog(), nil)
	blk := sampleBlock(50_000_000, types.ZeroHash, time.Now().Unix())

	g.BroadcastBlock(context.Background(), blk)

	waitForCount(t, blkHits1.Load, 1, time.Second)
	waitForCount(t, blkHits2.Load, 1, time.Second)
}

func TestGossiper_BroadcastTx_DuplicateSuppressed(t *testing.T) {
	url, txHits, _ := startFakePeer(t)
	g := p2p.New([]string{url}, discardLog(), nil)
	tx := sampleTx(2000)

	g.BroadcastTx(context.Background(), tx)
	g.BroadcastTx(context.Background(), tx) // same TxID — must be suppressed

	// Wait a bit to ensure the second call had time to fire if it were going to.
	waitForCount(t, txHits.Load, 1, time.Second)
	time.Sleep(50 * time.Millisecond)
	if txHits.Load() != 1 {
		t.Errorf("peer hit count = %d, want exactly 1 (duplicate should be suppressed)", txHits.Load())
	}
}

func TestGossiper_BroadcastBlock_DuplicateSuppressed(t *testing.T) {
	url, _, blkHits := startFakePeer(t)
	g := p2p.New([]string{url}, discardLog(), nil)
	blk := sampleBlock(50_000_000, types.ZeroHash, time.Now().Unix())

	g.BroadcastBlock(context.Background(), blk)
	g.BroadcastBlock(context.Background(), blk)

	waitForCount(t, blkHits.Load, 1, time.Second)
	time.Sleep(50 * time.Millisecond)
	if blkHits.Load() != 1 {
		t.Errorf("peer hit count = %d, want exactly 1", blkHits.Load())
	}
}

func TestGossiper_BroadcastTx_NoPeers_NoPanic(t *testing.T) {
	g := p2p.New(nil, discardLog(), nil)
	g.BroadcastTx(context.Background(), sampleTx(500))
	// Just verifying no panic occurs.
}

func TestGossiper_BroadcastTx_PeerDown_NoError(t *testing.T) {
	// Point at a server that is never started — broadcast should swallow the error.
	g := p2p.New([]string{"http://127.0.0.1:19999"}, discardLog(), nil)
	done := make(chan struct{})
	go func() {
		g.BroadcastTx(context.Background(), sampleTx(300))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("BroadcastTx with dead peer hung longer than timeout")
	}
}

// ── ReceiveTx / ReceiveBlock tests ────────────────────────────────────────────

func TestGossiper_ReceiveTx_CallsCallback(t *testing.T) {
	cb := &stubCallbacks{}
	g := p2p.New(nil, discardLog(), cb)

	tx := sampleTx(4000)
	if err := g.ReceiveTx(context.Background(), tx); err != nil {
		t.Fatalf("ReceiveTx: %v", err)
	}
	if cb.txCount() != 1 {
		t.Errorf("callback txReceived count = %d, want 1", cb.txCount())
	}
}

func TestGossiper_ReceiveTx_DuplicateSuppressed(t *testing.T) {
	cb := &stubCallbacks{}
	g := p2p.New(nil, discardLog(), cb)

	tx := sampleTx(5000)
	_ = g.ReceiveTx(context.Background(), tx)
	_ = g.ReceiveTx(context.Background(), tx) // same TxID

	if cb.txCount() != 1 {
		t.Errorf("callback called %d times, want exactly 1", cb.txCount())
	}
}

func TestGossiper_ReceiveBlock_CallsCallback(t *testing.T) {
	cb := &stubCallbacks{}
	g := p2p.New(nil, discardLog(), cb)

	blk := sampleBlock(50_000_000, types.ZeroHash, time.Now().Unix())
	if err := g.ReceiveBlock(context.Background(), blk); err != nil {
		t.Fatalf("ReceiveBlock: %v", err)
	}
	if cb.blkCount() != 1 {
		t.Errorf("callback blkReceived count = %d, want 1", cb.blkCount())
	}
}

func TestGossiper_ReceiveBlock_DuplicateSuppressed(t *testing.T) {
	cb := &stubCallbacks{}
	g := p2p.New(nil, discardLog(), cb)

	blk := sampleBlock(50_000_000, types.ZeroHash, time.Now().Unix())
	_ = g.ReceiveBlock(context.Background(), blk)
	_ = g.ReceiveBlock(context.Background(), blk)

	if cb.blkCount() != 1 {
		t.Errorf("callback called %d times, want exactly 1", cb.blkCount())
	}
}

// TestGossiper_ReceiveTx_ReBroadcasts verifies that a received (new) transaction
// is forwarded to the configured peers.
func TestGossiper_ReceiveTx_ReBroadcasts(t *testing.T) {
	peerURL, txHits, _ := startFakePeer(t)
	cb := &stubCallbacks{}
	g := p2p.New([]string{peerURL}, discardLog(), cb)

	tx := sampleTx(6000)
	_ = g.ReceiveTx(context.Background(), tx)

	waitForCount(t, txHits.Load, 1, time.Second)
}

// TestGossiper_ReceiveBlock_ReBroadcasts verifies that a received valid block
// is forwarded to peers.
func TestGossiper_ReceiveBlock_ReBroadcasts(t *testing.T) {
	peerURL, _, blkHits := startFakePeer(t)
	cb := &stubCallbacks{}
	g := p2p.New([]string{peerURL}, discardLog(), cb)

	blk := sampleBlock(50_000_000, types.ZeroHash, time.Now().Unix())
	_ = g.ReceiveBlock(context.Background(), blk)

	waitForCount(t, blkHits.Load, 1, time.Second)
}

// TestGossiper_ReceiveBlock_CallbackError_NoReBroadcast verifies that when
// HandleReceivedBlock returns an error (e.g. orphan block), the block is NOT
// re-broadcast.
func TestGossiper_ReceiveBlock_CallbackError_NoReBroadcast(t *testing.T) {
	peerURL, _, blkHits := startFakePeer(t)
	cb := &stubCallbacks{blkErr: errOrphan}
	g := p2p.New([]string{peerURL}, discardLog(), cb)

	blk := sampleBlock(50_000_000, types.ZeroHash, time.Now().Unix())
	_ = g.ReceiveBlock(context.Background(), blk)

	// Give goroutines a moment to fire if they were going to.
	time.Sleep(100 * time.Millisecond)
	if blkHits.Load() != 0 {
		t.Errorf("peer should not receive invalid block, hit count = %d", blkHits.Load())
	}
}

// ── API endpoint tests (POST /p2p/tx, POST /p2p/block) ───────────────────────

// testServer creates a minimal HTTP server that wires the p2p endpoints the
// same way the real api.Server does, without depending on the api package.
func testP2PServer(t *testing.T, g *p2p.Gossiper) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /p2p/tx", func(w http.ResponseWriter, r *http.Request) {
		var tx types.Transaction
		if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := g.ReceiveTx(r.Context(), &tx); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /p2p/block", func(w http.ResponseWriter, r *http.Request) {
		var blk types.Block
		if err := json.NewDecoder(r.Body).Decode(&blk); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := g.ReceiveBlock(r.Context(), &blk); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestP2PEndpoint_PostTx_204(t *testing.T) {
	cb := &stubCallbacks{}
	g := p2p.New(nil, discardLog(), cb)
	srv := testP2PServer(t, g)

	tx := sampleTx(7000)
	body, _ := json.Marshal(tx)
	resp, err := http.Post(srv.URL+"/p2p/tx", "application/json", jsonBody(body))
	if err != nil {
		t.Fatalf("POST /p2p/tx: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if cb.txCount() != 1 {
		t.Errorf("callback count = %d, want 1", cb.txCount())
	}
}

func TestP2PEndpoint_PostBlock_204(t *testing.T) {
	cb := &stubCallbacks{}
	g := p2p.New(nil, discardLog(), cb)
	srv := testP2PServer(t, g)

	blk := sampleBlock(50_000_000, types.ZeroHash, time.Now().Unix())
	body, _ := json.Marshal(blk)
	resp, err := http.Post(srv.URL+"/p2p/block", "application/json", jsonBody(body))
	if err != nil {
		t.Fatalf("POST /p2p/block: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if cb.blkCount() != 1 {
		t.Errorf("callback count = %d, want 1", cb.blkCount())
	}
}

func TestP2PEndpoint_PostTx_InvalidJSON_400(t *testing.T) {
	g := p2p.New(nil, discardLog(), nil)
	srv := testP2PServer(t, g)

	resp, err := http.Post(srv.URL+"/p2p/tx", "application/json", jsonBody([]byte("not-json")))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestP2PEndpoint_PostBlock_InvalidJSON_400(t *testing.T) {
	g := p2p.New(nil, discardLog(), nil)
	srv := testP2PServer(t, g)

	resp, err := http.Post(srv.URL+"/p2p/block", "application/json", jsonBody([]byte("{bad")))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

var errOrphan = errors.New("orphan block: parent not found")

// jsonBody wraps a byte slice in an io.Reader suitable for http.Post.
func jsonBody(b []byte) io.Reader {
	return bytes.NewReader(b)
}
