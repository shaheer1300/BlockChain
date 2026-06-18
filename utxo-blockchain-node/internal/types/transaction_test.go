package types

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

func sampleTx() Transaction {
	var prevTxID Hash32
	for i := range prevTxID {
		prevTxID[i] = byte(i + 1)
	}
	var recipientA, recipientB Address
	for i := range recipientA {
		recipientA[i] = byte(i + 10)
		recipientB[i] = byte(i + 50)
	}
	return Transaction{
		Version: 1,
		Inputs: []TxInput{
			{
				PrevOut:   OutPoint{TxID: prevTxID, Index: 7},
				Signature: []byte{0xAA, 0xBB, 0xCC, 0xDD},
				PubKey:    []byte{0x02, 0x11, 0x22, 0x33, 0x44},
			},
		},
		Outputs: []TxOutput{
			{Value: 100, Recipient: recipientA},
			{Value: 250, Recipient: recipientB},
		},
		LockTime: 0,
	}
}

func TestTransaction_TxIDDeterministic(t *testing.T) {
	a := sampleTx()
	b := sampleTx()
	if a.TxID() != b.TxID() {
		t.Fatalf("identical transactions produced different TxIDs")
	}
}

func TestTransaction_TxIDChangesWithFields(t *testing.T) {
	base := sampleTx()
	baseID := base.TxID()

	mutations := map[string]func(*Transaction){
		"version": func(t *Transaction) {
			t.Version = 2
		},
		"input prevout index": func(t *Transaction) {
			t.Inputs[0].PrevOut.Index = 8
		},
		"input signature": func(t *Transaction) {
			t.Inputs[0].Signature = append([]byte(nil), t.Inputs[0].Signature...)
			t.Inputs[0].Signature[0] ^= 0xFF
		},
		"input pubkey": func(t *Transaction) {
			t.Inputs[0].PubKey = append([]byte(nil), t.Inputs[0].PubKey...)
			t.Inputs[0].PubKey[0] ^= 0xFF
		},
		"output value": func(t *Transaction) {
			t.Outputs[0].Value++
		},
		"output recipient": func(t *Transaction) {
			t.Outputs[0].Recipient[0] ^= 0xFF
		},
		"locktime": func(t *Transaction) {
			t.LockTime = 42
		},
	}

	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			mutated := sampleTx()
			mutate(&mutated)
			if mutated.TxID() == baseID {
				t.Fatalf("mutation %q did not change TxID", name)
			}
		})
	}
}

func TestTransaction_TxIDIsNotJSON(t *testing.T) {
	tx := sampleTx()
	encoded, err := tx.CanonicalEncode()
	if err != nil {
		t.Fatalf("CanonicalEncode: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("empty encoding")
	}
	if encoded[0] == '{' || encoded[0] == '[' || encoded[0] == '"' {
		t.Fatalf("canonical encoding looks like JSON: starts with %q", encoded[0])
	}
	// Version=1 little-endian => 01 00 00 00 must be the prefix.
	wantPrefix := []byte{0x01, 0x00, 0x00, 0x00}
	if !bytes.HasPrefix(encoded, wantPrefix) {
		t.Fatalf("encoding does not start with LE Version: got %x", encoded[:4])
	}
}

func TestTransaction_IsCoinbase(t *testing.T) {
	t.Run("regular tx is not coinbase", func(t *testing.T) {
		tx := sampleTx()
		if tx.IsCoinbase() {
			t.Fatal("sample tx incorrectly reported as coinbase")
		}
	})
	t.Run("synthetic coinbase passes", func(t *testing.T) {
		var addr Address
		addr[0] = 1
		cb := Transaction{
			Version: 1,
			Inputs: []TxInput{{
				PrevOut: OutPoint{TxID: ZeroHash, Index: CoinbaseInputIndex},
			}},
			Outputs: []TxOutput{{Value: 50, Recipient: addr}},
		}
		if !cb.IsCoinbase() {
			t.Fatal("coinbase tx not detected")
		}
	})
	t.Run("two inputs fail", func(t *testing.T) {
		cb := Transaction{
			Inputs: []TxInput{
				{PrevOut: OutPoint{TxID: ZeroHash, Index: CoinbaseInputIndex}},
				{PrevOut: OutPoint{TxID: ZeroHash, Index: CoinbaseInputIndex}},
			},
		}
		if cb.IsCoinbase() {
			t.Fatal("two-input tx incorrectly reported as coinbase")
		}
	})
}

func TestTransaction_TotalOutputValue(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		tx := sampleTx()
		got, err := tx.TotalOutputValue()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 350 {
			t.Fatalf("got %d, want 350", got)
		}
	})
	t.Run("no outputs", func(t *testing.T) {
		tx := Transaction{}
		if _, err := tx.TotalOutputValue(); !errors.Is(err, ErrNoOutputs) {
			t.Fatalf("got err %v, want ErrNoOutputs", err)
		}
	})
	t.Run("overflow", func(t *testing.T) {
		var addr Address
		tx := Transaction{
			Outputs: []TxOutput{
				{Value: math.MaxUint64, Recipient: addr},
				{Value: 1, Recipient: addr},
			},
		}
		if _, err := tx.TotalOutputValue(); !errors.Is(err, ErrAmountOverflow) {
			t.Fatalf("got err %v, want ErrAmountOverflow", err)
		}
	})
}
