package consensus

import (
	"errors"
	"testing"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/crypto"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── test constants ────────────────────────────────────────────────────────────

const (
	testSubsidy = types.Amount(50_000_000) // 50 coins (satoshi-like units)
	testHeight  = uint32(1)
)

// ── test helpers ──────────────────────────────────────────────────────────────

// newCoinbaseTx creates a minimal coinbase transaction paying value to addr.
func newCoinbaseTx(addr types.Address, value types.Amount) types.Transaction {
	return types.Transaction{
		Version: 1,
		Inputs: []types.TxInput{{
			PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex},
		}},
		Outputs: []types.TxOutput{{Value: value, Recipient: addr}},
	}
}

// buildBlock assembles a *Block, computing the Merkle root from txs and
// setting it in header before returning.
func buildBlock(t *testing.T, header types.BlockHeader, txs []types.Transaction) *types.Block {
	t.Helper()
	leaves := make([]types.Hash32, len(txs))
	for i := range txs {
		leaves[i] = txs[i].TxID()
	}
	header.MerkleRoot = crypto.MerkleRoot(leaves)
	return &types.Block{Header: header, Transactions: txs}
}

// makeHeader returns a BlockHeader that correctly extends parent at the given
// Unix timestamp. The MerkleRoot is left zero — buildBlock fills it in.
func makeHeader(parent *types.BlockHeader, ts int64) types.BlockHeader {
	var prevHash types.Hash32
	if parent != nil {
		prevHash = parent.BlockHash()
	}
	return types.BlockHeader{
		Version:   1,
		PrevHash:  prevHash,
		Timestamp: ts,
	}
}

// signedSpendTx creates a transaction that spends op (owned by key k), with a
// single output of outVal to k's address.
func signedSpendTx(t *testing.T, op types.OutPoint, k *crypto.PrivateKey, outVal types.Amount) types.Transaction {
	t.Helper()
	addr := crypto.PubKeyToAddress(k.PubKey())
	tx := &types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: op}},
		Outputs: []types.TxOutput{{Value: outVal, Recipient: addr}},
	}
	sig, err := crypto.SignInput(tx, 0, k)
	if err != nil {
		t.Fatalf("SignInput: %v", err)
	}
	tx.Inputs[0].Signature = sig
	tx.Inputs[0].PubKey = k.PubKey().SerializeCompressed()
	return *tx
}

// ── valid blocks ──────────────────────────────────────────────────────────────

func TestValidateBlock_Valid_CoinbaseOnly(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}

	coinbase := newCoinbaseTx(addr, testSubsidy)
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{coinbase})

	res, err := ValidateBlock(block, parent, NewMapUTXOView(), testHeight, testSubsidy, 0, now)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if res.TotalFees != 0 {
		t.Errorf("fees = %d, want 0", res.TotalFees)
	}
}

func TestValidateBlock_Valid_WithRegularTx(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()

	// UTXO in the base state (value 100, fee = 10 after spending)
	var prevTxID types.Hash32
	prevTxID[0] = 0xAA
	op := types.OutPoint{TxID: prevTxID, Index: 0}
	baseView := NewMapUTXOView()
	baseView.AddUTXO(&types.UTXO{
		OutPoint: op,
		Output:   types.TxOutput{Value: 100, Recipient: addr},
		Height:   0,
	})

	spendTx := signedSpendTx(t, op, k, 90) // fee = 10
	coinbase := newCoinbaseTx(addr, testSubsidy+10)

	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{coinbase, spendTx})

	res, err := ValidateBlock(block, parent, baseView, testHeight, testSubsidy, 0, now)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if res.TotalFees != 10 {
		t.Errorf("fees = %d, want 10", res.TotalFees)
	}
}

func TestValidateBlock_Valid_ZeroFee(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()

	var prevTxID types.Hash32
	prevTxID[0] = 0xBB
	op := types.OutPoint{TxID: prevTxID, Index: 0}
	baseView := NewMapUTXOView()
	baseView.AddUTXO(&types.UTXO{
		OutPoint: op,
		Output:   types.TxOutput{Value: 50, Recipient: addr},
		Height:   0,
	})

	spendTx := signedSpendTx(t, op, k, 50) // zero fee
	coinbase := newCoinbaseTx(addr, testSubsidy)

	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{coinbase, spendTx})

	res, err := ValidateBlock(block, parent, baseView, testHeight, testSubsidy, 0, now)
	if err != nil {
		t.Fatalf("expected nil error for zero-fee tx, got: %v", err)
	}
	if res.TotalFees != 0 {
		t.Errorf("fees = %d, want 0", res.TotalFees)
	}
}

// ── genesis (nil parent) ──────────────────────────────────────────────────────

func TestValidateBlock_Valid_NilParentGenesis(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()

	coinbase := newCoinbaseTx(addr, testSubsidy)
	header := makeHeader(nil, now) // PrevHash stays ZeroHash
	block := buildBlock(t, header, []types.Transaction{coinbase})

	_, err := ValidateBlock(block, nil, NewMapUTXOView(), 0, testSubsidy, 0, now)
	if err != nil {
		t.Fatalf("genesis block with nil parent should pass: %v", err)
	}
}

