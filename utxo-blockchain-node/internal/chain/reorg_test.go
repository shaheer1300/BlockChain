package chain_test

// Reorg tests verify Milestone 11: fork-choice and chain reorganisation.
//
// Test scenario (all blocks are coinbase-only so they validate against any
// committed UTXO state — no UTXO conflicts between the two branches):
//
//   genesis
//     ├─ A1 → A2        (active chain, height 2, TotalWork=2)
//     └─ B1 → B2 → B3   (side chain, height 3, TotalWork=3 → triggers reorg)
//
// After importing B3 the node must switch to the longer chain and the UTXO
// set must reflect exactly the blocks on the new active chain.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/chain"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/crypto"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/storage"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── reorg scenario builder ────────────────────────────────────────────────────

// reorgScenario holds all the pieces of the standard fork scenario.
type reorgScenario struct {
	mgr         *chain.Manager
	db          *storage.DB
	genesisHash types.Hash32
	genesisAddr types.Address
	genesisCbID types.Hash32
	addrA       types.Address // miner for A-branch blocks
	addrB       types.Address // miner for B-branch blocks (different → distinct TxIDs)
	A1, A2      *types.Block
	B1, B2, B3  *types.Block
}

// buildReorgScenario creates a Manager, mines genesis, then mines A1+A2 on the
// active chain and B1+B2+B3 on a separate branch (from genesis). It imports
// A1, A2, B1, B2 but NOT B3 — the caller decides when to import B3.
//
// A-branch and B-branch use different miner addresses so their coinbase
// transactions have distinct TxIDs even at the same height.
func buildReorgScenario(t *testing.T) *reorgScenario {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chain.db")
	db := openDB(t, path)
	m, err := chain.NewManager(db, 0)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	kGenesis := newKey(t)
	addrG := crypto.PubKeyToAddress(kGenesis.PubKey())
	kA := newKey(t)
	addrA := crypto.PubKeyToAddress(kA.PubKey())
	kB := newKey(t)
	addrB := crypto.PubKeyToAddress(kB.PubKey())
	now := time.Now().Unix()

	if err := m.InitGenesis(context.Background(), addrG, chain.InitialSubsidy, now); err != nil {
		t.Fatalf("InitGenesis: %v", err)
	}
	genesisHash := m.Tip().Hash
	genesisHeader, err := db.GetHeader(genesisHash)
	if err != nil || genesisHeader == nil {
		t.Fatalf("GetHeader genesis: %v", err)
	}
	genesisCbID := genesisCoinbaseTxID(addrG, chain.InitialSubsidy)

	// ── Chain A: two blocks from genesis (miner = addrA) ─────────────
	A1, err := chain.MineBlock(context.Background(), genesisHeader, addrA,
		chain.BlockSubsidy(1), nil, 0, now+1)
	if err != nil {
		t.Fatalf("MineBlock A1: %v", err)
	}
	if _, err := m.ImportBlock(A1, now+1); err != nil {
		t.Fatalf("ImportBlock A1: %v", err)
	}
	A1hdr := A1.Header
	A2, err := chain.MineBlock(context.Background(), &A1hdr, addrA,
		chain.BlockSubsidy(2), nil, 0, now+2)
	if err != nil {
		t.Fatalf("MineBlock A2: %v", err)
	}
	if _, err := m.ImportBlock(A2, now+2); err != nil {
		t.Fatalf("ImportBlock A2: %v", err)
	}

	// ── Chain B: three blocks from genesis (miner = addrB) ───────────
	B1, err := chain.MineBlock(context.Background(), genesisHeader, addrB,
		chain.BlockSubsidy(1), nil, 0, now+3)
	if err != nil {
		t.Fatalf("MineBlock B1: %v", err)
	}
	if _, err := m.ImportBlock(B1, now+3); err != nil {
		t.Fatalf("ImportBlock B1: %v", err)
	}
	B1hdr := B1.Header
	B2, err := chain.MineBlock(context.Background(), &B1hdr, addrB,
		chain.BlockSubsidy(2), nil, 0, now+4)
	if err != nil {
		t.Fatalf("MineBlock B2: %v", err)
	}
	if _, err := m.ImportBlock(B2, now+4); err != nil {
		t.Fatalf("ImportBlock B2: %v", err)
	}
	B2hdr := B2.Header
	B3, err := chain.MineBlock(context.Background(), &B2hdr, addrB,
		chain.BlockSubsidy(3), nil, 0, now+5)
	if err != nil {
		t.Fatalf("MineBlock B3: %v", err)
	}

	return &reorgScenario{
		mgr: m, db: db,
		genesisHash: genesisHash, genesisAddr: addrG, genesisCbID: genesisCbID,
		addrA: addrA, addrB: addrB,
		A1: A1, A2: A2,
		B1: B1, B2: B2, B3: B3,
	}
}

