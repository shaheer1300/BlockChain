package chain_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/chain"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/crypto"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/storage"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── test helpers ──────────────────────────────────────────────────────────────

// openDB opens a bbolt database at path and registers cleanup.
func openDB(t *testing.T, path string) *storage.DB {
	t.Helper()
	db, err := storage.Open(path)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
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

// genesisCoinbaseTxID derives the TxID of the genesis coinbase transaction
// without reading the database. This works because TxID is fully deterministic
// from the transaction's content.
func genesisCoinbaseTxID(miner types.Address, subsidy types.Amount) types.Hash32 {
	cb := types.Transaction{
		Version: 1,
		Inputs: []types.TxInput{{
			PrevOut: types.OutPoint{TxID: types.ZeroHash, Index: types.CoinbaseInputIndex},
		}},
		Outputs: []types.TxOutput{{Value: subsidy, Recipient: miner}},
	}
	return cb.TxID()
}

// genesisAndBlock1 is a shared setup helper: initialises genesis and mines
// block1 (coinbase only, powNibbles=0) via MineBlock. Returns the manager,
// the db, the genesis header, and block1.
func genesisAndBlock1(t *testing.T, path string) (*chain.Manager, *storage.DB, *types.BlockHeader, *types.Block) {
	t.Helper()
	db := openDB(t, path)
	m, err := chain.NewManager(db, 0)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()

	if err := m.InitGenesis(context.Background(), addr, chain.InitialSubsidy, now); err != nil {
		t.Fatalf("InitGenesis: %v", err)
	}

	genesisHeader, err := db.GetHeader(m.Tip().Hash)
	if err != nil || genesisHeader == nil {
		t.Fatalf("GetHeader genesis: %v, header=%v", err, genesisHeader)
	}

	block1, err := chain.MineBlock(
		context.Background(), genesisHeader, addr,
		chain.BlockSubsidy(1), nil, 0, now+1,
	)
	if err != nil {
		t.Fatalf("MineBlock block1: %v", err)
	}
	return m, db, genesisHeader, block1
}

// ── BlockSubsidy ──────────────────────────────────────────────────────────────

func TestBlockSubsidy_AtHeight0(t *testing.T) {
	if got := chain.BlockSubsidy(0); got != chain.InitialSubsidy {
		t.Errorf("height 0: got %d, want %d", got, chain.InitialSubsidy)
	}
}

func TestBlockSubsidy_FirstHalving(t *testing.T) {
	want := chain.InitialSubsidy / 2
	if got := chain.BlockSubsidy(210_000); got != want {
		t.Errorf("height 210_000: got %d, want %d", got, want)
	}
}

func TestBlockSubsidy_ZeroAfter64Halvings(t *testing.T) {
	if got := chain.BlockSubsidy(210_000 * 64); got != 0 {
		t.Errorf("want 0 after 64 halvings, got %d", got)
	}
}

// ── Genesis ───────────────────────────────────────────────────────────────────

func TestInitGenesis_CreatesGenesis(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	db := openDB(t, path)
	m, err := chain.NewManager(db, 0)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if m.Tip() != nil {
		t.Fatal("fresh manager should have nil tip")
	}

	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()

	if err := m.InitGenesis(context.Background(), addr, chain.InitialSubsidy, now); err != nil {
		t.Fatalf("InitGenesis: %v", err)
	}

	tip := m.Tip()
	if tip == nil {
		t.Fatal("tip is nil after genesis")
	}
	if tip.Height != 0 {
		t.Errorf("genesis height = %d, want 0", tip.Height)
	}

	// Coinbase UTXO must exist in the database.
	cbTxID := genesisCoinbaseTxID(addr, chain.InitialSubsidy)
	utxo, err := db.GetUTXO(types.OutPoint{TxID: cbTxID, Index: 0})
	if err != nil {
		t.Fatalf("GetUTXO genesis coinbase: %v", err)
	}
	if utxo == nil {
		t.Fatal("genesis coinbase UTXO not found in database")
	}
	if utxo.Output.Value != chain.InitialSubsidy {
		t.Errorf("coinbase value = %d, want %d", utxo.Output.Value, chain.InitialSubsidy)
	}
	if !utxo.Coinbase {
		t.Error("genesis UTXO should be flagged as coinbase")
	}
}

func TestInitGenesis_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	db := openDB(t, path)
	m, _ := chain.NewManager(db, 0)

	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()

	if err := m.InitGenesis(context.Background(), addr, chain.InitialSubsidy, now); err != nil {
		t.Fatalf("first InitGenesis: %v", err)
	}
	firstHash := m.Tip().Hash

	// Second call with a different timestamp must be a no-op.
	if err := m.InitGenesis(context.Background(), addr, chain.InitialSubsidy, now+999); err != nil {
		t.Fatalf("second InitGenesis: %v", err)
	}
	if m.Tip().Hash != firstHash {
		t.Error("second InitGenesis changed the genesis block hash")
	}
}

