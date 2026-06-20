package storage

import (
	"path/filepath"
	"testing"

	bbolt "go.etcd.io/bbolt"
)

// tempDB opens a fresh database in a temporary directory and registers
// cleanup so the test never leaks file handles.
func tempDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return db
}

func TestDB_OpenClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDB_BucketsExist(t *testing.T) {
	db := tempDB(t)
	required := [][]byte{
		bucketBlocks,
		bucketHeaders,
		bucketBlockIndex,
		bucketActiveChain,
		bucketUTXOs,
		bucketUndo,
		bucketMempoolOptional,
		bucketChainMeta,
	}
	err := db.bolt.View(func(tx *bbolt.Tx) error {
		for _, name := range required {
			if tx.Bucket(name) == nil {
				t.Errorf("bucket %q not found", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("View: %v", err)
	}
}

func TestDB_ReopenPreservesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")

	// Open, write a UTXO, close.
	db1, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	utxo := sampleUTXO()
	if err := db1.PutUTXO(utxo); err != nil {
		t.Fatalf("PutUTXO: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and confirm the UTXO is still there.
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()

	got, err := db2.GetUTXO(utxo.OutPoint)
	if err != nil {
		t.Fatalf("GetUTXO after reopen: %v", err)
	}
	if got == nil {
		t.Fatal("UTXO not found after reopen")
	}
	if got.OutPoint != utxo.OutPoint {
		t.Errorf("OutPoint mismatch after reopen")
	}
}