// ── test cases ────────────────────────────────────────────────────────────────

// TestSideChain_StoredWithoutReorg verifies that a side-chain block that does
// not beat the active chain is stored but leaves the tip unchanged.
func TestSideChain_StoredWithoutReorg(t *testing.T) {
	sc := buildReorgScenario(t)

	// After importing B1 and B2 the active tip must still be A2 (height 2).
	tip := sc.mgr.Tip()
	if tip.Height != 2 {
		t.Errorf("tip height = %d, want 2 (no reorg should have happened)", tip.Height)
	}
	if tip.Hash != sc.A2.BlockHash() {
		t.Errorf("tip hash = %s, want A2 hash %s", tip.Hash, sc.A2.BlockHash())
	}

	// B1 and B2 must be in the block index (stored) even though they are not active.
	b1Idx, err := sc.db.GetBlockIndex(sc.B1.BlockHash())
	if err != nil || b1Idx == nil {
		t.Fatalf("B1 not in block index: %v", err)
	}
	b2Idx, err := sc.db.GetBlockIndex(sc.B2.BlockHash())
	if err != nil || b2Idx == nil {
		t.Fatalf("B2 not in block index: %v", err)
	}
}

// TestReorg_TriggerOnHeavierSideChain verifies that importing B3 (which gives
// chain B TotalWork=3 vs chain A TotalWork=2) triggers a reorg.
func TestReorg_TriggerOnHeavierSideChain(t *testing.T) {
	sc := buildReorgScenario(t)
	now := time.Now().Unix()

	result, err := sc.mgr.ImportBlock(sc.B3, now+5)
	if err != nil {
		t.Fatalf("ImportBlock B3: %v", err)
	}
	if result == nil {
		t.Fatal("expected ReorgResult from B3 import, got nil")
	}

	// Tip must now be B3 at height 3.
	tip := sc.mgr.Tip()
	if tip.Height != 3 {
		t.Errorf("tip height = %d, want 3", tip.Height)
	}
	if tip.Hash != sc.B3.BlockHash() {
		t.Errorf("tip hash = %s, want B3 hash %s", tip.Hash, sc.B3.BlockHash())
	}
}

// TestReorg_ResultDescribesChanges verifies the contents of the ReorgResult.
func TestReorg_ResultDescribesChanges(t *testing.T) {
	sc := buildReorgScenario(t)
	now := time.Now().Unix()

	result, err := sc.mgr.ImportBlock(sc.B3, now+5)
	if err != nil || result == nil {
		t.Fatalf("ImportBlock B3: result=%v err=%v", result, err)
	}

	// Fork point must be genesis.
	if result.ForkPoint != sc.genesisHash {
		t.Errorf("ForkPoint = %s, want genesis %s", result.ForkPoint, sc.genesisHash)
	}
	// OldTip must have been A2.
	if result.OldTip != sc.A2.BlockHash() {
		t.Errorf("OldTip = %s, want A2 hash", result.OldTip)
	}
	// NewTip must be B3.
	if result.NewTip != sc.B3.BlockHash() {
		t.Errorf("NewTip = %s, want B3 hash", result.NewTip)
	}
	// Disconnected: [A1, A2] (oldest-first).
	if len(result.Disconnected) != 2 {
		t.Fatalf("Disconnected count = %d, want 2", len(result.Disconnected))
	}
	if result.Disconnected[0].BlockHash() != sc.A1.BlockHash() {
		t.Errorf("Disconnected[0] = %s, want A1", result.Disconnected[0].BlockHash())
	}
	if result.Disconnected[1].BlockHash() != sc.A2.BlockHash() {
		t.Errorf("Disconnected[1] = %s, want A2", result.Disconnected[1].BlockHash())
	}
	// Connected: [B1, B2, B3] (oldest-first).
	if len(result.Connected) != 3 {
		t.Fatalf("Connected count = %d, want 3", len(result.Connected))
	}
	if result.Connected[0].BlockHash() != sc.B1.BlockHash() {
		t.Errorf("Connected[0] = %s, want B1", result.Connected[0].BlockHash())
	}
	if result.Connected[2].BlockHash() != sc.B3.BlockHash() {
		t.Errorf("Connected[2] = %s, want B3", result.Connected[2].BlockHash())
	}
}

