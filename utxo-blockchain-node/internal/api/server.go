// Package api implements the HTTP API server. Handlers in this package
// are thin adapters: parse the request, call service methods, write JSON.
// No consensus logic belongs here.
package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/config"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

// Server owns the HTTP listener and all route handlers.
type Server struct {
	cfg  *config.Config
	log  *slog.Logger
	svc  Services           // nil in skeleton mode — routes that need it return 503
	diag DiagnosticServices // optional; enables GET /utxos and GET /blocks
	demo DemoServices       // optional; enables POST/GET /demo/*
	http *http.Server
}

// Options bundles optional sub-services. Pass nil sub-services to disable
// the corresponding routes (they will return 404 because they are not
// registered when the matching interface is nil).
type Options struct {
	Diagnostic DiagnosticServices
	Demo       DemoServices
	// EnableCORS, when true, wraps the mux with a permissive CORS handler
	// suitable for local development frontends.
	EnableCORS bool
}

// New constructs a Server and registers all routes. svc may be nil during
// early startup or in tests that only exercise /health and /peers. Any route
// that requires svc returns 503 when svc is nil. opts may be nil.
func New(cfg *config.Config, log *slog.Logger, svc Services, opts *Options) *Server {
	if opts == nil {
		opts = &Options{}
	}
	s := &Server{
		cfg:  cfg,
		log:  log,
		svc:  svc,
		diag: opts.Diagnostic,
		demo: opts.Demo,
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	var handler http.Handler = mux
	if opts.EnableCORS {
		handler = corsMiddleware(handler)
	}

	s.http = &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	return s
}

// Start begins accepting connections. It returns immediately; the listener
// runs in its own goroutine.
func (s *Server) Start() error {
	s.log.Info("API server starting", "addr", s.cfg.HTTPAddr)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("API server error", "err", err)
		}
	}()
	return nil
}

// Shutdown gracefully drains active connections. It blocks until all
// connections are idle or ctx is cancelled.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("API server shutting down")
	return s.http.Shutdown(ctx)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("GET /blocks/{hash}", s.handleGetBlock)
	mux.HandleFunc("GET /utxos/{address}", s.handleGetUTXOs)
	mux.HandleFunc("GET /balance/{address}", s.handleGetBalance)
	mux.HandleFunc("GET /mempool", s.handleGetMempool)
	mux.HandleFunc("POST /tx", s.handleSubmitTx)
	mux.HandleFunc("POST /mine", s.handleMine)
	mux.HandleFunc("GET /peers", s.handleGetPeers)
	// P2P gossip receive endpoints — called by remote peers.
	mux.HandleFunc("POST /p2p/tx", s.handleP2PTx)
	mux.HandleFunc("POST /p2p/block", s.handleP2PBlock)

	// Diagnostic (read-only) endpoints — registered only if a
	// DiagnosticServices implementation was provided.
	if s.diag != nil {
		mux.HandleFunc("GET /utxos", s.handleListUTXOs)
		mux.HandleFunc("GET /blocks", s.handleListBlocks)
	}

	// Educational demo endpoints — registered only if a DemoServices
	// implementation was provided.
	if s.demo != nil {
		mux.HandleFunc("GET /demo/state", s.handleDemoState)
		mux.HandleFunc("GET /demo/wallets", s.handleDemoListWallets)
		mux.HandleFunc("POST /demo/wallets", s.handleDemoCreateWallet)
		mux.HandleFunc("POST /demo/tx", s.handleDemoBuildTx)
		mux.HandleFunc("POST /demo/double-spend", s.handleDemoDoubleSpend)
		mux.HandleFunc("POST /demo/mine", s.handleDemoMine)
		mux.HandleFunc("POST /demo/reset", s.handleDemoReset)
		mux.HandleFunc("POST /demo/hard-reset", s.handleDemoHardReset)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// errorResponse is the JSON envelope for all error responses.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON sets Content-Type, writes the status code, and encodes v as JSON.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a structured JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// requireSvc checks that s.svc is available and writes 503 if not.
func (s *Server) requireSvc(w http.ResponseWriter) bool {
	if s.svc == nil {
		writeError(w, http.StatusServiceUnavailable, "node services not yet initialized")
		return false
	}
	return true
}

// parseHash decodes a 64-character hex string into a Hash32.
func parseHash(raw string) (types.Hash32, error) {
	var h types.Hash32
	if err := h.SetHex(raw); err != nil {
		return h, err
	}
	return h, nil
}

// parseAddress decodes a 40-character hex string into an Address.
func parseAddress(raw string) (types.Address, error) {
	b, err := hex.DecodeString(raw)
	if err != nil {
		return types.Address{}, fmt.Errorf("invalid hex: %w", err)
	}
	if len(b) != types.AddressSize {
		return types.Address{}, fmt.Errorf("address must be %d bytes (%d hex chars), got %d bytes",
			types.AddressSize, types.AddressSize*2, len(b))
	}
	var addr types.Address
	copy(addr[:], b)
	return addr, nil
}

// ── handlers ──────────────────────────────────────────────────────────────────

// healthResponse is the JSON payload returned by GET /health.
type healthResponse struct {
	Status  string `json:"status"`
	NodeID  string `json:"node_id"`
	Network string `json:"network"`
}

// GET /health — always available; does not require svc.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		NodeID:  s.cfg.NodeID,
		Network: s.cfg.NetworkID,
	})
}

