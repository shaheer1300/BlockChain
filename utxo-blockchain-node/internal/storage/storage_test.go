package storage

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func sampleHash(seed byte) types.Hash32 {
	var h types.Hash32
	for i := range h {
		h[i] = seed + byte(i)
	}
	return h
}

func sampleAddress(seed byte) types.Address {
	var a types.Address
	for i := range a {
		a[i] = seed + byte(i)
	}
	return a
}

func sampleHeader() types.BlockHeader {
	return types.BlockHeader{
		Version:    1,
		PrevHash:   sampleHash(1),
		MerkleRoot: sampleHash(50),
		Timestamp:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		Bits:       0x1d00ffff,
		Nonce:      42,
	}
}

func sampleBlock() *types.Block {
	header := sampleHeader()
	var addr types.Address
	addr[0] = 0xFF
	cb := types.Transaction{
		Version: 1,
		Inputs: []types.TxInput{{
			PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex},
		}},
		Outputs: []types.TxOutput{{Value: 50, Recipient: addr}},
	}
	// Set the Merkle root to the single coinbase TxID.
	header.MerkleRoot = cb.TxID()
	return &types.Block{Header: header, Transactions: []types.Transaction{cb}}
}

func sampleUTXO() *types.UTXO {
	return &types.UTXO{
		OutPoint: types.OutPoint{TxID: sampleHash(10), Index: 0},
		Output:   types.TxOutput{Value: 100, Recipient: sampleAddress(20)},
		Height:   5,
		Coinbase: false,
	}
}

func sampleBlockIndex() *types.BlockIndex {
	h := sampleHeader()
	hash := h.BlockHash()
	return &types.BlockIndex{
		Hash:      hash,
		Header:    h,
		Height:    3,
		TotalWork: big.NewInt(1_000_000),
		Status:    types.BlockStatusValid,
	}
}

// ── block tests ───────────────────────────────────────────────────────────────

func TestSaveGetBlock(t *testing.T) {
	db := tempDB(t)
	block := sampleBlock()
	if err := db.SaveBlock(block); err != nil {
		t.Fatalf("SaveBlock: %v", err)
	}
	got, err := db.GetBlock(block.BlockHash())
	if err != nil {
		t.Fatalf("GetBlock: %v", err)
	}
	if got == nil {
		t.Fatal("GetBlock returned nil")
	}
	if got.BlockHash() != block.BlockHash() {
		t.Errorf("hash mismatch: got %s, want %s", got.BlockHash(), block.BlockHash())
	}
}

func TestGetBlock_NotFound(t *testing.T) {
	db := tempDB(t)
	got, err := db.GetBlock(sampleHash(99))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for unknown hash")
	}
}

// ── header tests ──────────────────────────────────────────────────────────────

func TestSaveGetHeader(t *testing.T) {
	db := tempDB(t)
	h := sampleHeader()
	if err := db.SaveHeader(&h); err != nil {
		t.Fatalf("SaveHeader: %v", err)
	}
	got, err := db.GetHeader(h.BlockHash())
	if err != nil {
		t.Fatalf("GetHeader: %v", err)
	}
	if got == nil {
		t.Fatal("GetHeader returned nil")
	}
	if got.BlockHash() != h.BlockHash() {
		t.Errorf("hash mismatch")
	}
}

// ── UTXO tests ────────────────────────────────────────────────────────────────

func TestPutGetDeleteUTXO(t *testing.T) {
	db := tempDB(t)
	utxo := sampleUTXO()

	if err := db.PutUTXO(utxo); err != nil {
		t.Fatalf("PutUTXO: %v", err)
	}

	got, err := db.GetUTXO(utxo.OutPoint)
	if err != nil {
		t.Fatalf("GetUTXO: %v", err)
	}
	if got == nil {
		t.Fatal("GetUTXO returned nil after put")
	}
	if got.Output.Value != utxo.Output.Value {
		t.Errorf("value mismatch: got %d, want %d", got.Output.Value, utxo.Output.Value)
	}

	if err := db.DeleteUTXO(utxo.OutPoint); err != nil {
		t.Fatalf("DeleteUTXO: %v", err)
	}

	deleted, err := db.GetUTXO(utxo.OutPoint)
	if err != nil {
		t.Fatalf("GetUTXO after delete: %v", err)
	}
	if deleted != nil {
		t.Fatal("UTXO still present after delete")
	}
}