// TestReorg_OldChainUTXOsRemoved verifies that coinbase UTXOs from the
// disconnected blocks (A1, A2) no longer exist after the reorg.
func TestReorg_OldChainUTXOsRemoved(t *testing.T) {
	sc := buildReorgScenario(t)
	now := time.Now().Unix()

	if _, err := sc.mgr.ImportBlock(sc.B3, now+5); err != nil {
		t.Fatalf("ImportBlock B3: %v", err)
	}

	for _, blk := range []*types.Block{sc.A1, sc.A2} {
		cbTxID := blk.Transactions[0].TxID()
		op := types.OutPoint{TxID: cbTxID, Index: 0}
		utxo, err := sc.db.GetUTXO(op)
		if err != nil {
			t.Fatalf("GetUTXO for disconnected block coinbase: %v", err)
		}
		if utxo != nil {
			t.Errorf("coinbase UTXO of disconnected block %s still exists after reorg", blk.BlockHash())
		}
	}
}

// TestReorg_NewChainUTXOsPresent verifies that coinbase UTXOs from the newly
// connected blocks (B1, B2, B3) exist after the reorg.
func TestReorg_NewChainUTXOsPresent(t *testing.T) {
	sc := buildReorgScenario(t)
	now := time.Now().Unix()

	if _, err := sc.mgr.ImportBlock(sc.B3, now+5); err != nil {
		t.Fatalf("ImportBlock B3: %v", err)
	}

	for _, blk := range []*types.Block{sc.B1, sc.B2, sc.B3} {
		cbTxID := blk.Transactions[0].TxID()
		op := types.OutPoint{TxID: cbTxID, Index: 0}
		utxo, err := sc.db.GetUTXO(op)
		if err != nil {
			t.Fatalf("GetUTXO for new chain coinbase: %v", err)
		}
		if utxo == nil {
			t.Errorf("coinbase UTXO of connected block %s missing after reorg", blk.BlockHash())
		}
	}
}

// TestReorg_GenesisCoinbasePreserved verifies that the genesis coinbase UTXO
// (which belongs to the common ancestor) is untouched by the reorg.
func TestReorg_GenesisCoinbasePreserved(t *testing.T) {
	sc := buildReorgScenario(t)
	now := time.Now().Unix()

	if _, err := sc.mgr.ImportBlock(sc.B3, now+5); err != nil {
		t.Fatalf("ImportBlock B3: %v", err)
	}

	op := types.OutPoint{TxID: sc.genesisCbID, Index: 0}
	utxo, err := sc.db.GetUTXO(op)
	if err != nil {
		t.Fatalf("GetUTXO genesis coinbase: %v", err)
	}
	if utxo == nil {
		t.Fatal("genesis coinbase UTXO was incorrectly removed during reorg")
	}
}

// TestReorg_ActiveChainIndexUpdated verifies that the active-chain index
// entries for heights 1–3 point to the new branch blocks after the reorg.
func TestReorg_ActiveChainIndexUpdated(t *testing.T) {
	sc := buildReorgScenario(t)
	now := time.Now().Unix()

	if _, err := sc.mgr.ImportBlock(sc.B3, now+5); err != nil {
		t.Fatalf("ImportBlock B3: %v", err)
	}

	wantAtHeight := map[uint32]types.Hash32{
		1: sc.B1.BlockHash(),
		2: sc.B2.BlockHash(),
		3: sc.B3.BlockHash(),
	}
	for h, want := range wantAtHeight {
		got, found, err := sc.db.GetActiveHash(h)
		if err != nil {
			t.Fatalf("GetActiveHash(%d): %v", h, err)
		}
		if !found {
			t.Errorf("height %d not in active chain index after reorg", h)
			continue
		}
		if got != want {
			t.Errorf("height %d: got %s, want %s", h, got, want)
		}
	}
}

