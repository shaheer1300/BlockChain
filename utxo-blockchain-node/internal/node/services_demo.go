// Package node — demo service methods that power the educational frontend.
//
// These methods implement api.DemoServices and api.DiagnosticServices.
// They live in a separate file from services.go to keep the production
// API surface clearly separated from the educational additions.
package node

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/api"
	chainfmt "github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/chain"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/storage"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── api.DiagnosticServices ───────────────────────────────────────────────────

// GetAllUTXOs returns every unspent output in the chain.
func (s *nodeServices) GetAllUTXOs() ([]*types.UTXO, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.GetAllUTXOs()
}

// ListBlocks returns the most-recent `limit` blocks (newest first).
func (s *nodeServices) ListBlocks(limit int) ([]api.BlockSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listBlocksNoLock(limit)
}

// listBlocksNoLock is the lock-free body of ListBlocks. Call while holding mu.
func (s *nodeServices) listBlocksNoLock(limit int) ([]api.BlockSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	hashes, err := s.db.ListActiveHashes()
	if err != nil {
		return nil, err
	}
	out := make([]api.BlockSummary, 0, len(hashes))
	start := len(hashes) - 1
	for i := start; i >= 0 && len(out) < limit; i-- {
		h := hashes[i]
		block, err := s.db.GetBlock(h)
		if err != nil {
			return nil, fmt.Errorf("listBlocks: load %s: %w", h, err)
		}
		if block == nil {
			continue
		}
		out = append(out, api.BlockSummary{
			Height:     uint32(i),
			Hash:       h,
			PrevHash:   block.Header.PrevHash,
			MerkleRoot: block.Header.MerkleRoot,
			Timestamp:  block.Header.Timestamp,
			Nonce:      block.Header.Nonce,
			TxCount:    len(block.Transactions),
		})
	}
	return out, nil
}

// ── api.DemoServices ─────────────────────────────────────────────────────────

// DemoState bundles the snapshot the frontend needs after every action.
// It acquires a single RLock and calls all internal helpers directly to
// avoid reentrant lock acquisitions.
func (s *nodeServices) DemoState() (*api.DemoStateResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wallets, err := s.demoListWalletsNoLock()
	if err != nil {
		return nil, err
	}
	utxos, err := s.db.GetAllUTXOs()
	if err != nil {
		return nil, err
	}
	if utxos == nil {
		utxos = []*types.UTXO{}
	}
	blocks, err := s.listBlocksNoLock(100)
	if err != nil {
		return nil, err
	}
	return &api.DemoStateResponse{
		Status:  s.statusNoLock(),
		Wallets: wallets,
		UTXOs:   utxos,
		Mempool: s.mp.Entries(),
		Blocks:  blocks,
	}, nil
}

// DemoListWallets returns the current wallets with computed balances.
func (s *nodeServices) DemoListWallets() ([]api.DemoWalletInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.demoListWalletsNoLock()
}

// demoListWalletsNoLock is the lock-free body. Call while holding mu.
func (s *nodeServices) demoListWalletsNoLock() ([]api.DemoWalletInfo, error) {
	if s.wallets == nil {
		return []api.DemoWalletInfo{}, nil
	}
	all := s.wallets.List()
	out := make([]api.DemoWalletInfo, 0, len(all))
	for _, w := range all {
		bal, err := s.balanceOf(w.Address)
		if err != nil {
			return nil, err
		}
		out = append(out, api.DemoWalletInfo{
			Name:    w.Name,
			Address: w.Address,
			Balance: bal,
		})
	}
	return out, nil
}

// DemoCreateWallet generates a new keypair under `name`.
func (s *nodeServices) DemoCreateWallet(name string) (api.DemoWalletInfo, error) {
	if s.wallets == nil {
		return api.DemoWalletInfo{}, errors.New("demo mode is not enabled")
	}
	w, err := s.wallets.Create(name)
	if err != nil {
		return api.DemoWalletInfo{}, err
	}
	return api.DemoWalletInfo{
		Name:    w.Name,
		Address: w.Address,
		Balance: 0,
	}, nil
}