func TestNewManager_RestoresTipFromDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()

	// First manager: init genesis, then close the database explicitly.
	db1, err := storage.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	m1, _ := chain.NewManager(db1, 0)
	if err := m1.InitGenesis(context.Background(), addr, chain.InitialSubsidy, now); err != nil {
		_ = db1.Close()
		t.Fatalf("InitGenesis: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("Close db1: %v", err)
	}

	// Second manager on the same path: tip must be restored from persisted state.
	db2 := openDB(t, path)
	m2, err := chain.NewManager(db2, 0)
	if err != nil {
		t.Fatalf("NewManager second: %v", err)
	}
	if m2.Tip() == nil {
		t.Fatal("tip is nil after reopen; NewManager did not restore from DB")
	}
	if m2.Tip().Height != 0 {
		t.Errorf("restored tip height = %d, want 0", m2.Tip().Height)
	}
}

// ── ImportBlock ───────────────────────────────────────────────────────────────

func TestImportBlock_BeforeGenesisReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	db := openDB(t, path)
	m, _ := chain.NewManager(db, 0)

	dummy := &types.Block{Header: types.BlockHeader{Version: 1}}
	if err := m.ImportBlock(dummy, time.Now().Unix()); err == nil {
		t.Fatal("expected error when importing before genesis")
	}
}

func TestImportBlock_ConnectsBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	m, _, _, block1 := genesisAndBlock1(t, path)

	if err := m.ImportBlock(block1, time.Now().Unix()+1); err != nil {
		t.Fatalf("ImportBlock: %v", err)
	}

	tip := m.Tip()
	if tip.Height != 1 {
		t.Errorf("height = %d, want 1", tip.Height)
	}
	if tip.Hash != block1.BlockHash() {
		t.Errorf("tip hash mismatch: got %s, want %s", tip.Hash, block1.BlockHash())
	}
}

func TestImportBlock_CoinbaseUTXOAdded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	m, db, _, block1 := genesisAndBlock1(t, path)

	if err := m.ImportBlock(block1, time.Now().Unix()+1); err != nil {
		t.Fatalf("ImportBlock: %v", err)
	}

	// Block1 coinbase UTXO must exist.
	cb1TxID := block1.Transactions[0].TxID()
	utxo, err := db.GetUTXO(types.OutPoint{TxID: cb1TxID, Index: 0})
	if err != nil {
		t.Fatalf("GetUTXO block1 coinbase: %v", err)
	}
	if utxo == nil {
		t.Fatal("block1 coinbase UTXO not found")
	}
	if utxo.Height != 1 {
		t.Errorf("UTXO height = %d, want 1", utxo.Height)
	}
	if !utxo.Coinbase {
		t.Error("block1 UTXO should be flagged as coinbase")
	}
}

func TestImportBlock_UndoExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	m, db, _, block1 := genesisAndBlock1(t, path)

	if err := m.ImportBlock(block1, time.Now().Unix()+1); err != nil {
		t.Fatalf("ImportBlock: %v", err)
	}

	undo, err := db.GetUndo(block1.BlockHash())
	if err != nil {
		t.Fatalf("GetUndo: %v", err)
	}
	if undo == nil {
		t.Fatal("undo record not found for block1")
	}
	// Coinbase-only block has no spends.
	if len(undo.Spent) != 0 {
		t.Errorf("Spent length = %d, want 0 for coinbase-only block", len(undo.Spent))
	}
}

