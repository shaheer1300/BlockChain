package consensus

import (
	"errors"
	"testing"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/crypto"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── test helpers ──────────────────────────────────────────────────────────────

// newKey generates a secp256k1 key or fatals the test.
func newKey(t *testing.T) *crypto.PrivateKey {
	t.Helper()
	k, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return k
}

// populatedView builds a MapUTXOView that contains one UTXO per entry in
// the provided slice. Each entry maps (txID seed byte, output index) →
// (value, address derived from key).
type utxoEntry struct {
	txIDSeed byte
	index    uint32
	value    types.Amount
	key      *crypto.PrivateKey
}

func buildView(entries []utxoEntry) *MapUTXOView {
	view := NewMapUTXOView()
	for _, e := range entries {
		var txID types.Hash32
		txID[0] = e.txIDSeed
		op := types.OutPoint{TxID: txID, Index: e.index}
		addr := crypto.PubKeyToAddress(e.key.PubKey())
		view.AddUTXO(&types.UTXO{
			OutPoint: op,
			Output:   types.TxOutput{Value: e.value, Recipient: addr},
			Height:   1,
		})
	}
	return view
}

// buildSignedTx creates a transaction spending the given entries and signs
// each input with the corresponding key. Returns the transaction and the view.
func buildSignedTx(t *testing.T, entries []utxoEntry, outputValue types.Amount) (*types.Transaction, *MapUTXOView) {
	t.Helper()
	view := buildView(entries)

	inputs := make([]types.TxInput, len(entries))
	for i, e := range entries {
		var txID types.Hash32
		txID[0] = e.txIDSeed
		inputs[i] = types.TxInput{PrevOut: types.OutPoint{TxID: txID, Index: e.index}}
	}

	var recipientKey, _ = crypto.GenerateKey()
	recipient := crypto.PubKeyToAddress(recipientKey.PubKey())
	tx := &types.Transaction{
		Version: 1,
		Inputs:  inputs,
		Outputs: []types.TxOutput{{Value: outputValue, Recipient: recipient}},
	}

	for i, e := range entries {
		sig, err := crypto.SignInput(tx, i, e.key)
		if err != nil {
			t.Fatalf("SignInput %d: %v", i, err)
		}
		tx.Inputs[i].Signature = sig
		tx.Inputs[i].PubKey = e.key.PubKey().SerializeCompressed()
	}
	return tx, view
}

// ── valid transaction ─────────────────────────────────────────────────────────

func TestValidateTx_Valid(t *testing.T) {
	k := newKey(t)
	tx, view := buildSignedTx(t, []utxoEntry{{0x01, 0, 200, k}}, 150)
	res, err := ValidateTx(tx, view)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if res.Fee != 50 {
		t.Errorf("fee = %d, want 50", res.Fee)
	}
}

func TestValidateTx_Valid_MultipleInputs(t *testing.T) {
	k1, k2 := newKey(t), newKey(t)
	entries := []utxoEntry{{0x01, 0, 100, k1}, {0x02, 0, 200, k2}}
	tx, view := buildSignedTx(t, entries, 280)
	res, err := ValidateTx(tx, view)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if res.Fee != 20 {
		t.Errorf("fee = %d, want 20", res.Fee)
	}
}

func TestValidateTx_ZeroFee(t *testing.T) {
	k := newKey(t)
	tx, view := buildSignedTx(t, []utxoEntry{{0x01, 0, 100, k}}, 100)
	res, err := ValidateTx(tx, view)
	if err != nil {
		t.Fatalf("zero-fee tx should be valid: %v", err)
	}
	if res.Fee != 0 {
		t.Errorf("fee = %d, want 0", res.Fee)
	}
}

// ── coinbase rejection ────────────────────────────────────────────────────────

func TestValidateTx_CoinbaseRejected(t *testing.T) {
	var addr types.Address
	cb := &types.Transaction{
		Version: 1,
		Inputs: []types.TxInput{{
			PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex},
		}},
		Outputs: []types.TxOutput{{Value: 50, Recipient: addr}},
	}
	_, err := ValidateTx(cb, NewMapUTXOView())
	if !errors.Is(err, ErrCoinbaseNotAllowed) {
		t.Fatalf("got %v, want ErrCoinbaseNotAllowed", err)
	}
}