// ── Merkle root ───────────────────────────────────────────────────────────────

func TestValidateBlock_BadMerkleRoot(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}

	coinbase := newCoinbaseTx(addr, testSubsidy)
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{coinbase})
	block.Header.MerkleRoot[0] ^= 0xFF // corrupt after MerkleRoot is set

	_, err := ValidateBlock(block, parent, NewMapUTXOView(), testHeight, testSubsidy, 0, now)
	if !errors.Is(err, ErrBadMerkleRoot) {
		t.Fatalf("got %v, want ErrBadMerkleRoot", err)
	}
}

// ── proof-of-work ─────────────────────────────────────────────────────────────

func TestValidateBlock_BadPoW(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}

	coinbase := newCoinbaseTx(addr, testSubsidy)
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{coinbase})

	// Require all 64 nibbles to be zero — effectively impossible without mining.
	_, err := ValidateBlock(block, parent, NewMapUTXOView(), testHeight, testSubsidy, 64, now)
	if !errors.Is(err, ErrBadPoW) {
		t.Fatalf("got %v, want ErrBadPoW", err)
	}
}

// ── coinbase rules ────────────────────────────────────────────────────────────

func TestValidateBlock_NoTransactions(t *testing.T) {
	now := time.Now().Unix()
	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}
	header := makeHeader(parent, now)
	// Empty Transactions; MerkleRoot stays ZeroHash.
	block := &types.Block{Header: header, Transactions: nil}

	_, err := ValidateBlock(block, parent, NewMapUTXOView(), testHeight, testSubsidy, 0, now)
	if !errors.Is(err, ErrNoTransactions) {
		t.Fatalf("got %v, want ErrNoTransactions", err)
	}
}

func TestValidateBlock_NoCoinbase_FirstTxIsRegular(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}

	var txID types.Hash32
	txID[0] = 1
	notCoinbase := types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: types.OutPoint{TxID: txID, Index: 0}}},
		Outputs: []types.TxOutput{{Value: 10, Recipient: addr}},
	}
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{notCoinbase})

	_, err := ValidateBlock(block, parent, NewMapUTXOView(), testHeight, testSubsidy, 0, now)
	if !errors.Is(err, ErrNoCoinbase) {
		t.Fatalf("got %v, want ErrNoCoinbase", err)
	}
}

func TestValidateBlock_MultipleCoinbases(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}

	cb1 := newCoinbaseTx(addr, testSubsidy/2)
	cb2 := newCoinbaseTx(addr, testSubsidy/2) // second coinbase — invalid
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{cb1, cb2})

	_, err := ValidateBlock(block, parent, NewMapUTXOView(), testHeight, testSubsidy, 0, now)
	if !errors.Is(err, ErrMultipleCoinbases) {
		t.Fatalf("got %v, want ErrMultipleCoinbases", err)
	}
}

func TestValidateBlock_CoinbaseOverpay(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}

	// Overpay by 1 satoshi with no transaction fees.
	coinbase := newCoinbaseTx(addr, testSubsidy+1)
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{coinbase})

	_, err := ValidateBlock(block, parent, NewMapUTXOView(), testHeight, testSubsidy, 0, now)
	if !errors.Is(err, ErrCoinbaseOverpay) {
		t.Fatalf("got %v, want ErrCoinbaseOverpay", err)
	}
}

func TestValidateBlock_CoinbaseExactlySubsidyPlusFees(t *testing.T) {
	// Coinbase claiming exactly subsidy+fees should succeed.
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()

	var prevTxID types.Hash32
	prevTxID[0] = 0xCC
	op := types.OutPoint{TxID: prevTxID, Index: 0}
	baseView := NewMapUTXOView()
	baseView.AddUTXO(&types.UTXO{
		OutPoint: op,
		Output:   types.TxOutput{Value: 200, Recipient: addr},
		Height:   0,
	})

	spendTx := signedSpendTx(t, op, k, 180) // fee = 20
	coinbase := newCoinbaseTx(addr, testSubsidy+20)

	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{coinbase, spendTx})

	_, err := ValidateBlock(block, parent, baseView, testHeight, testSubsidy, 0, now)
	if err != nil {
		t.Fatalf("coinbase claiming exactly subsidy+fees should pass: %v", err)
	}
}

// ── double-spend ──────────────────────────────────────────────────────────────

