package mempool_test

import (
	"errors"
	"testing"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/crypto"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/mempool"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── test helpers ──────────────────────────────────────────────────────────────

// mapView is a trivial UTXOView backed by a Go map — exactly what the mempool
// interface requires.
type mapView map[types.OutPoint]*types.UTXO

func (v mapView) GetUTXO(op types.OutPoint) (*types.UTXO, error) {
	return v[op], nil
}

// newKey generates a secp256k1 key pair or fatals the test.
func newKey(t *testing.T) *crypto.PrivateKey {
	t.Helper()
	k, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return k
}

// buildView creates a mapView with a single UTXO keyed by op.
func buildView(op types.OutPoint, value types.Amount, addr types.Address) mapView {
	return mapView{
		op: {OutPoint: op, Output: types.TxOutput{Value: value, Recipient: addr}, Height: 1},
	}
}

// signedTx builds and signs a transaction spending op with key k, paying
// outValue to addr.
func signedTx(t *testing.T, op types.OutPoint, k *crypto.PrivateKey, outValue types.Amount, addr types.Address) *types.Transaction {
	t.Helper()
	tx := &types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: op}},
		Outputs: []types.TxOutput{{Value: outValue, Recipient: addr}},
	}
	sig, err := crypto.SignInput(tx, 0, k)
	if err != nil {
		t.Fatalf("SignInput: %v", err)
	}
	tx.Inputs[0].Signature = sig
	tx.Inputs[0].PubKey = k.PubKey().SerializeCompressed()
	return tx
}

// seedHash returns a Hash32 with byte 0 set to seed.
func seedHash(seed byte) types.Hash32 {
	var h types.Hash32
	h[0] = seed
	return h
}

// ── basic acceptance ──────────────────────────────────────────────────────────

func TestAdd_ValidTxEnterMempool(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(1), Index: 0}
	view := buildView(op, 1000, addr)
	mp := mempool.New(0)

	tx := signedTx(t, op, k, 900, addr)
	if err := mp.Add(tx, view); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if mp.Size() != 1 {
		t.Errorf("Size = %d, want 1", mp.Size())
	}
}

func TestAdd_FeeIsNonZero(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(1), Index: 0}
	view := buildView(op, 1000, addr)
	mp := mempool.New(0)

	tx := signedTx(t, op, k, 900, addr)
	if err := mp.Add(tx, view); err != nil {
		t.Fatalf("Add: %v", err)
	}
	entry := mp.Get(tx.TxID())
	if entry == nil {
		t.Fatal("Get returned nil")
	}
	if entry.Fee != 100 {
		t.Errorf("fee = %d, want 100", entry.Fee)
	}
}

// ── duplicate rejection ───────────────────────────────────────────────────────

func TestAdd_DuplicateTxRejected(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(2), Index: 0}
	view := buildView(op, 500, addr)
	mp := mempool.New(0)

	tx := signedTx(t, op, k, 400, addr)
	if err := mp.Add(tx, view); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	err := mp.Add(tx, view)
	if !errors.Is(err, mempool.ErrDuplicateTx) {
		t.Fatalf("second Add: got %v, want ErrDuplicateTx", err)
	}
	if mp.Size() != 1 {
		t.Errorf("Size = %d after duplicate, want 1", mp.Size())
	}
}

// ── double-spend rejection ────────────────────────────────────────────────────

func TestAdd_DoubleSpendRejected(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(3), Index: 0}
	view := buildView(op, 1000, addr)
	mp := mempool.New(0)

	tx1 := signedTx(t, op, k, 800, addr)
	if err := mp.Add(tx1, view); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	// tx2 spends the same outpoint — must be rejected.
	tx2 := signedTx(t, op, k, 700, addr)
	err := mp.Add(tx2, view)
	if !errors.Is(err, mempool.ErrDoubleSpend) {
		t.Fatalf("second Add: got %v, want ErrDoubleSpend", err)
	}
	if mp.Size() != 1 {
		t.Errorf("Size = %d, want 1", mp.Size())
	}
}

func TestMempool_NeverHasTwoTxsSpendingSameOutPoint(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(4), Index: 0}
	view := buildView(op, 2000, addr)
	mp := mempool.New(0)

	tx1 := signedTx(t, op, k, 1900, addr)
	tx2 := signedTx(t, op, k, 1800, addr) // different outputs → different TxID

	_ = mp.Add(tx1, view)
	_ = mp.Add(tx2, view) // second must fail silently (ErrDoubleSpend)

	// Prove the invariant: only one tx may claim op.
	count := 0
	for _, e := range mp.Entries() {
		for _, in := range e.Tx.Inputs {
			if in.PrevOut == op {
				count++
			}
		}
	}
	if count > 1 {
		t.Errorf("mempool has %d entries spending the same OutPoint, want ≤1", count)
	}
}