func TestImportBlock_SpendUTXO(t *testing.T) {
	// Build a block containing a signed transaction that spends the genesis
	// coinbase. Verify that: the genesis UTXO is deleted, the spend-tx output
	// is added, and the undo record records the spent UTXO.
	path := filepath.Join(t.TempDir(), "chain.db")
	db := openDB(t, path)

	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	subsidy := chain.InitialSubsidy

	m, _ := chain.NewManager(db, 0)
	if err := m.InitGenesis(context.Background(), addr, subsidy, now); err != nil {
		t.Fatalf("InitGenesis: %v", err)
	}

	genesisHeader, _ := db.GetHeader(m.Tip().Hash)
	genCbTxID := genesisCoinbaseTxID(addr, subsidy)
	op := types.OutPoint{TxID: genCbTxID, Index: 0}

	// Build and sign a transaction spending the genesis coinbase.
	fee := types.Amount(1_000)
	outValue := subsidy - fee
	spendTx := &types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: op}},
		Outputs: []types.TxOutput{{Value: outValue, Recipient: addr}},
	}
	sig, err := crypto.SignInput(spendTx, 0, k)
	if err != nil {
		t.Fatalf("SignInput: %v", err)
	}
	spendTx.Inputs[0].Signature = sig
	spendTx.Inputs[0].PubKey = k.PubKey().SerializeCompressed()

	// Block1 coinbase claims subsidy + fee (exactly the maximum allowed).
	block1, err := chain.MineBlock(
		context.Background(), genesisHeader, addr,
		chain.BlockSubsidy(1)+fee, []types.Transaction{*spendTx}, 0, now+1,
	)
	if err != nil {
		t.Fatalf("MineBlock: %v", err)
	}

	if err := m.ImportBlock(block1, now+1); err != nil {
		t.Fatalf("ImportBlock: %v", err)
	}

	// Genesis coinbase UTXO must be gone.
	gone, err := db.GetUTXO(op)
	if err != nil {
		t.Fatalf("GetUTXO genesis after spend: %v", err)
	}
	if gone != nil {
		t.Fatal("genesis coinbase UTXO should be deleted after spend")
	}

	// The spend-tx output UTXO must exist with the correct value.
	newOp := types.OutPoint{TxID: spendTx.TxID(), Index: 0}
	newUtxo, err := db.GetUTXO(newOp)
	if err != nil {
		t.Fatalf("GetUTXO spend-tx output: %v", err)
	}
	if newUtxo == nil {
		t.Fatal("spend-tx output UTXO not found")
	}
	if newUtxo.Output.Value != outValue {
		t.Errorf("UTXO value = %d, want %d", newUtxo.Output.Value, outValue)
	}

	// Undo must record the genesis coinbase as the spent output.
	undo, err := db.GetUndo(block1.BlockHash())
	if err != nil {
		t.Fatalf("GetUndo: %v", err)
	}
	if undo == nil {
		t.Fatal("undo not found")
	}
	if len(undo.Spent) != 1 {
		t.Fatalf("Spent length = %d, want 1", len(undo.Spent))
	}
	if undo.Spent[0].OutPoint != op {
		t.Errorf("undo spent outpoint = %v, want %v", undo.Spent[0].OutPoint, op)
	}
	if undo.Spent[0].Output.Value != subsidy {
		t.Errorf("undo spent value = %d, want %d", undo.Spent[0].Output.Value, subsidy)
	}
}

func TestImportBlock_OrphanBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	m, _, _, _ := genesisAndBlock1(t, path)

	// Block with a PrevHash that does not exist in the block index.
	var unknownHash types.Hash32
	unknownHash[0] = 0xDE
	unknownHash[1] = 0xAD
	orphan := &types.Block{
		Header: types.BlockHeader{Version: 1, PrevHash: unknownHash},
	}
	err := m.ImportBlock(orphan, time.Now().Unix()+1)
	if !errors.Is(err, chain.ErrOrphanBlock) {
		t.Errorf("expected ErrOrphanBlock, got: %v", err)
	}
}

// ── Restart persists chain state ──────────────────────────────────────────────

func TestChain_ReopenPreservesTipAndUTXO(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "chain.db")

	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	now := time.Now().Unix()
	var block1Hash types.Hash32
	var block1CbTxID types.Hash32

	// ── first run: genesis + block1 ───────────────────────────────────
	{
		db, err := storage.Open(dbPath)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		m, _ := chain.NewManager(db, 0)
		if err := m.InitGenesis(context.Background(), addr, chain.InitialSubsidy, now); err != nil {
			t.Fatalf("InitGenesis: %v", err)
		}
		genesisHeader, _ := db.GetHeader(m.Tip().Hash)
		block1, _ := chain.MineBlock(
			context.Background(), genesisHeader, addr,
			chain.BlockSubsidy(1), nil, 0, now+1,
		)
		if err := m.ImportBlock(block1, now+1); err != nil {
			t.Fatalf("ImportBlock: %v", err)
		}
		block1Hash = block1.BlockHash()
		block1CbTxID = block1.Transactions[0].TxID()
		_ = db.Close()
	}

	// ── second run: verify everything survived ────────────────────────
	{
		db, err := storage.Open(dbPath)
		if err != nil {
			t.Fatalf("Reopen: %v", err)
		}
		defer db.Close()

		m2, err := chain.NewManager(db, 0)
		if err != nil {
			t.Fatalf("NewManager after reopen: %v", err)
		}

		tip := m2.Tip()
		if tip == nil {
			t.Fatal("tip nil after reopen")
		}
		if tip.Height != 1 {
			t.Errorf("tip height = %d, want 1", tip.Height)
		}
		if tip.Hash != block1Hash {
			t.Errorf("tip hash mismatch after reopen")
		}

		// Block1 coinbase UTXO must still be present.
		utxo, err := db.GetUTXO(types.OutPoint{TxID: block1CbTxID, Index: 0})
		if err != nil {
			t.Fatalf("GetUTXO after reopen: %v", err)
		}
		if utxo == nil {
			t.Fatal("block1 coinbase UTXO missing after reopen")
		}
	}
}

