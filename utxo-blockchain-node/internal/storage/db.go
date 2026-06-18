// Package storage owns all persistent state for the blockchain node. It
// wraps bbolt and exposes repository-style methods for blocks, headers,
// UTXOs, undo records, active-chain index, and chain metadata.
//
// Storage uses JSON for persisted values so that types.BlockIndex,
// types.ChainTip, and friends serialise without a separate binary schema.
// Consensus hashes are never derived from stored JSON — those always use
// the canonical binary encoder in internal/types.
package storage

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
	bbolt "go.etcd.io/bbolt"
)

// Bucket names. Every bucket must be created in initBuckets; add new
// buckets there if this list grows.
var (
	bucketBlocks          = []byte("blocks")
	bucketHeaders         = []byte("headers")
	bucketBlockIndex      = []byte("block_index")
	bucketActiveChain     = []byte("active_chain")
	bucketUTXOs           = []byte("utxos")
	bucketUndo            = []byte("undo")
	bucketMempoolOptional = []byte("mempool_optional")
	bucketChainMeta       = []byte("chain_meta")
)

// openTimeout is how long Open will wait if the database file is locked
// by another process.
const openTimeout = 5 * time.Second

// DB owns the bbolt database handle. All repository methods are defined
// as pointer receivers on DB so the caller never sees raw bbolt types.
type DB struct {
	bolt *bbolt.DB
}

// Open opens (or creates) the bbolt database at path and ensures all
// required buckets exist. The file is created with mode 0600. If another
// process holds the file lock, Open returns an error after openTimeout.
func Open(path string) (*DB, error) {
	opts := &bbolt.Options{Timeout: openTimeout}
	bdb, err := bbolt.Open(path, 0o600, opts)
	if err != nil {
		return nil, fmt.Errorf("storage: open %q: %w", path, err)
	}
	if err := bdb.Update(initBuckets); err != nil {
		_ = bdb.Close()
		return nil, fmt.Errorf("storage: init buckets: %w", err)
	}
	return &DB{bolt: bdb}, nil
}

// Close releases the database file lock and flushes any pending writes.
func (db *DB) Close() error {
	return db.bolt.Close()
}

// initBuckets creates all required buckets inside a single writable
// transaction. Adding a new bucket here is the only change needed when
// the bucket list grows.
func initBuckets(tx *bbolt.Tx) error {
	for _, name := range [][]byte{
		bucketBlocks,
		bucketHeaders,
		bucketBlockIndex,
		bucketActiveChain,
		bucketUTXOs,
		bucketUndo,
		bucketMempoolOptional,
		bucketChainMeta,
	} {
		if _, err := tx.CreateBucketIfNotExists(name); err != nil {
			return fmt.Errorf("storage: create bucket %q: %w", name, err)
		}
	}
	return nil
}

// heightKey encodes height as a 4-byte big-endian key so that bbolt's
// byte-ordered cursor returns heights in ascending numeric order.
func heightKey(height uint32) [4]byte {
	var key [4]byte
	binary.BigEndian.PutUint32(key[:], height)
	return key
}

// outpointKey encodes an OutPoint as a 36-byte key (32-byte TxID followed
// by 4-byte big-endian Index). Using big-endian for the index keeps the
// key layout consistent with heightKey and avoids allocation.
func outpointKey(op types.OutPoint) [36]byte {
	var key [36]byte
	copy(key[:32], op.TxID[:])
	binary.BigEndian.PutUint32(key[32:], op.Index)
	return key
}

// marshalJSON serialises v to JSON for storage.
func marshalJSON(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("storage: marshal: %w", err)
	}
	return data, nil
}

// unmarshalJSON deserialises data from storage into v.
func unmarshalJSON(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("storage: unmarshal: %w", err)
	}
	return nil
}
