package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── diagnostic handlers (require s.diag) ─────────────────────────────────────

// listUTXOsResponse is the JSON payload for GET /utxos.
type listUTXOsResponse struct {
	Count int           `json:"count"`
	UTXOs []*types.UTXO `json:"utxos"`
}

// GET /utxos — returns the full UTXO set.
func (s *Server) handleListUTXOs(w http.ResponseWriter, _ *http.Request) {
	utxos, err := s.diag.GetAllUTXOs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if utxos == nil {
		utxos = []*types.UTXO{}
	}
	writeJSON(w, http.StatusOK, listUTXOsResponse{Count: len(utxos), UTXOs: utxos})
}

// listBlocksResponse is the JSON payload for GET /blocks.
type listBlocksResponse struct {
	Count  int            `json:"count"`
	Blocks []BlockSummary `json:"blocks"`
}

// GET /blocks?limit=N — returns the most recent N blocks (default 50).
func (s *Server) handleListBlocks(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		// best-effort parse; ignore garbage and use the default.
		var v int
		if _, err := jsonNumber(raw, &v); err == nil && v > 0 {
			limit = v
		}
	}
	blocks, err := s.diag.ListBlocks(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if blocks == nil {
		blocks = []BlockSummary{}
	}
	writeJSON(w, http.StatusOK, listBlocksResponse{Count: len(blocks), Blocks: blocks})
}

// jsonNumber is a tiny strconv-free integer parser tolerant of leading
// whitespace; on success it writes the parsed value into out.
func jsonNumber(raw string, out *int) (int, error) {
	n := 0
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c < '0' || c > '9' {
			return i, errors.New("not a digit")
		}
		n = n*10 + int(c-'0')
	}
	*out = n
	return len(raw), nil
}

// ── demo handlers (require s.demo) ───────────────────────────────────────────

// GET /demo/state — one-shot snapshot for the frontend.
func (s *Server) handleDemoState(w http.ResponseWriter, _ *http.Request) {
	state, err := s.demo.DemoState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// GET /demo/wallets — list demo wallets with balances.
func (s *Server) handleDemoListWallets(w http.ResponseWriter, _ *http.Request) {
	wallets, err := s.demo.DemoListWallets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if wallets == nil {
		wallets = []DemoWalletInfo{}
	}
	writeJSON(w, http.StatusOK, wallets)
}

// POST /demo/wallets — create a new demo wallet.
func (s *Server) handleDemoCreateWallet(w http.ResponseWriter, r *http.Request) {
	var req DemoCreateWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	info, err := s.demo.DemoCreateWallet(req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

// POST /demo/tx — build, sign, and submit a transaction.
func (s *Server) handleDemoBuildTx(w http.ResponseWriter, r *http.Request) {
	var req DemoTxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	res, err := s.demo.DemoBuildAndSubmitTx(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// POST /demo/double-spend — submit two conflicting transactions.
func (s *Server) handleDemoDoubleSpend(w http.ResponseWriter, r *http.Request) {
	var req DemoDoubleSpendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	res, err := s.demo.DemoDoubleSpend(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// POST /demo/mine — mine one block; optionally with a specific wallet.
func (s *Server) handleDemoMine(w http.ResponseWriter, r *http.Request) {
	var req DemoMineRequest
	// Body is optional; ignore decode errors when it is empty.
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	}
	res, err := s.demo.DemoMine(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// POST /demo/reset — clear all demo state.
func (s *Server) handleDemoReset(w http.ResponseWriter, _ *http.Request) {
	if err := s.demo.DemoReset(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}