// TestReorg_Idempotent verifies that importing B3 a second time (block already
// known) does not crash or corrupt state.
func TestReorg_Idempotent(t *testing.T) {
	sc := buildReorgScenario(t)
	now := time.Now().Unix()

	if _, err := sc.mgr.ImportBlock(sc.B3, now+5); err != nil {
		t.Fatalf("first ImportBlock B3: %v", err)
	}
	tipAfterFirst := sc.mgr.Tip().Hash

	// Second import: B3's parent (B2) is not the current tip (B3), so it
	// will be treated as a side-chain block. TotalWork equals active chain
	// TotalWork so no reorg fires — tip is unchanged.
	_, _ = sc.mgr.ImportBlock(sc.B3, now+5)
	if sc.mgr.Tip().Hash != tipAfterFirst {
		t.Error("tip changed after re-importing an already-known block")
	}
}

// TestReorg_PersistsThroughRestart verifies that the new chain state survives
// a database close and reopen.
func TestReorg_PersistsThroughRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	kG := newKey(t)
	addrG := crypto.PubKeyToAddress(kG.PubKey())
	kA := newKey(t)
	addrA := crypto.PubKeyToAddress(kA.PubKey())
	kB := newKey(t)
	addrB := crypto.PubKeyToAddress(kB.PubKey())
	now := time.Now().Unix()

	var b3Hash types.Hash32

	// ── First run: build scenario and trigger reorg ───────────────────
	{
		db1, err := storage.Open(path)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		m1, _ := chain.NewManager(db1, 0)
		if err := m1.InitGenesis(context.Background(), addrG, chain.InitialSubsidy, now); err != nil {
			t.Fatalf("InitGenesis: %v", err)
		}

		gHdr, _ := db1.GetHeader(m1.Tip().Hash)

		A1, _ := chain.MineBlock(context.Background(), gHdr, addrA, chain.BlockSubsidy(1), nil, 0, now+1)
		_, _ = m1.ImportBlock(A1, now+1)
		A1hdr := A1.Header
		A2, _ := chain.MineBlock(context.Background(), &A1hdr, addrA, chain.BlockSubsidy(2), nil, 0, now+2)
		_, _ = m1.ImportBlock(A2, now+2)

		B1, _ := chain.MineBlock(context.Background(), gHdr, addrB, chain.BlockSubsidy(1), nil, 0, now+3)
		_, _ = m1.ImportBlock(B1, now+3)
		B1hdr := B1.Header
		B2, _ := chain.MineBlock(context.Background(), &B1hdr, addrB, chain.BlockSubsidy(2), nil, 0, now+4)
		_, _ = m1.ImportBlock(B2, now+4)
		B2hdr := B2.Header
		B3, _ := chain.MineBlock(context.Background(), &B2hdr, addrB, chain.BlockSubsidy(3), nil, 0, now+5)
		_, _ = m1.ImportBlock(B3, now+5)
		b3Hash = B3.BlockHash()
		_ = db1.Close()
	}

	// ── Second run: confirm tip and UTXO set are correct ─────────────
	{
		db2, err := storage.Open(path)
		if err != nil {
			t.Fatalf("Reopen: %v", err)
		}
		defer db2.Close()

		m2, err := chain.NewManager(db2, 0)
		if err != nil {
			t.Fatalf("NewManager after reopen: %v", err)
		}

		tip := m2.Tip()
		if tip == nil {
			t.Fatal("tip is nil after reopen")
		}
		if tip.Height != 3 {
			t.Errorf("tip height = %d, want 3 after reorg", tip.Height)
		}
		if tip.Hash != b3Hash {
			t.Errorf("tip hash = %s, want B3 hash %s", tip.Hash, b3Hash)
		}
	}
}
