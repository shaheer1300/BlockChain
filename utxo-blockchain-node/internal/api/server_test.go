package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/config"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── mock services ─────────────────────────────────────────────────────────────

// mockSvc is a test-double for Services. Each method calls the corresponding
// function field if non-nil, otherwise returns a safe zero value.
type mockSvc struct {
	statusFn            func() StatusResult
	getBlockFn          func(types.Hash32) (*types.Block, error)
	getUTXOsByAddressFn func(types.Address) ([]*types.UTXO, error)
	submitTxFn          func(*types.Transaction) error
	mineFn              func(context.Context) (*MineResult, error)
	getPeersFn          func() []string
	getMempoolFn        func() []*types.MempoolEntry
	receiveTxFn         func(context.Context, *types.Transaction) error
	receiveBlockFn      func(context.Context, *types.Block) error
}

func (m *mockSvc) Status() StatusResult {
	if m.statusFn != nil {
		return m.statusFn()
	}
	return StatusResult{NodeID: "mock", Network: "mocknet"}
}
func (m *mockSvc) GetBlock(h types.Hash32) (*types.Block, error) {
	if m.getBlockFn != nil {
		return m.getBlockFn(h)
	}
	return nil, nil
}
func (m *mockSvc) GetUTXOsByAddress(a types.Address) ([]*types.UTXO, error) {
	if m.getUTXOsByAddressFn != nil {
		return m.getUTXOsByAddressFn(a)
	}
	return nil, nil
}
func (m *mockSvc) SubmitTx(tx *types.Transaction) error {
	if m.submitTxFn != nil {
		return m.submitTxFn(tx)
	}
	return nil
}
func (m *mockSvc) Mine(ctx context.Context) (*MineResult, error) {
	if m.mineFn != nil {
		return m.mineFn(ctx)
	}
	return &MineResult{Height: 1}, nil
}
func (m *mockSvc) GetPeers() []string {
	if m.getPeersFn != nil {
		return m.getPeersFn()
	}
	return []string{}
}
func (m *mockSvc) GetMempool() []*types.MempoolEntry {
	if m.getMempoolFn != nil {
		return m.getMempoolFn()
	}
	return nil
}
func (m *mockSvc) ReceiveTx(ctx context.Context, tx *types.Transaction) error {
	if m.receiveTxFn != nil {
		return m.receiveTxFn(ctx, tx)
	}
	return nil
}
func (m *mockSvc) ReceiveBlock(ctx context.Context, blk *types.Block) error {
	if m.receiveBlockFn != nil {
		return m.receiveBlockFn(ctx, blk)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return newTestServerWith(t, nil)
}

func newTestServerWith(t *testing.T, svc Services) *Server {
	t.Helper()
	cfg := &config.Config{
		NodeID:    "testnode",
		NetworkID: "testnet",
		HTTPAddr:  "127.0.0.1:0",
		Peers:     []string{"http://peer1:8001"},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(cfg, log, svc)
}

func serveRequest(s *Server, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

// ── GET /health ───────────────────────────────────────────────────────────────

func TestHandleHealth_Status200(t *testing.T) {
	rec := serveRequest(newTestServer(t), httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleHealth_Body(t *testing.T) {
	rec := serveRequest(newTestServer(t), httptest.NewRequest(http.MethodGet, "/health", nil))
	var got healthResponse
	decodeJSON(t, rec, &got)
	if got.Status != "ok" {
		t.Errorf("status = %q, want ok", got.Status)
	}
	if got.NodeID != "testnode" {
		t.Errorf("node_id = %q, want testnode", got.NodeID)
	}
	if got.Network != "testnet" {
		t.Errorf("network = %q, want testnet", got.Network)
	}
}

func TestHandleHealth_ContentType(t *testing.T) {
	rec := serveRequest(newTestServer(t), httptest.NewRequest(http.MethodGet, "/health", nil))
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestHandleHealth_PostMethodNotAllowed(t *testing.T) {
	rec := serveRequest(newTestServer(t), httptest.NewRequest(http.MethodPost, "/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// ── nil services returns 503 ──────────────────────────────────────────────────

func TestHandleStatus_NilSvc_503(t *testing.T) {
	rec := serveRequest(newTestServer(t), httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestHandleMine_NilSvc_503(t *testing.T) {
	rec := serveRequest(newTestServer(t), httptest.NewRequest(http.MethodPost, "/mine", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

// ── GET /status ───────────────────────────────────────────────────────────────

func TestHandleStatus_ReturnsHeightAndTip(t *testing.T) {
	height := uint32(5)
	tipHash := types.Hash32{0xAB, 0xCD}
	svc := &mockSvc{
		statusFn: func() StatusResult {
			return StatusResult{
				NodeID:  "node1",
				Network: "testnet",
				Height:  &height,
				TipHash: &tipHash,
			}
		},
	}
	rec := serveRequest(newTestServerWith(t, svc), httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got StatusResult
	decodeJSON(t, rec, &got)
	if got.Height == nil || *got.Height != height {
		t.Errorf("height = %v, want %d", got.Height, height)
	}
}

func TestHandleStatus_NilHeightWhenUninitialized(t *testing.T) {
	svc := &mockSvc{
		statusFn: func() StatusResult {
			return StatusResult{NodeID: "node1", Network: "testnet"} // no Height/TipHash
		},
	}
	rec := serveRequest(newTestServerWith(t, svc), httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got StatusResult
	decodeJSON(t, rec, &got)
	if got.Height != nil {
		t.Errorf("height should be nil for uninitialised chain, got %v", *got.Height)
	}
}

// ── GET /blocks/{hash} ────────────────────────────────────────────────────────

func TestHandleGetBlock_Found(t *testing.T) {
	var addr types.Address
	block := &types.Block{
		Header: types.BlockHeader{Version: 1},
		Transactions: []types.Transaction{{
			Version: 1,
			Inputs: []types.TxInput{{
				PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex},
			}},
			Outputs: []types.TxOutput{{Value: 50, Recipient: addr}},
		}},
	}
	svc := &mockSvc{
		getBlockFn: func(_ types.Hash32) (*types.Block, error) { return block, nil },
	}
	hashHex := block.BlockHash().String()
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodGet, "/blocks/"+hashHex, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleGetBlock_NotFound(t *testing.T) {
	svc := &mockSvc{
		getBlockFn: func(_ types.Hash32) (*types.Block, error) { return nil, nil },
	}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodGet, "/blocks/"+types.ZeroHash.String(), nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleGetBlock_InvalidHash(t *testing.T) {
	svc := &mockSvc{}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodGet, "/blocks/notahash", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ── GET /utxos/{address} ──────────────────────────────────────────────────────

func TestHandleGetUTXOs_ReturnsUTXOs(t *testing.T) {
	var addr types.Address
	addr[0] = 0x11
	utxo := &types.UTXO{
		OutPoint: types.OutPoint{TxID: types.ZeroHash, Index: 0},
		Output:   types.TxOutput{Value: 1000, Recipient: addr},
	}
	svc := &mockSvc{
		getUTXOsByAddressFn: func(_ types.Address) ([]*types.UTXO, error) {
			return []*types.UTXO{utxo}, nil
		},
	}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodGet, "/utxos/"+addr.String(), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got utxosResponse
	decodeJSON(t, rec, &got)
	if len(got.UTXOs) != 1 {
		t.Errorf("utxo count = %d, want 1", len(got.UTXOs))
	}
}

func TestHandleGetUTXOs_InvalidAddress(t *testing.T) {
	svc := &mockSvc{}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodGet, "/utxos/ZZZZ", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ── GET /balance/{address} ────────────────────────────────────────────────────

func TestHandleGetBalance_ReflectsUTXOs(t *testing.T) {
	var addr types.Address
	addr[0] = 0x22
	svc := &mockSvc{
		getUTXOsByAddressFn: func(_ types.Address) ([]*types.UTXO, error) {
			return []*types.UTXO{
				{Output: types.TxOutput{Value: 300, Recipient: addr}},
				{Output: types.TxOutput{Value: 700, Recipient: addr}},
			}, nil
		},
	}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodGet, "/balance/"+addr.String(), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got balanceResponse
	decodeJSON(t, rec, &got)
	if got.Balance != 1000 {
		t.Errorf("balance = %d, want 1000", got.Balance)
	}
}

func TestHandleGetBalance_EmptyAddress_ZeroBalance(t *testing.T) {
	var addr types.Address
	addr[0] = 0x33
	svc := &mockSvc{
		getUTXOsByAddressFn: func(_ types.Address) ([]*types.UTXO, error) {
			return nil, nil
		},
	}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodGet, "/balance/"+addr.String(), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got balanceResponse
	decodeJSON(t, rec, &got)
	if got.Balance != 0 {
		t.Errorf("balance = %d, want 0", got.Balance)
	}
}

// ── GET /mempool ──────────────────────────────────────────────────────────────

func TestHandleGetMempool_ReturnsEntries(t *testing.T) {
	entry := &types.MempoolEntry{Fee: 50, FeeRate: 5}
	svc := &mockSvc{
		getMempoolFn: func() []*types.MempoolEntry { return []*types.MempoolEntry{entry} },
	}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodGet, "/mempool", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got mempoolResponse
	decodeJSON(t, rec, &got)
	if got.Count != 1 {
		t.Errorf("count = %d, want 1", got.Count)
	}
}

func TestHandleGetMempool_Empty(t *testing.T) {
	svc := &mockSvc{getMempoolFn: func() []*types.MempoolEntry { return nil }}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodGet, "/mempool", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got mempoolResponse
	decodeJSON(t, rec, &got)
	if got.Count != 0 {
		t.Errorf("count = %d, want 0", got.Count)
	}
}

// ── POST /tx ──────────────────────────────────────────────────────────────────

func TestHandleSubmitTx_Accepted(t *testing.T) {
	submitted := false
	svc := &mockSvc{
		submitTxFn: func(_ *types.Transaction) error {
			submitted = true
			return nil
		},
	}
	tx := types.Transaction{Version: 1}
	body, _ := json.Marshal(tx)
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodPost, "/tx", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if !submitted {
		t.Error("SubmitTx was not called")
	}
	var got submitTxResponse
	decodeJSON(t, rec, &got)
	if got.TxID == "" {
		t.Error("txid should not be empty")
	}
}

func TestHandleSubmitTx_InvalidJSON(t *testing.T) {
	svc := &mockSvc{}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodPost, "/tx", bytes.NewBufferString("not-json")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleSubmitTx_RejectedByMempool(t *testing.T) {
	svc := &mockSvc{
		submitTxFn: func(_ *types.Transaction) error {
			return errors.New("mempool: transaction already in mempool")
		},
	}
	tx := types.Transaction{Version: 1}
	body, _ := json.Marshal(tx)
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodPost, "/tx", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ── POST /mine ────────────────────────────────────────────────────────────────

func TestHandleMine_Success(t *testing.T) {
	var hash types.Hash32
	hash[0] = 0x01
	svc := &mockSvc{
		mineFn: func(_ context.Context) (*MineResult, error) {
			return &MineResult{Hash: hash, Height: 1}, nil
		},
	}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodPost, "/mine", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var got MineResult
	decodeJSON(t, rec, &got)
	if got.Height != 1 {
		t.Errorf("height = %d, want 1", got.Height)
	}
}

func TestHandleMine_GetMethodNotAllowed(t *testing.T) {
	svc := &mockSvc{}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodGet, "/mine", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// ── GET /peers ────────────────────────────────────────────────────────────────

func TestHandleGetPeers_WithNilSvc(t *testing.T) {
	// /peers works even without services — reads from config.
	rec := serveRequest(newTestServer(t), httptest.NewRequest(http.MethodGet, "/peers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got peersResponse
	decodeJSON(t, rec, &got)
	// The test config sets one peer.
	if len(got.Peers) != 1 {
		t.Errorf("peers count = %d, want 1", len(got.Peers))
	}
}

func TestHandleGetPeers_WithSvc(t *testing.T) {
	svc := &mockSvc{
		getPeersFn: func() []string { return []string{"http://a:8001", "http://b:8001"} },
	}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodGet, "/peers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got peersResponse
	decodeJSON(t, rec, &got)
	if len(got.Peers) != 2 {
		t.Errorf("peers count = %d, want 2", len(got.Peers))
	}
}

func TestHandleHealth_UnknownPath(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// ── POST /p2p/tx ──────────────────────────────────────────────────────────────

func TestHandleP2PTx_Accepted_204(t *testing.T) {
	called := false
	svc := &mockSvc{
		receiveTxFn: func(_ context.Context, _ *types.Transaction) error {
			called = true
			return nil
		},
	}
	tx := types.Transaction{Version: 1}
	body, _ := json.Marshal(tx)
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodPost, "/p2p/tx", bytes.NewReader(body)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
	if !called {
		t.Error("ReceiveTx was not called")
	}
}

func TestHandleP2PTx_InvalidJSON_400(t *testing.T) {
	svc := &mockSvc{}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodPost, "/p2p/tx", bytes.NewBufferString("not-json")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleP2PTx_NilSvc_503(t *testing.T) {
	rec := serveRequest(newTestServer(t),
		httptest.NewRequest(http.MethodPost, "/p2p/tx", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// ── POST /p2p/block ───────────────────────────────────────────────────────────

func TestHandleP2PBlock_Accepted_204(t *testing.T) {
	called := false
	svc := &mockSvc{
		receiveBlockFn: func(_ context.Context, _ *types.Block) error {
			called = true
			return nil
		},
	}
	blk := types.Block{Header: types.BlockHeader{Version: 1}}
	body, _ := json.Marshal(blk)
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodPost, "/p2p/block", bytes.NewReader(body)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
	if !called {
		t.Error("ReceiveBlock was not called")
	}
}

func TestHandleP2PBlock_InvalidJSON_400(t *testing.T) {
	svc := &mockSvc{}
	rec := serveRequest(newTestServerWith(t, svc),
		httptest.NewRequest(http.MethodPost, "/p2p/block", bytes.NewBufferString("{bad")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleP2PBlock_NilSvc_503(t *testing.T) {
	rec := serveRequest(newTestServer(t),
		httptest.NewRequest(http.MethodPost, "/p2p/block", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