// ── min fee policy ────────────────────────────────────────────────────────────

func TestAdd_BelowMinFeeRateRejected(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(5), Index: 0}
	view := buildView(op, 10_000, addr)
	mp := mempool.New(100) // 100 sat/byte minimum

	// Zero-fee transaction must be rejected.
	tx := signedTx(t, op, k, 10_000, addr)
	err := mp.Add(tx, view)
	if !errors.Is(err, mempool.ErrFeeTooLow) {
		t.Fatalf("got %v, want ErrFeeTooLow", err)
	}
}

func TestAdd_ExactlyMinFeeRateAccepted(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(6), Index: 0}
	const inValue = types.Amount(100_000)
	view := buildView(op, inValue, addr)
	mp := mempool.New(1) // 1 sat/byte

	// ECDSA DER signature length varies (70–72 bytes) depending on the
	// high bit of r and s, so the encoded tx size cannot be predicted
	// before signing. Iterate until the fee covers the actual signed
	// size, guaranteeing fee/size ≥ 1 sat/byte.
	fee := uint64(1)
	const maxAttempts = 8
	for attempt := 0; attempt < maxAttempts; attempt++ {
		outValue := inValue - types.Amount(fee)
		tx := signedTx(t, op, k, outValue, addr)
		encoded, err := tx.CanonicalEncode()
		if err != nil {
			t.Fatalf("CanonicalEncode: %v", err)
		}
		size := uint64(len(encoded))
		if fee >= size {
			if err := mp.Add(tx, view); err != nil {
				t.Fatalf("Add with exact min fee rate (fee=%d, size=%d): %v", fee, size, err)
			}
			return
		}
		fee = size
	}
	t.Fatalf("signed tx size did not converge after %d attempts", maxAttempts)
}

// ── RemoveMined ───────────────────────────────────────────────────────────────

func TestRemoveMined_MinedTxIsRemoved(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(7), Index: 0}
	view := buildView(op, 1000, addr)
	mp := mempool.New(0)

	tx := signedTx(t, op, k, 900, addr)
	if err := mp.Add(tx, view); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if mp.Size() != 1 {
		t.Fatalf("Size = %d, want 1", mp.Size())
	}

	// Build a fake block that includes tx.
	cb := types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex}}},
		Outputs: []types.TxOutput{{Value: 50, Recipient: addr}},
	}
	block := &types.Block{Transactions: []types.Transaction{cb, *tx}}

	mp.RemoveMined(block)
	if mp.Size() != 0 {
		t.Errorf("Size = %d after RemoveMined, want 0", mp.Size())
	}
	if mp.HasSpend(op) {
		t.Error("spend index still references spent outpoint after RemoveMined")
	}
}

func TestRemoveMined_SpendIndexCleaned(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(8), Index: 0}
	view := buildView(op, 500, addr)
	mp := mempool.New(0)

	tx := signedTx(t, op, k, 400, addr)
	_ = mp.Add(tx, view)

	cb := types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex}}},
		Outputs: []types.TxOutput{{Value: 50, Recipient: addr}},
	}
	block := &types.Block{Transactions: []types.Transaction{cb, *tx}}
	mp.RemoveMined(block)

	if mp.HasSpend(op) {
		t.Error("HasSpend returned true after the spending tx was mined")
	}
}

// ── RemoveConflicting ─────────────────────────────────────────────────────────

func TestRemoveConflicting_ConflictingTxRemovedAfterBlock(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(9), Index: 0}
	view := buildView(op, 1000, addr)
	mp := mempool.New(0)

	// mempoolTx claims op but is NOT included in the block.
	mempoolTx := signedTx(t, op, k, 800, addr)
	if err := mp.Add(mempoolTx, view); err != nil {
		t.Fatalf("Add mempoolTx: %v", err)
	}

	// A different key also owns an output we can use for the block's spend tx.
	k2 := newKey(t)
	addr2 := crypto.PubKeyToAddress(k2.PubKey())
	confirmedTx := signedTx(t, op, k, 700, addr2) // spends op too

	cb := types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex}}},
		Outputs: []types.TxOutput{{Value: 50, Recipient: addr}},
	}
	block := &types.Block{Transactions: []types.Transaction{cb, *confirmedTx}}

	mp.RemoveConflicting(block)

	if mp.Size() != 0 {
		t.Errorf("Size = %d after RemoveConflicting, want 0", mp.Size())
	}
	if mp.HasSpend(op) {
		t.Error("spend index still references outpoint after RemoveConflicting")
	}
}