// GET /status — returns chain height and best-tip hash.
func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	if !s.requireSvc(w) {
		return
	}
	writeJSON(w, http.StatusOK, s.svc.Status())
}

// GET /blocks/{hash} — returns the full block for the given hash.
func (s *Server) handleGetBlock(w http.ResponseWriter, r *http.Request) {
	if !s.requireSvc(w) {
		return
	}
	hash, err := parseHash(r.PathValue("hash"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid block hash: "+err.Error())
		return
	}
	block, err := s.svc.GetBlock(hash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if block == nil {
		writeError(w, http.StatusNotFound, "block not found")
		return
	}
	writeJSON(w, http.StatusOK, block)
}

// utxosResponse is the JSON payload for GET /utxos/{address}.
type utxosResponse struct {
	Address string        `json:"address"`
	UTXOs   []*types.UTXO `json:"utxos"`
}

// GET /utxos/{address} — returns all UTXOs for a given address.
func (s *Server) handleGetUTXOs(w http.ResponseWriter, r *http.Request) {
	if !s.requireSvc(w) {
		return
	}
	addr, err := parseAddress(r.PathValue("address"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid address: "+err.Error())
		return
	}
	utxos, err := s.svc.GetUTXOsByAddress(addr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if utxos == nil {
		utxos = []*types.UTXO{}
	}
	writeJSON(w, http.StatusOK, utxosResponse{Address: addr.String(), UTXOs: utxos})
}

// balanceResponse is the JSON payload for GET /balance/{address}.
type balanceResponse struct {
	Address string       `json:"address"`
	Balance types.Amount `json:"balance"`
}

// GET /balance/{address} — returns the total confirmed balance for an address.
func (s *Server) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	if !s.requireSvc(w) {
		return
	}
	addr, err := parseAddress(r.PathValue("address"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid address: "+err.Error())
		return
	}
	utxos, err := s.svc.GetUTXOsByAddress(addr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var balance types.Amount
	for _, u := range utxos {
		next, addErr := balance.SafeAdd(u.Output.Value)
		if addErr != nil {
			writeError(w, http.StatusInternalServerError, "balance overflow")
			return
		}
		balance = next
	}
	writeJSON(w, http.StatusOK, balanceResponse{Address: addr.String(), Balance: balance})
}

// mempoolResponse is the JSON payload for GET /mempool.
type mempoolResponse struct {
	Count   int                   `json:"count"`
	Entries []*types.MempoolEntry `json:"entries"`
}

// GET /mempool — returns a snapshot of the mempool sorted by fee rate.
func (s *Server) handleGetMempool(w http.ResponseWriter, _ *http.Request) {
	if !s.requireSvc(w) {
		return
	}
	entries := s.svc.GetMempool()
	if entries == nil {
		entries = []*types.MempoolEntry{}
	}
	writeJSON(w, http.StatusOK, mempoolResponse{Count: len(entries), Entries: entries})
}

// submitTxResponse is the JSON payload for POST /tx.
type submitTxResponse struct {
	TxID string `json:"txid"`
}

// POST /tx — validates and submits a transaction to the mempool.
func (s *Server) handleSubmitTx(w http.ResponseWriter, r *http.Request) {
	if !s.requireSvc(w) {
		return
	}
	var tx types.Transaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction JSON: "+err.Error())
		return
	}
	if err := s.svc.SubmitTx(&tx); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, submitTxResponse{TxID: tx.TxID().String()})
}

// POST /mine — mines one block using mempool transactions and the configured
// miner address. The request body is ignored.
func (s *Server) handleMine(w http.ResponseWriter, r *http.Request) {
	if !s.requireSvc(w) {
		return
	}
	result, err := s.svc.Mine(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// peersResponse is the JSON payload for GET /peers.
type peersResponse struct {
	Peers []string `json:"peers"`
}

// POST /p2p/tx — receive a transaction pushed by a peer.
// The node deduplicates, validates, adds to mempool, and re-broadcasts.
func (s *Server) handleP2PTx(w http.ResponseWriter, r *http.Request) {
	if !s.requireSvc(w) {
		return
	}
	var tx types.Transaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction JSON: "+err.Error())
		return
	}
	if err := s.svc.ReceiveTx(r.Context(), &tx); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /p2p/block — receive a block pushed by a peer.
// The node deduplicates, validates, connects (or stores as side-chain), and re-broadcasts.
func (s *Server) handleP2PBlock(w http.ResponseWriter, r *http.Request) {
	if !s.requireSvc(w) {
		return
	}
	var block types.Block
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		writeError(w, http.StatusBadRequest, "invalid block JSON: "+err.Error())
		return
	}
	if err := s.svc.ReceiveBlock(r.Context(), &block); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /peers — returns the configured peer list; does not require svc.
func (s *Server) handleGetPeers(w http.ResponseWriter, _ *http.Request) {
	peers := s.cfg.Peers
	if peers == nil {
		peers = []string{}
	}
	// If services are available, delegate to svc (allows node to report
	// dynamically discovered peers in future milestones).
	if s.svc != nil {
		peers = s.svc.GetPeers()
		if peers == nil {
			peers = []string{}
		}
	}
	writeJSON(w, http.StatusOK, peersResponse{Peers: peers})
}