// ── structural errors ─────────────────────────────────────────────────────────

func TestValidateTx_NoInputs(t *testing.T) {
	tx := &types.Transaction{
		Version: 1,
		Outputs: []types.TxOutput{{Value: 10}},
	}
	_, err := ValidateTx(tx, NewMapUTXOView())
	if !errors.Is(err, ErrNoInputs) {
		t.Fatalf("got %v, want ErrNoInputs", err)
	}
}

func TestValidateTx_NoOutputs(t *testing.T) {
	var txID types.Hash32
	txID[0] = 1
	tx := &types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: types.OutPoint{TxID: txID, Index: 0}}},
	}
	_, err := ValidateTx(tx, NewMapUTXOView())
	if !errors.Is(err, ErrNoOutputs) {
		t.Fatalf("got %v, want ErrNoOutputs", err)
	}
}

func TestValidateTx_ZeroOutput(t *testing.T) {
	k := newKey(t)
	var txID types.Hash32
	txID[0] = 1
	view := NewMapUTXOView()
	view.AddUTXO(&types.UTXO{
		OutPoint: types.OutPoint{TxID: txID, Index: 0},
		Output:   types.TxOutput{Value: 100, Recipient: crypto.PubKeyToAddress(k.PubKey())},
	})
	tx := &types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: types.OutPoint{TxID: txID, Index: 0}}},
		Outputs: []types.TxOutput{{Value: 0}},
	}
	sig, _ := crypto.SignInput(tx, 0, k)
	tx.Inputs[0].Signature = sig
	tx.Inputs[0].PubKey = k.PubKey().SerializeCompressed()

	_, err := ValidateTx(tx, view)
	if !errors.Is(err, ErrZeroOutput) {
		t.Fatalf("got %v, want ErrZeroOutput", err)
	}
}

// ── duplicate input ───────────────────────────────────────────────────────────

func TestValidateTx_DuplicateInput(t *testing.T) {
	k := newKey(t)
	var txID types.Hash32
	txID[0] = 1
	op := types.OutPoint{TxID: txID, Index: 0}
	view := NewMapUTXOView()
	view.AddUTXO(&types.UTXO{
		OutPoint: op,
		Output:   types.TxOutput{Value: 100, Recipient: crypto.PubKeyToAddress(k.PubKey())},
	})
	tx := &types.Transaction{
		Version: 1,
		Inputs: []types.TxInput{
			{PrevOut: op},
			{PrevOut: op}, // duplicate
		},
		Outputs: []types.TxOutput{{Value: 50}},
	}
	_, err := ValidateTx(tx, view)
	if !errors.Is(err, ErrDuplicateInput) {
		t.Fatalf("got %v, want ErrDuplicateInput", err)
	}
}

// ── missing UTXO ──────────────────────────────────────────────────────────────

func TestValidateTx_MissingUTXO(t *testing.T) {
	k := newKey(t)
	var txID types.Hash32
	txID[0] = 0xFF
	tx := &types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: types.OutPoint{TxID: txID, Index: 0}}},
		Outputs: []types.TxOutput{{Value: 50}},
	}
	sig, _ := crypto.SignInput(tx, 0, k)
	tx.Inputs[0].Signature = sig
	tx.Inputs[0].PubKey = k.PubKey().SerializeCompressed()

	_, err := ValidateTx(tx, NewMapUTXOView()) // empty view
	if !errors.Is(err, ErrMissingUTXO) {
		t.Fatalf("got %v, want ErrMissingUTXO", err)
	}
}

// ── invalid signature ─────────────────────────────────────────────────────────