// ── MineBlock ─────────────────────────────────────────────────────────────────

func TestMineBlock_PowerZeroIsInstant(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	block, err := chain.MineBlock(
		context.Background(), nil, addr,
		chain.InitialSubsidy, nil, 0, time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("MineBlock: %v", err)
	}
	if block == nil {
		t.Fatal("block is nil with powNibbles=0")
	}
}

func TestMineBlock_SatisfiesPoWTarget(t *testing.T) {
	// 1 nibble requires the high nibble of byte[0] to be 0x0; on average ~16
	// hashes needed — fast for a unit test.
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	block, err := chain.MineBlock(
		context.Background(), nil, addr,
		chain.InitialSubsidy, nil, 1, time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("MineBlock: %v", err)
	}
	hash := block.BlockHash()
	if hash[0]>>4 != 0 {
		t.Errorf("mined hash %s does not satisfy 1 leading zero nibble", hash)
	}
}

func TestMineBlock_ContextCancellation(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the first iteration

	_, err := chain.MineBlock(ctx, nil, addr, chain.InitialSubsidy, nil, 64, time.Now().Unix())
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

func TestMineBlock_ProducesCoinbaseForMiner(t *testing.T) {
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	value := chain.InitialSubsidy

	block, err := chain.MineBlock(
		context.Background(), nil, addr, value, nil, 0, time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("MineBlock: %v", err)
	}
	if len(block.Transactions) == 0 {
		t.Fatal("no transactions in mined block")
	}
	cb := block.Transactions[0]
	if !cb.IsCoinbase() {
		t.Fatal("first transaction is not a coinbase")
	}
	if cb.Outputs[0].Recipient != addr {
		t.Error("coinbase recipient mismatch")
	}
	if cb.Outputs[0].Value != value {
		t.Errorf("coinbase value = %d, want %d", cb.Outputs[0].Value, value)
	}
}

func TestMineBlock_ThenImport_HeightIncreases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	m, _, _, block1 := genesisAndBlock1(t, path)

	if err := m.ImportBlock(block1, time.Now().Unix()+1); err != nil {
		t.Fatalf("ImportBlock block1: %v", err)
	}
	if got := m.Tip().Height; got != 1 {
		t.Errorf("height after block1 = %d, want 1", got)
	}

	// Mine and import block2 on top of block1.
	k := newKey(t)
	addr := crypto.PubKeyToAddress(k.PubKey())
	block1Header := block1.Header
	block2, err := chain.MineBlock(
		context.Background(), &block1Header, addr,
		chain.BlockSubsidy(2), nil, 0, time.Now().Unix()+2,
	)
	if err != nil {
		t.Fatalf("MineBlock block2: %v", err)
	}
	if err := m.ImportBlock(block2, time.Now().Unix()+2); err != nil {
		t.Fatalf("ImportBlock block2: %v", err)
	}
	if got := m.Tip().Height; got != 2 {
		t.Errorf("height after block2 = %d, want 2", got)
	}
}

func TestMineBlock_MinerReceivesCoinbaseUTXO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	m, db, _, block1 := genesisAndBlock1(t, path)

	if err := m.ImportBlock(block1, time.Now().Unix()+1); err != nil {
		t.Fatalf("ImportBlock: %v", err)
	}

	cbTxID := block1.Transactions[0].TxID()
	utxo, err := db.GetUTXO(types.OutPoint{TxID: cbTxID, Index: 0})
	if err != nil {
		t.Fatalf("GetUTXO: %v", err)
	}
	if utxo == nil {
		t.Fatal("miner coinbase UTXO not found after ImportBlock")
	}
	if utxo.Output.Value != chain.BlockSubsidy(1) {
		t.Errorf("UTXO value = %d, want %d", utxo.Output.Value, chain.BlockSubsidy(1))
	}
}

func TestMineBlock_MinedBlockIsValidOnImport(t *testing.T) {
	// Prove that any block produced by MineBlock always passes ImportBlock
	// validation — invalid mined blocks are impossible under the normal path.
	path := filepath.Join(t.TempDir(), "chain.db")
	m, _, _, block1 := genesisAndBlock1(t, path)

	if err := m.ImportBlock(block1, time.Now().Unix()+1); err != nil {
		t.Errorf("mined block failed ImportBlock: %v", err)
	}
}
