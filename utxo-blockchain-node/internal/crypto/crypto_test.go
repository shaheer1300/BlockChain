package crypto

import (
	"errors"
	"testing"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── hash / DoubleSHA256 tests ─────────────────────────────────────────────────

func TestSHA256_Deterministic(t *testing.T) {
	data := []byte("blockchain")
	a := SHA256(data)
	b := SHA256(data)
	if a != b {
		t.Fatal("SHA256 is not deterministic")
	}
}

func TestDoubleSHA256_Deterministic(t *testing.T) {
	data := []byte("blockchain")
	a := DoubleSHA256(data)
	b := DoubleSHA256(data)
	if a != b {
		t.Fatal("DoubleSHA256 is not deterministic")
	}
}

func TestDoubleSHA256_DifferentFromSingle(t *testing.T) {
	data := []byte("hello")
	if SHA256(data) == DoubleSHA256(data) {
		t.Fatal("SHA256 and DoubleSHA256 must differ")
	}
}

func TestDoubleSHA256_ChangesWithInput(t *testing.T) {
	if DoubleSHA256([]byte("a")) == DoubleSHA256([]byte("b")) {
		t.Fatal("different inputs produced same DoubleSHA256")
	}
}

// ── Merkle root tests ─────────────────────────────────────────────────────────

func makeLeaf(seed byte) types.Hash32 {
	var h types.Hash32
	for i := range h {
		h[i] = seed + byte(i)
	}
	return h
}

func TestMerkleRoot_Empty(t *testing.T) {
	got := MerkleRoot(nil)
	if got != types.ZeroHash {
		t.Fatalf("MerkleRoot(nil) = %s, want ZeroHash", got)
	}
}

func TestMerkleRoot_SingleLeaf(t *testing.T) {
	leaf := makeLeaf(1)
	got := MerkleRoot([]types.Hash32{leaf})
	if got != leaf {
		t.Fatalf("MerkleRoot([1]) = %s, want the leaf itself", got)
	}
}

func TestMerkleRoot_TwoLeaves(t *testing.T) {
	a, b := makeLeaf(1), makeLeaf(2)
	got := MerkleRoot([]types.Hash32{a, b})
	combined := append(a[:], b[:]...)
	want := DoubleSHA256(combined)
	if got != want {
		t.Fatalf("MerkleRoot([a,b]) mismatch\n got  %s\n want %s", got, want)
	}
}

func TestMerkleRoot_OddLeaves(t *testing.T) {
	leaves := []types.Hash32{makeLeaf(1), makeLeaf(2), makeLeaf(3)}
	got := MerkleRoot(leaves)
	// Level 1: hash(1,2) and hash(3,3) [duplicate]
	h12 := DoubleSHA256(append(leaves[0][:], leaves[1][:]...))
	h33 := DoubleSHA256(append(leaves[2][:], leaves[2][:]...))
	want := DoubleSHA256(append(h12[:], h33[:]...))
	if got != want {
		t.Fatalf("MerkleRoot(3 leaves) mismatch\n got  %s\n want %s", got, want)
	}
}

func TestMerkleRoot_FourLeaves(t *testing.T) {
	a, b, c, d := makeLeaf(1), makeLeaf(2), makeLeaf(3), makeLeaf(4)
	got := MerkleRoot([]types.Hash32{a, b, c, d})
	hab := DoubleSHA256(append(a[:], b[:]...))
	hcd := DoubleSHA256(append(c[:], d[:]...))
	want := DoubleSHA256(append(hab[:], hcd[:]...))
	if got != want {
		t.Fatalf("MerkleRoot(4 leaves) mismatch\n got  %s\n want %s", got, want)
	}
}

func TestMerkleRoot_Deterministic(t *testing.T) {
	leaves := []types.Hash32{makeLeaf(10), makeLeaf(20), makeLeaf(30)}
	if MerkleRoot(leaves) != MerkleRoot(leaves) {
		t.Fatal("MerkleRoot is not deterministic")
	}
}

func TestMerkleRoot_DoesNotMutateInput(t *testing.T) {
	leaves := []types.Hash32{makeLeaf(1), makeLeaf(2), makeLeaf(3)}
	before := [3]types.Hash32{leaves[0], leaves[1], leaves[2]}
	MerkleRoot(leaves)
	for i, l := range leaves {
		if l != before[i] {
			t.Fatalf("MerkleRoot mutated input leaf %d", i)
		}
	}
}

// ── key generation / address tests ───────────────────────────────────────────

func TestGenerateKey_Unique(t *testing.T) {
	a, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	b, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if string(a.Serialize()) == string(b.Serialize()) {
		t.Fatal("two generated keys are identical")
	}
}

func TestPubKeyToAddress_Deterministic(t *testing.T) {
	priv, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pub := PublicKeyFromPrivate(priv)
	a := PubKeyToAddress(pub)
	b := PubKeyToAddress(pub)
	if a != b {
		t.Fatal("PubKeyToAddress is not deterministic")
	}
}

func TestPubKeyToAddress_NotZero(t *testing.T) {
	priv, _ := GenerateKey()
	addr := PubKeyToAddress(priv.PubKey())
	if addr == types.ZeroAddress {
		t.Fatal("address must not be the zero address")
	}
}

func TestPubKeyToAddress_DifferentKeysGiveDifferentAddresses(t *testing.T) {
	a, _ := GenerateKey()
	b, _ := GenerateKey()
	if PubKeyToAddress(a.PubKey()) == PubKeyToAddress(b.PubKey()) {
		t.Fatal("different keys produced the same address")
	}
}

// ── signing and verification tests ───────────────────────────────────────────

func buildSignedTx(t *testing.T) (*types.Transaction, *PrivateKey) {
	t.Helper()
	priv, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	addr := PubKeyToAddress(priv.PubKey())

	var prevTxID types.Hash32
	prevTxID[0] = 0xAB
	tx := &types.Transaction{
		Version: 1,
		Inputs:  []types.TxInput{{PrevOut: types.OutPoint{TxID: prevTxID, Index: 0}}},
		Outputs: []types.TxOutput{{Value: 100, Recipient: addr}},
	}

	sig, err := SignInput(tx, 0, priv)
	if err != nil {
		t.Fatalf("SignInput: %v", err)
	}
	tx.Inputs[0].Signature = sig
	tx.Inputs[0].PubKey = priv.PubKey().SerializeCompressed()
	return tx, priv
}

func TestVerifyInput_ValidSignature(t *testing.T) {
	tx, _ := buildSignedTx(t)
	if err := VerifyInput(tx, 0); err != nil {
		t.Fatalf("valid signature failed verification: %v", err)
	}
}

func TestVerifyInput_ModifiedTxFails(t *testing.T) {
	tx, _ := buildSignedTx(t)
	// Flip a bit in the output value — should invalidate the signature.
	tx.Outputs[0].Value++
	err := VerifyInput(tx, 0)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("modified tx: got err %v, want ErrInvalidSignature", err)
	}
}