func TestRemoveConflicting_NonConflictingTxSurvives(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())

	op1 := types.OutPoint{TxID: seedHash(0xAA), Index: 0}
	op2 := types.OutPoint{TxID: seedHash(0xBB), Index: 0}

	view := mapView{
		op1: {OutPoint: op1, Output: types.TxOutput{Value: 300, Recipient: addr}, Height: 1},
		op2: {OutPoint: op2, Output: types.TxOutput{Value: 200, Recipient: addr}, Height: 1},
	}
	mp := mempool.New(0)

	// tx1 spends op1; tx2 spends op2.
	tx1 := signedTx(t, op1, k, 250, addr)
	tx2 := signedTx(t, op2, k, 150, addr)
	_ = mp.Add(tx1, view)
	_ = mp.Add(tx2, view)

	// Block only confirms op1; op2-spending tx must survive.
	cb := types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex}}},
		Outputs: []types.TxOutput{{Value: 50, Recipient: addr}},
	}
	confirmedTx := signedTx(t, op1, k, 200, addr)
	block := &types.Block{Transactions: []types.Transaction{cb, *confirmedTx}}

	mp.RemoveConflicting(block)

	if mp.Size() != 1 {
		t.Errorf("Size = %d, want 1 (non-conflicting tx should survive)", mp.Size())
	}
	if mp.Get(tx2.TxID()) == nil {
		t.Error("tx2 (spending op2) should still be in mempool")
	}
}

// ── Entries ordering ──────────────────────────────────────────────────────────

func TestEntries_SortedByDescendingFeeRate(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())

	op1 := types.OutPoint{TxID: seedHash(0x11), Index: 0}
	op2 := types.OutPoint{TxID: seedHash(0x22), Index: 0}
	op3 := types.OutPoint{TxID: seedHash(0x33), Index: 0}
	view := mapView{
		op1: {OutPoint: op1, Output: types.TxOutput{Value: 50_000, Recipient: addr}, Height: 1},
		op2: {OutPoint: op2, Output: types.TxOutput{Value: 50_000, Recipient: addr}, Height: 1},
		op3: {OutPoint: op3, Output: types.TxOutput{Value: 50_000, Recipient: addr}, Height: 1},
	}
	mp := mempool.New(0)

	// Deliberately add in low-fee order to test that Entries sorts correctly.
	txLow := signedTx(t, op1, k, 49_990, addr)  // fee 10
	txMid := signedTx(t, op2, k, 49_500, addr)  // fee 500
	txHigh := signedTx(t, op3, k, 45_000, addr) // fee 5000

	_ = mp.Add(txLow, view)
	_ = mp.Add(txMid, view)
	_ = mp.Add(txHigh, view)

	entries := mp.Entries()
	if len(entries) != 3 {
		t.Fatalf("len(Entries) = %d, want 3", len(entries))
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].FeeRate > entries[i-1].FeeRate {
			t.Errorf("entries not sorted by descending fee rate: [%d].FeeRate=%d > [%d].FeeRate=%d",
				i, entries[i].FeeRate, i-1, entries[i-1].FeeRate)
		}
	}
}

// ── HasSpend ──────────────────────────────────────────────────────────────────

func TestHasSpend_TrueWhileInMempool(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(0xCC), Index: 0}
	view := buildView(op, 1000, addr)
	mp := mempool.New(0)

	tx := signedTx(t, op, k, 900, addr)
	_ = mp.Add(tx, view)

	if !mp.HasSpend(op) {
		t.Error("HasSpend should return true after Add")
	}

	cb := types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex}}},
		Outputs: []types.TxOutput{{Value: 50, Recipient: addr}},
	}
	block := &types.Block{Transactions: []types.Transaction{cb, *tx}}
	mp.RemoveMined(block)

	if mp.HasSpend(op) {
		t.Error("HasSpend should return false after RemoveMined")
	}
}

// ── consensus rejection propagates ───────────────────────────────────────────

func TestAdd_ConsensusFailurePropagates(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	op := types.OutPoint{TxID: seedHash(0xDD), Index: 0}
	// Empty view: UTXO does not exist → consensus.ErrMissingUTXO.
	mp := mempool.New(0)
	tx := signedTx(t, op, k, 100, addr)
	err := mp.Add(tx, mapView{})
	if err == nil {
		t.Fatal("expected error from consensus validation, got nil")
	}
}