// DemoBuildAndSubmitTx greedily selects UTXOs owned by FromWallet, builds
// a signed transaction paying ToAddress, and submits it to the mempool.
func (s *nodeServices) DemoBuildAndSubmitTx(req api.DemoTxRequest) (*api.DemoTxResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tx, selected, change, err := s.buildSignedTx(req)
	if err != nil {
		return nil, err
	}
	if err := s.submitTxNoLock(tx); err != nil {
		return nil, fmt.Errorf("submit: %w", err)
	}
	return makeTxResult(tx, selected, req.ToAddress, req.Amount, change), nil
}

// DemoDoubleSpend builds two transactions consuming the same UTXOs and
// submits them in sequence so the UI can show the second being rejected.
func (s *nodeServices) DemoDoubleSpend(req api.DemoDoubleSpendRequest) (*api.DemoDoubleSpendResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.wallets == nil {
		return nil, errors.New("demo mode is not enabled")
	}
	w := s.wallets.Get(req.FromWallet)
	if w == nil {
		return nil, fmt.Errorf("wallet %q not found", req.FromWallet)
	}

	txA, _, _, err := s.buildSignedTx(api.DemoTxRequest{
		FromWallet: req.FromWallet,
		ToAddress:  req.ToAddressA,
		Amount:     req.Amount,
		Fee:        req.Fee,
	})
	if err != nil {
		return nil, fmt.Errorf("build tx A: %w", err)
	}

	txB := &types.Transaction{
		Version: txA.Version,
		Inputs:  make([]types.TxInput, len(txA.Inputs)),
		Outputs: []types.TxOutput{
			{Value: req.Amount, Recipient: req.ToAddressB},
		},
		LockTime: txA.LockTime,
	}
	totalIn := types.Amount(0)
	for i, in := range txA.Inputs {
		txB.Inputs[i] = types.TxInput{
			PrevOut: in.PrevOut,
			PubKey:  w.PubKeyCompressed(),
		}
		u, gerr := s.db.GetUTXO(in.PrevOut)
		if gerr != nil {
			return nil, gerr
		}
		if u == nil {
			return nil, fmt.Errorf("double-spend: utxo %s:%d disappeared", in.PrevOut.TxID, in.PrevOut.Index)
		}
		next, addErr := totalIn.SafeAdd(u.Output.Value)
		if addErr != nil {
			return nil, addErr
		}
		totalIn = next
	}
	spentB, addErr := req.Amount.SafeAdd(req.Fee)
	if addErr != nil {
		return nil, addErr
	}
	if totalIn > spentB {
		txB.Outputs = append(txB.Outputs, types.TxOutput{
			Value:     totalIn - spentB,
			Recipient: w.Address,
		})
	}
	for i := range txB.Inputs {
		sig, sErr := w.SignInput(txB, i)
		if sErr != nil {
			return nil, fmt.Errorf("sign tx B input %d: %w", i, sErr)
		}
		txB.Inputs[i].Signature = sig
	}

	res := &api.DemoDoubleSpendResult{}
	if err := s.submitTxNoLock(txA); err != nil {
		res.First = api.DemoSubmitOutcome{TxID: txA.TxID(), Accepted: false, Reason: err.Error()}
	} else {
		res.First = api.DemoSubmitOutcome{TxID: txA.TxID(), Accepted: true}
	}
	if err := s.submitTxNoLock(txB); err != nil {
		res.Second = api.DemoSubmitOutcome{TxID: txB.TxID(), Accepted: false, Reason: err.Error()}
	} else {
		res.Second = api.DemoSubmitOutcome{TxID: txB.TxID(), Accepted: true}
	}
	return res, nil
}

// DemoMine mines one block; optionally with a specific wallet.
func (s *nodeServices) DemoMine(ctx context.Context, req api.DemoMineRequest) (*api.MineResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.wallets == nil {
		return nil, errors.New("demo mode is not enabled")
	}

	var miner types.Address
	switch {
	case req.MinerWallet != "":
		w := s.wallets.Get(req.MinerWallet)
		if w == nil {
			return nil, fmt.Errorf("wallet %q not found", req.MinerWallet)
		}
		miner = w.Address
	case !s.minerAddr.IsZero():
		miner = s.minerAddr
	default:
		return nil, errors.New("no miner wallet supplied and MINER_ADDRESS unset")
	}

	if s.chain.Tip() == nil {
		if err := s.chain.InitGenesis(ctx, miner, chainfmt.InitialSubsidy, time.Now().Unix()); err != nil {
			return nil, fmt.Errorf("init genesis: %w", err)
		}
		tip := s.chain.Tip()
		return &api.MineResult{Hash: tip.Hash, Height: tip.Height}, nil
	}

	// Temporarily use the resolved miner address for this call only.
	prev := s.minerAddr
	s.minerAddr = miner
	defer func() { s.minerAddr = prev }()
	return s.mineNoLock(ctx)
}