func TestVerifyInput_WrongPublicKey(t *testing.T) {
	tx, _ := buildSignedTx(t)
	// Replace the stored pubkey with a different key's pubkey.
	other, _ := GenerateKey()
	tx.Inputs[0].PubKey = other.PubKey().SerializeCompressed()
	err := VerifyInput(tx, 0)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("wrong pubkey: got err %v, want ErrInvalidSignature", err)
	}
}

func TestVerifyInput_CorruptedSignature(t *testing.T) {
	tx, _ := buildSignedTx(t)
	// Corrupt the first byte of the DER signature.
	tx.Inputs[0].Signature[0] ^= 0xFF
	err := VerifyInput(tx, 0)
	if err == nil {
		t.Fatal("corrupted signature should not verify")
	}
}

func TestSigHashPreimage_InputIndexSuffix(t *testing.T) {
	var prevID types.Hash32
	prevID[0] = 1
	tx := &types.Transaction{
		Version: 1,
		Inputs: []types.TxInput{
			{PrevOut: types.OutPoint{TxID: prevID, Index: 0}},
			{PrevOut: types.OutPoint{TxID: prevID, Index: 1}},
		},
		Outputs: []types.TxOutput{{Value: 50}},
	}
	p0, err := SigHashPreimage(tx, 0)
	if err != nil {
		t.Fatalf("SigHashPreimage(0): %v", err)
	}
	p1, err := SigHashPreimage(tx, 1)
	if err != nil {
		t.Fatalf("SigHashPreimage(1): %v", err)
	}
	if len(p0) != len(p1) {
		t.Fatalf("preimage lengths differ: %d vs %d", len(p0), len(p1))
	}
	// Must differ because the index suffix is different.
	same := true
	for i := range p0 {
		if p0[i] != p1[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("preimages for different input indices must differ")
	}
}

func TestSigHashPreimage_OutOfRange(t *testing.T) {
	tx := &types.Transaction{Inputs: []types.TxInput{{}}}
	if _, err := SigHashPreimage(tx, 1); err == nil {
		t.Fatal("expected error for out-of-range input index")
	}
}