func TestValidateTx_WrongSignature(t *testing.T) {
	k := newKey(t)
	wrongKey := newKey(t)
	var txID types.Hash32
	txID[0] = 1
	op := types.OutPoint{TxID: txID, Index: 0}
	view := NewMapUTXOView()
	view.AddUTXO(&types.UTXO{
		OutPoint: op,
		Output:   types.TxOutput{Value: 100, Recipient: crypto.PubKeyToAddress(k.PubKey())},
	})
	tx := &types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: op}},
		Outputs: []types.TxOutput{{Value: 80}},
	}
	// Sign with a different key — verification should fail.
	sig, _ := crypto.SignInput(tx, 0, wrongKey)
	tx.Inputs[0].Signature = sig
	tx.Inputs[0].PubKey = wrongKey.PubKey().SerializeCompressed()

	_, err := ValidateTx(tx, view)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("got %v, want ErrInvalidSignature", err)
	}
}

func TestValidateTx_ModifiedAfterSigning(t *testing.T) {
	k := newKey(t)
	tx, view := buildSignedTx(t, []utxoEntry{{0x01, 0, 200, k}}, 150)
	// Mutate the output value after signing — preimage changes, sig invalid.
	tx.Outputs[0].Value = 199
	_, err := ValidateTx(tx, view)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("got %v, want ErrInvalidSignature", err)
	}
}

// ── outputs exceed inputs ─────────────────────────────────────────────────────

func TestValidateTx_OutputsExceedInputs(t *testing.T) {
	k := newKey(t)
	tx, view := buildSignedTx(t, []utxoEntry{{0x01, 0, 100, k}}, 100)
	// Re-sign after bumping output beyond the UTXO value.
	tx.Outputs[0].Value = 101
	sig, _ := crypto.SignInput(tx, 0, k)
	tx.Inputs[0].Signature = sig
	tx.Inputs[0].PubKey = k.PubKey().SerializeCompressed()

	_, err := ValidateTx(tx, view)
	if !errors.Is(err, ErrOutputsExceedInputs) {
		t.Fatalf("got %v, want ErrOutputsExceedInputs", err)
	}
}

// ── overlay UTXO view ─────────────────────────────────────────────────────────

func TestOverlayUTXOView_SpendHidesParent(t *testing.T) {
	k := newKey(t)
	var txID types.Hash32
	txID[0] = 1
	op := types.OutPoint{TxID: txID, Index: 0}

	base := NewMapUTXOView()
	base.AddUTXO(&types.UTXO{OutPoint: op, Output: types.TxOutput{Value: 50, Recipient: crypto.PubKeyToAddress(k.PubKey())}})

	overlay := NewOverlayUTXOView(base)
	overlay.SpendUTXO(op)

	got, err := overlay.GetUTXO(op)
	if err != nil {
		t.Fatalf("GetUTXO: %v", err)
	}
	if got != nil {
		t.Fatal("spent UTXO should return nil from overlay")
	}
}

func TestOverlayUTXOView_AddVisible(t *testing.T) {
	k := newKey(t)
	var txID types.Hash32
	txID[0] = 2
	op := types.OutPoint{TxID: txID, Index: 0}
	utxo := &types.UTXO{OutPoint: op, Output: types.TxOutput{Value: 77, Recipient: crypto.PubKeyToAddress(k.PubKey())}}

	base := NewMapUTXOView()
	overlay := NewOverlayUTXOView(base)
	overlay.AddUTXO(utxo)

	got, err := overlay.GetUTXO(op)
	if err != nil {
		t.Fatalf("GetUTXO: %v", err)
	}
	if got == nil || got.Output.Value != 77 {
		t.Fatalf("expected value 77, got %v", got)
	}
}

func TestOverlayUTXOView_FallsThrough(t *testing.T) {
	k := newKey(t)
	var txID types.Hash32
	txID[0] = 3
	op := types.OutPoint{TxID: txID, Index: 0}
	utxo := &types.UTXO{OutPoint: op, Output: types.TxOutput{Value: 99, Recipient: crypto.PubKeyToAddress(k.PubKey())}}

	base := NewMapUTXOView()
	base.AddUTXO(utxo)
	overlay := NewOverlayUTXOView(base)

	got, err := overlay.GetUTXO(op)
	if err != nil {
		t.Fatalf("GetUTXO: %v", err)
	}
	if got == nil || got.Output.Value != 99 {
		t.Fatalf("expected value 99 from parent, got %v", got)
	}
}