// DemoReset clears in-memory wallets and the mempool. On-disk chain state
// is preserved; use DemoHardReset to wipe it entirely.
func (s *nodeServices) DemoReset() error {
	if s.wallets != nil {
		if err := s.wallets.Reset(); err != nil {
			return err
		}
	}
	s.mp.Clear()
	return nil
}

// DemoHardReset performs a full destructive reset of all demo state:
//
//  1. Clears the mempool and wallet store (in-memory + persisted file).
//  2. Closes the bbolt database.
//  3. Removes the data directory entirely (validated against the configured
//     DataDir — will refuse to act on ".", "/", or empty paths).
//  4. Recreates the data directory and reopens a fresh database.
//  5. Reinitialises the chain manager (tip becomes nil, pre-genesis state).
//  6. Rebuilds the wallet store file at the new path.
//
// The caller should trigger a fresh "Initialise demo" flow after this call.
// The method holds the exclusive write lock for its duration, so all
// concurrent API requests will block until the reset is complete (~50 ms).
func (s *nodeServices) DemoHardReset(ctx context.Context) (*api.DemoStateResponse, error) {
	// Acquire the exclusive write lock — all readers (other handlers) block
	// until the reset is complete.
	s.mu.Lock()
	defer s.mu.Unlock()

	dataDir := filepath.Clean(s.cfg.DataDir)

	// Safety guard: refuse obviously dangerous paths.
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("hard reset: resolve data dir: %w", err)
	}
	if abs == "/" || abs == `\` || abs == filepath.VolumeName(abs)+`\` ||
		abs == "." || dataDir == "" {
		return nil, fmt.Errorf("hard reset: refusing unsafe data dir %q", abs)
	}

	// 1. Clear in-memory state that doesn't need db access.
	s.mp.Clear()
	if s.wallets != nil {
		_ = s.wallets.Reset() // best-effort; file will be deleted anyway
	}

	// 2. Close the database before deleting its file.
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return nil, fmt.Errorf("hard reset: close db: %w", err)
		}
		s.db = nil
	}

	// 3. Remove the entire data directory (chain.db + wallets.json + any
	//    future files). We validated abs above.
	if err := os.RemoveAll(abs); err != nil {
		return nil, fmt.Errorf("hard reset: remove data dir %q: %w", abs, err)
	}

	// 4. Recreate the data directory.
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("hard reset: recreate data dir: %w", err)
	}

	// 5. Reopen a fresh database.
	dbPath := filepath.Join(abs, "chain.db")
	newDB, err := storage.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("hard reset: reopen db: %w", err)
	}

	// 6. Recreate the chain manager (tip will be nil — pre-genesis).
	newChain, err := chainfmt.NewManager(newDB, s.powNibbles)
	if err != nil {
		_ = newDB.Close()
		return nil, fmt.Errorf("hard reset: recreate chain manager: %w", err)
	}

	// 7. Swap in the new instances atomically (within the write lock).
	s.db = newDB
	s.chain = newChain

	// 8. Rebuild the wallet store.
	if s.wallets != nil {
		walletsPath := filepath.Join(abs, "wallets.json")
		ws, wsErr := newWalletStore(walletsPath)
		if wsErr != nil {
			return nil, fmt.Errorf("hard reset: recreate wallet store: %w", wsErr)
		}
		s.wallets = ws
	}

	// Return an empty state snapshot; the frontend will show the
	// "Initialise demo" card and the user can start fresh.
	return &api.DemoStateResponse{
		Status:  s.statusNoLock(),
		Wallets: []api.DemoWalletInfo{},
		UTXOs:   []*types.UTXO{},
		Mempool: []*types.MempoolEntry{},
		Blocks:  []api.BlockSummary{},
	}, nil
}

// ── shared helpers ───────────────────────────────────────────────────────────

// balanceOf sums every UTXO owned by addr. Call while holding mu (RLock or Lock).
func (s *nodeServices) balanceOf(addr types.Address) (types.Amount, error) {
	utxos, err := s.db.GetUTXOsByAddress(addr)
	if err != nil {
		return 0, err
	}
	var total types.Amount
	for _, u := range utxos {
		next, addErr := total.SafeAdd(u.Output.Value)
		if addErr != nil {
			return 0, addErr
		}
		total = next
	}
	return total, nil
}

// buildSignedTx selects UTXOs, constructs and signs a payment.
// Call while holding mu (RLock or Lock).
func (s *nodeServices) buildSignedTx(req api.DemoTxRequest) (*types.Transaction, []*types.UTXO, types.Amount, error) {
	if s.wallets == nil {
		return nil, nil, 0, errors.New("demo mode is not enabled")
	}
	if req.Amount == 0 {
		return nil, nil, 0, errors.New("amount must be > 0")
	}
	w := s.wallets.Get(req.FromWallet)
	if w == nil {
		return nil, nil, 0, fmt.Errorf("wallet %q not found", req.FromWallet)
	}
	if req.ToAddress.IsZero() {
		return nil, nil, 0, errors.New("to_address is required")
	}

	utxos, err := s.db.GetUTXOsByAddress(w.Address)
	if err != nil {
		return nil, nil, 0, err
	}
	sort.Slice(utxos, func(i, j int) bool { return utxos[i].Output.Value > utxos[j].Output.Value })

	target, addErr := req.Amount.SafeAdd(req.Fee)
	if addErr != nil {
		return nil, nil, 0, addErr
	}

	var selected []*types.UTXO
	var collected types.Amount
	for _, u := range utxos {
		selected = append(selected, u)
		next, sErr := collected.SafeAdd(u.Output.Value)
		if sErr != nil {
			return nil, nil, 0, sErr
		}
		collected = next
		if collected >= target {
			break
		}
	}
	if collected < target {
		return nil, nil, 0, fmt.Errorf("insufficient funds: have %d, need %d (amount %d + fee %d)",
			collected, target, req.Amount, req.Fee)
	}

	tx := &types.Transaction{
		Version: 1,
		Inputs:  make([]types.TxInput, len(selected)),
		Outputs: []types.TxOutput{{Value: req.Amount, Recipient: req.ToAddress}},
	}
	for i, u := range selected {
		tx.Inputs[i] = types.TxInput{
			PrevOut: u.OutPoint,
			PubKey:  w.PubKeyCompressed(),
		}
	}
	change := collected - target
	if change > 0 {
		tx.Outputs = append(tx.Outputs, types.TxOutput{Value: change, Recipient: w.Address})
	}
	for i := range tx.Inputs {
		sig, sErr := w.SignInput(tx, i)
		if sErr != nil {
			return nil, nil, 0, fmt.Errorf("sign input %d: %w", i, sErr)
		}
		tx.Inputs[i].Signature = sig
	}
	return tx, selected, change, nil
}

// makeTxResult builds the educational summary returned to the frontend.
func makeTxResult(tx *types.Transaction, selected []*types.UTXO, to types.Address, amount, change types.Amount) *api.DemoTxResult {
	res := &api.DemoTxResult{
		TxID:    tx.TxID(),
		Inputs:  make([]api.DemoInput, 0, len(selected)),
		Outputs: make([]api.DemoOutput, 0, len(tx.Outputs)),
	}
	for _, u := range selected {
		res.Inputs = append(res.Inputs, api.DemoInput{
			TxID:  u.OutPoint.TxID,
			Index: u.OutPoint.Index,
			Value: u.Output.Value,
			Owner: u.Output.Recipient,
		})
	}
	res.Outputs = append(res.Outputs, api.DemoOutput{
		Value: amount, Recipient: to, Purpose: "payment",
	})
	if change > 0 {
		res.Outputs = append(res.Outputs, api.DemoOutput{
			Value: change, Recipient: tx.Outputs[len(tx.Outputs)-1].Recipient, Purpose: "change",
		})
	}
	return res
}