func TestValidateBlock_DoubleSpend(t *testing.T) {
	// Two transactions in the same block both spend the same UTXO.
	// After the first is applied to the overlay, the second will find
	// the UTXO missing → ErrMissingUTXO propagated from ValidateTx.
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()

	var prevTxID types.Hash32
	prevTxID[0] = 0xDD
	op := types.OutPoint{TxID: prevTxID, Index: 0}
	baseView := NewMapUTXOView()
	baseView.AddUTXO(&types.UTXO{
		OutPoint: op,
		Output:   types.TxOutput{Value: 100, Recipient: addr},
		Height:   0,
	})

	tx1 := signedSpendTx(t, op, k, 80) // output 80, fee 20
	tx2 := signedSpendTx(t, op, k, 70) // output 70, fee 30 — double-spend

	coinbase := newCoinbaseTx(addr, testSubsidy)
	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{coinbase, tx1, tx2})

	_, err := ValidateBlock(block, parent, baseView, testHeight, testSubsidy, 0, now)
	if err == nil {
		t.Fatal("double-spend block should be rejected, got nil error")
	}
	// The error wraps ErrMissingUTXO because tx2's input is spent by tx1.
	if !errors.Is(err, ErrMissingUTXO) {
		t.Errorf("expected ErrMissingUTXO (via double-spend), got: %v", err)
	}
}

// ── prev hash ─────────────────────────────────────────────────────────────────

func TestValidateBlock_BadPrevHash(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}

	coinbase := newCoinbaseTx(addr, testSubsidy)
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{coinbase})
	block.Header.PrevHash[0] ^= 0xFF // corrupt after MerkleRoot is computed

	_, err := ValidateBlock(block, parent, NewMapUTXOView(), testHeight, testSubsidy, 0, now)
	if !errors.Is(err, ErrBadPrevHash) {
		t.Fatalf("got %v, want ErrBadPrevHash", err)
	}
}

// ── timestamps ───────────────────────────────────────────────────────────────

func TestValidateBlock_BadTimestamp_NotGreaterThanParent(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	parent := &types.BlockHeader{Version: 1, Timestamp: now}

	coinbase := newCoinbaseTx(addr, testSubsidy)
	// Timestamp equal to parent's is rejected (must be strictly greater).
	header := makeHeader(parent, now) // Timestamp == parent.Timestamp
	block := buildBlock(t, header, []types.Transaction{coinbase})

	_, err := ValidateBlock(block, parent, NewMapUTXOView(), testHeight, testSubsidy, 0, now)
	if !errors.Is(err, ErrBadTimestamp) {
		t.Fatalf("got %v, want ErrBadTimestamp", err)
	}
}

func TestValidateBlock_BadTimestamp_TooFarInFuture(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}

	coinbase := newCoinbaseTx(addr, testSubsidy)
	// One second past the 2-hour tolerance.
	header := makeHeader(parent, now+maxFutureSeconds+1)
	block := buildBlock(t, header, []types.Transaction{coinbase})

	_, err := ValidateBlock(block, parent, NewMapUTXOView(), testHeight, testSubsidy, 0, now)
	if !errors.Is(err, ErrBadTimestamp) {
		t.Fatalf("got %v, want ErrBadTimestamp", err)
	}
}

func TestValidateBlock_TimestampExactlyAtFutureLimit(t *testing.T) {
	// Timestamp == now + maxFutureSeconds must be accepted (≤ is OK).
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}

	coinbase := newCoinbaseTx(addr, testSubsidy)
	header := makeHeader(parent, now+maxFutureSeconds) // right at boundary
	block := buildBlock(t, header, []types.Transaction{coinbase})

	_, err := ValidateBlock(block, parent, NewMapUTXOView(), testHeight, testSubsidy, 0, now)
	if err != nil {
		t.Fatalf("timestamp at exact future limit should pass: %v", err)
	}
}

// ── intra-block UTXO chaining ─────────────────────────────────────────────────

func TestValidateBlock_IntraBlockChaining(t *testing.T) {
	// tx1 creates an output; tx2 in the same block spends that new output.
	// This requires the overlay to expose tx1's outputs to tx2.
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()

	var prevTxID types.Hash32
	prevTxID[0] = 0xEE
	op := types.OutPoint{TxID: prevTxID, Index: 0}
	baseView := NewMapUTXOView()
	baseView.AddUTXO(&types.UTXO{
		OutPoint: op,
		Output:   types.TxOutput{Value: 100, Recipient: addr},
		Height:   0,
	})

	// tx1: spend op, create output (value 90, fee 10)
	tx1 := signedSpendTx(t, op, k, 90)
	tx1ID := tx1.TxID()
	op2 := types.OutPoint{TxID: tx1ID, Index: 0} // output of tx1

	// tx2: spend tx1's output (value 90), create output (value 80, fee 10)
	tx2 := signedSpendTx(t, op2, k, 80)

	coinbase := newCoinbaseTx(addr, testSubsidy+20) // fees = 10+10=20

	parent := &types.BlockHeader{Version: 1, Timestamp: now - 100}
	block := buildBlock(t, makeHeader(parent, now), []types.Transaction{coinbase, tx1, tx2})

	res, err := ValidateBlock(block, parent, baseView, testHeight, testSubsidy, 0, now)
	if err != nil {
		t.Fatalf("intra-block chaining should pass: %v", err)
	}
	if res.TotalFees != 20 {
		t.Errorf("fees = %d, want 20", res.TotalFees)
	}
}