func TestDeleteUTXO_NotFound(t *testing.T) {
	db := tempDB(t)
	// Deleting a non-existent outpoint must not return an error.
	if err := db.DeleteUTXO(types.OutPoint{TxID: sampleHash(77), Index: 9}); err != nil {
		t.Fatalf("DeleteUTXO on absent key: %v", err)
	}
}

// ── block index tests ─────────────────────────────────────────────────────────

func TestSaveGetBlockIndex(t *testing.T) {
	db := tempDB(t)
	idx := sampleBlockIndex()

	if err := db.SaveBlockIndex(idx); err != nil {
		t.Fatalf("SaveBlockIndex: %v", err)
	}
	got, err := db.GetBlockIndex(idx.Hash)
	if err != nil {
		t.Fatalf("GetBlockIndex: %v", err)
	}
	if got == nil {
		t.Fatal("GetBlockIndex returned nil")
	}
	if got.Height != idx.Height {
		t.Errorf("height mismatch: got %d, want %d", got.Height, idx.Height)
	}
	// Verify *big.Int survives JSON round-trip.
	if got.TotalWork.Cmp(idx.TotalWork) != 0 {
		t.Errorf("TotalWork mismatch: got %s, want %s", got.TotalWork, idx.TotalWork)
	}
	if got.Status != idx.Status {
		t.Errorf("Status mismatch: got %d, want %d", got.Status, idx.Status)
	}
}

// ── active chain tests ────────────────────────────────────────────────────────

func TestSetGetActiveHash(t *testing.T) {
	db := tempDB(t)
	hash := sampleHash(33)

	if err := db.SetActiveHash(0, hash); err != nil {
		t.Fatalf("SetActiveHash: %v", err)
	}
	got, found, err := db.GetActiveHash(0)
	if err != nil {
		t.Fatalf("GetActiveHash: %v", err)
	}
	if !found {
		t.Fatal("GetActiveHash: not found")
	}
	if got != hash {
		t.Errorf("hash mismatch: got %s, want %s", got, hash)
	}
}

func TestGetActiveHash_NotFound(t *testing.T) {
	db := tempDB(t)
	got, found, err := db.GetActiveHash(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected not-found for unknown height")
	}
	if got != types.ZeroHash {
		t.Errorf("expected ZeroHash, got %s", got)
	}
}

// ── best tip tests ────────────────────────────────────────────────────────────

func TestSetGetBestTip(t *testing.T) {
	db := tempDB(t)
	tip := &types.ChainTip{
		Hash:      sampleHash(5),
		Height:    10,
		TotalWork: big.NewInt(9_999_999),
	}

	if err := db.SetBestTip(tip); err != nil {
		t.Fatalf("SetBestTip: %v", err)
	}
	got, err := db.GetBestTip()
	if err != nil {
		t.Fatalf("GetBestTip: %v", err)
	}
	if got == nil {
		t.Fatal("GetBestTip returned nil")
	}
	if got.Height != tip.Height {
		t.Errorf("height mismatch: got %d, want %d", got.Height, tip.Height)
	}
	if got.TotalWork.Cmp(tip.TotalWork) != 0 {
		t.Errorf("TotalWork mismatch: got %s, want %s", got.TotalWork, tip.TotalWork)
	}
}

func TestGetBestTip_NotFound(t *testing.T) {
	db := tempDB(t)
	got, err := db.GetBestTip()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil on fresh DB")
	}
}

// ── undo tests ────────────────────────────────────────────────────────────────

func TestSaveGetUndo(t *testing.T) {
	db := tempDB(t)
	blockHash := sampleHash(7)
	undo := &types.BlockUndo{
		BlockHash: blockHash,
		Spent: []types.SpentOutput{
			{
				OutPoint: types.OutPoint{TxID: sampleHash(11), Index: 0},
				Output:   types.TxOutput{Value: 75, Recipient: sampleAddress(30)},
				Height:   2,
				Coinbase: false,
			},
		},
	}

	if err := db.SaveUndo(undo); err != nil {
		t.Fatalf("SaveUndo: %v", err)
	}
	got, err := db.GetUndo(blockHash)
	if err != nil {
		t.Fatalf("GetUndo: %v", err)
	}
	if got == nil {
		t.Fatal("GetUndo returned nil")
	}
	if len(got.Spent) != 1 {
		t.Fatalf("Spent length: got %d, want 1", len(got.Spent))
	}
	if got.Spent[0].Output.Value != 75 {
		t.Errorf("spent value mismatch: got %d, want 75", got.Spent[0].Output.Value)
	}
}

