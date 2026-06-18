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
	"sort"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/api"
	chainfmt "github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/chain"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── api.DiagnosticServices ───────────────────────────────────────────────────

// GetAllUTXOs returns every unspent output in the chain.
func (s *nodeServices) GetAllUTXOs() ([]*types.UTXO, error) {
	return s.db.GetAllUTXOs()
}

// ListBlocks returns the most-recent `limit` blocks (newest first). When
// `limit` is non-positive the configured default of 50 is used.
func (s *nodeServices) ListBlocks(limit int) ([]api.BlockSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	hashes, err := s.db.ListActiveHashes()
	if err != nil {
		return nil, err
	}
	// Iterate from the tip backwards so callers always see the freshest
	// blocks first.
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
func (s *nodeServices) DemoState() (*api.DemoStateResponse, error) {
	wallets, err := s.DemoListWallets()
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
	blocks, err := s.ListBlocks(100)
	if err != nil {
		return nil, err
	}
	return &api.DemoStateResponse{
		Status:  s.Status(),
		Wallets: wallets,
		UTXOs:   utxos,
		Mempool: s.mp.Entries(),
		Blocks:  blocks,
	}, nil
}

// DemoListWallets returns the current wallets with computed balances.
func (s *nodeServices) DemoListWallets() ([]api.DemoWalletInfo, error) {
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
	tx, selected, change, err := s.buildSignedTx(req)
	if err != nil {
		return nil, err
	}
	if err := s.SubmitTx(tx); err != nil {
		return nil, fmt.Errorf("submit: %w", err)
	}
	return makeTxResult(tx, selected, req.ToAddress, req.Amount, change), nil
}

// DemoDoubleSpend builds two transactions consuming the same UTXOs and
// submits them in sequence so the UI can show the second being rejected.
func (s *nodeServices) DemoDoubleSpend(req api.DemoDoubleSpendRequest) (*api.DemoDoubleSpendResult, error) {
	if s.wallets == nil {
		return nil, errors.New("demo mode is not enabled")
	}
	w := s.wallets.Get(req.FromWallet)
	if w == nil {
		return nil, fmt.Errorf("wallet %q not found", req.FromWallet)
	}

	// Build tx A
	txA, _, _, err := s.buildSignedTx(api.DemoTxRequest{
		FromWallet: req.FromWallet,
		ToAddress:  req.ToAddressA,
		Amount:     req.Amount,
		Fee:        req.Fee,
	})
	if err != nil {
		return nil, fmt.Errorf("build tx A: %w", err)
	}
	// Build tx B reusing the SAME UTXO inputs as tx A but paying ToAddressB.
	// We do this by manually constructing a parallel transaction that
	// references identical PrevOuts, then signing each input with w.
	txB := &types.Transaction{
		Version: txA.Version,
		Inputs:  make([]types.TxInput, len(txA.Inputs)),
		Outputs: []types.TxOutput{
			{Value: req.Amount, Recipient: req.ToAddressB},
		},
		LockTime: txA.LockTime,
	}
	// Recompute change for B's output structure
	totalIn := types.Amount(0)
	for i, in := range txA.Inputs {
		txB.Inputs[i] = types.TxInput{
			PrevOut: in.PrevOut,
			PubKey:  w.PubKeyCompressed(),
		}
		// look up value
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
	// Submit A first.
	if err := s.SubmitTx(txA); err != nil {
		res.First = api.DemoSubmitOutcome{TxID: txA.TxID(), Accepted: false, Reason: err.Error()}
	} else {
		res.First = api.DemoSubmitOutcome{TxID: txA.TxID(), Accepted: true}
	}
	// Submit B second — expected to be rejected as a conflict.
	if err := s.SubmitTx(txB); err != nil {
		res.Second = api.DemoSubmitOutcome{TxID: txB.TxID(), Accepted: false, Reason: err.Error()}
	} else {
		res.Second = api.DemoSubmitOutcome{TxID: txB.TxID(), Accepted: true}
	}
	return res, nil
}

// DemoMine mines one block; optionally with a specific wallet. When the
// chain has not been initialised this call performs InitGenesis using
// MinerWallet as the genesis miner.
func (s *nodeServices) DemoMine(ctx context.Context, req api.DemoMineRequest) (*api.MineResult, error) {
	if s.wallets == nil {
		return nil, errors.New("demo mode is not enabled")
	}

	// Resolve the miner address: explicit wallet > configured MINER_ADDRESS.
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

	// If the chain hasn't been initialised, do it now using the resolved
	// miner address. After this we fall through to the normal mining path
	// (which mines block 1).
	if s.chain.Tip() == nil {
		if err := s.chain.InitGenesis(ctx, miner, chainfmt.InitialSubsidy, time.Now().Unix()); err != nil {
			return nil, fmt.Errorf("init genesis: %w", err)
		}
		tip := s.chain.Tip()
		return &api.MineResult{Hash: tip.Hash, Height: tip.Height}, nil
	}

	// Mine using the resolved miner. Temporarily swap minerAddr; this is
	// safe because Mine reads s.minerAddr synchronously and the demo is
	// single-user. We do NOT persist the swap.
	prev := s.minerAddr
	s.minerAddr = miner
	defer func() { s.minerAddr = prev }()
	return s.Mine(ctx)
}

// DemoReset clears the mempool and wallets. Chain state on disk is
// intentionally left intact — restart the node to fully wipe state.
func (s *nodeServices) DemoReset() error {
	if s.wallets != nil {
		if err := s.wallets.Reset(); err != nil {
			return err
		}
	}
	s.mp.Clear()
	return nil
}

// ── shared helpers ───────────────────────────────────────────────────────────

// balanceOf sums every UTXO owned by addr.
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

// buildSignedTx selects UTXOs, constructs and signs a payment from
// req.FromWallet to req.ToAddress paying req.Amount with req.Fee.
// Returns the signed tx, the list of selected UTXOs, and the change
// amount paid back to the sender (0 if no change output was created).
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
	// Greedy selection: largest-value UTXOs first to minimise input count.
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