func TestGetUndo_NotFound(t *testing.T) {
	db := tempDB(t)
	got, err := db.GetUndo(sampleHash(88))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for unknown hash")
	}
}

// ── atomic update tests ───────────────────────────────────────────────────────

func TestUpdate_Atomic_AllSucceed(t *testing.T) {
	// Write block + UTXO + undo in one transaction; all must persist.
	db := tempDB(t)
	block := sampleBlock()
	utxo := sampleUTXO()
	undo := &types.BlockUndo{
		BlockHash: block.BlockHash(),
		Spent: []types.SpentOutput{{
			OutPoint: utxo.OutPoint,
			Output:   utxo.Output,
			Height:   utxo.Height,
		}},
	}

	err := db.Update(func(tx *WriteTx) error {
		if err := tx.SaveBlock(block); err != nil {
			return err
		}
		if err := tx.PutUTXO(utxo); err != nil {
			return err
		}
		return tx.SaveUndo(undo)
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Verify all three objects survived the commit.
	gotBlock, err := db.GetBlock(block.BlockHash())
	if err != nil || gotBlock == nil {
		t.Errorf("block not persisted after atomic write: %v", err)
	}
	gotUTXO, err := db.GetUTXO(utxo.OutPoint)
	if err != nil || gotUTXO == nil {
		t.Errorf("UTXO not persisted after atomic write: %v", err)
	}
	gotUndo, err := db.GetUndo(undo.BlockHash)
	if err != nil || gotUndo == nil {
		t.Errorf("undo not persisted after atomic write: %v", err)
	}
}

func TestUpdate_Atomic_Rollback(t *testing.T) {
	// When fn returns an error mid-way, no writes should survive.
	db := tempDB(t)
	utxo := sampleUTXO()
	sentinel := errors.New("intentional rollback")

	err := db.Update(func(tx *WriteTx) error {
		if err := tx.PutUTXO(utxo); err != nil {
			return err
		}
		// Return an error after the PutUTXO write — bbolt must roll back.
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got: %v", err)
	}

	// The UTXO must NOT be present; bbolt rolled back the entire transaction.
	got, err := db.GetUTXO(utxo.OutPoint)
	if err != nil {
		t.Fatalf("GetUTXO after rollback: %v", err)
	}
	if got != nil {
		t.Fatal("UTXO persisted despite transaction rollback")
	}
}

func TestUpdate_Atomic_MultiWriteKinds(t *testing.T) {
	// Atomically write block, header, blockIndex, activeHash, bestTip, undo,
	// UTXO — then verify every one is readable after commit.
	db := tempDB(t)
	block := sampleBlock()
	header := block.Header
	idx := sampleBlockIndex()
	utxo := sampleUTXO()
	tip := &types.ChainTip{Hash: block.BlockHash(), Height: 1, TotalWork: big.NewInt(1)}
	undo := &types.BlockUndo{BlockHash: block.BlockHash()}

	err := db.Update(func(tx *WriteTx) error {
		if err := tx.SaveBlock(block); err != nil {
			return err
		}
		if err := tx.SaveHeader(&header); err != nil {
			return err
		}
		if err := tx.SaveBlockIndex(idx); err != nil {
			return err
		}
		if err := tx.SetActiveHash(0, block.BlockHash()); err != nil {
			return err
		}
		if err := tx.SetBestTip(tip); err != nil {
			return err
		}
		if err := tx.SaveUndo(undo); err != nil {
			return err
		}
		return tx.PutUTXO(utxo)
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Spot-check a few reads.
	if gotBlock, _ := db.GetBlock(block.BlockHash()); gotBlock == nil {
		t.Error("block missing after commit")
	}
	if gotTip, _ := db.GetBestTip(); gotTip == nil || gotTip.Height != 1 {
		t.Error("tip missing or wrong after commit")
	}
	if gotHash, found, _ := db.GetActiveHash(0); !found || gotHash != block.BlockHash() {
		t.Error("active hash missing or wrong after commit")
	}
}
