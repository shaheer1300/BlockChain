package storage

import (
	"fmt"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
	bbolt "go.etcd.io/bbolt"
)

var keyBestTip = []byte("best_tip")

// saveBlockIndexTx is the transaction-scoped implementation shared by
// DB.SaveBlockIndex and WriteTx.SaveBlockIndex.
func saveBlockIndexTx(tx *bbolt.Tx, idx *types.BlockIndex) error {
	data, err := marshalJSON(idx)
	if err != nil {
		return err
	}
	return tx.Bucket(bucketBlockIndex).Put(idx.Hash[:], data)
}

// SaveBlockIndex persists a BlockIndex entry keyed by block hash.
func (db *DB) SaveBlockIndex(idx *types.BlockIndex) error {
	return db.bolt.Update(func(tx *bbolt.Tx) error {
		return saveBlockIndexTx(tx, idx)
	})
}

// GetBlockIndex retrieves a BlockIndex by hash. Returns (nil, nil) when not found.
func (db *DB) GetBlockIndex(hash types.Hash32) (*types.BlockIndex, error) {
	var idx *types.BlockIndex
	err := db.bolt.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket(bucketBlockIndex).Get(hash[:])
		if data == nil {
			return nil
		}
		idx = new(types.BlockIndex)
		return unmarshalJSON(data, idx)
	})
	if err != nil {
		return nil, fmt.Errorf("storage: GetBlockIndex %s: %w", hash, err)
	}
	return idx, nil
}

// setActiveHashTx is the transaction-scoped implementation shared by
// DB.SetActiveHash and WriteTx.SetActiveHash.
func setActiveHashTx(tx *bbolt.Tx, height uint32, hash types.Hash32) error {
	key := heightKey(height)
	return tx.Bucket(bucketActiveChain).Put(key[:], hash[:])
}

// SetActiveHash records the canonical block hash at a given chain height.
// The value is stored as raw 32 bytes (no JSON) because it is a fixed-size
// binary value and the active-chain index is read on every block lookup.
func (db *DB) SetActiveHash(height uint32, hash types.Hash32) error {
	return db.bolt.Update(func(tx *bbolt.Tx) error {
		return setActiveHashTx(tx, height, hash)
	})
}

// GetActiveHash returns the canonical block hash at height.
// Returns (ZeroHash, false, nil) when not found.
func (db *DB) GetActiveHash(height uint32) (types.Hash32, bool, error) {
	key := heightKey(height)
	var hash types.Hash32
	var found bool
	err := db.bolt.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket(bucketActiveChain).Get(key[:])
		if data == nil {
			return nil
		}
		if len(data) != types.HashSize {
			return fmt.Errorf("storage: active hash at height %d: got %d bytes, want %d",
				height, len(data), types.HashSize)
		}
		copy(hash[:], data)
		found = true
		return nil
	})
	if err != nil {
		return types.ZeroHash, false, err
	}
	return hash, found, nil
}

// ListActiveHashes returns every active-chain hash in ascending height
// order. The result length equals tip.Height+1 when the chain is fully
// consistent. Returns an empty slice when the chain has not been
// initialised. The bucket is iterated using bbolt's natural byte order,
// which matches the big-endian heightKey encoding.
func (db *DB) ListActiveHashes() ([]types.Hash32, error) {
	var hashes []types.Hash32
	err := db.bolt.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketActiveChain).ForEach(func(_, v []byte) error {
			if len(v) != types.HashSize {
				return fmt.Errorf("storage: ListActiveHashes: got %d bytes, want %d",
					len(v), types.HashSize)
			}
			var h types.Hash32
			copy(h[:], v)
			hashes = append(hashes, h)
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("storage: ListActiveHashes: %w", err)
	}
	return hashes, nil
}

// setBestTipTx is the transaction-scoped implementation shared by
// DB.SetBestTip and WriteTx.SetBestTip.
func setBestTipTx(tx *bbolt.Tx, tip *types.ChainTip) error {
	data, err := marshalJSON(tip)
	if err != nil {
		return err
	}
	return tx.Bucket(bucketChainMeta).Put(keyBestTip, data)
}

// SetBestTip persists the current best chain tip. Overwrites any previous value.
func (db *DB) SetBestTip(tip *types.ChainTip) error {
	return db.bolt.Update(func(tx *bbolt.Tx) error {
		return setBestTipTx(tx, tip)
	})
}

// GetBestTip retrieves the current best chain tip.
// Returns (nil, nil) when the chain has not been initialised yet.
func (db *DB) GetBestTip() (*types.ChainTip, error) {
	var tip *types.ChainTip
	err := db.bolt.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket(bucketChainMeta).Get(keyBestTip)
		if data == nil {
			return nil
		}
		tip = new(types.ChainTip)
		return unmarshalJSON(data, tip)
	})
	if err != nil {
		return nil, fmt.Errorf("storage: GetBestTip: %w", err)
	}
	return tip, nil
}

// saveUndoTx is the transaction-scoped implementation shared by
// DB.SaveUndo and WriteTx.SaveUndo.
func saveUndoTx(tx *bbolt.Tx, undo *types.BlockUndo) error {
	data, err := marshalJSON(undo)
	if err != nil {
		return err
	}
	return tx.Bucket(bucketUndo).Put(undo.BlockHash[:], data)
}

// SaveUndo persists the undo record for a block keyed by its hash.
func (db *DB) SaveUndo(undo *types.BlockUndo) error {
	return db.bolt.Update(func(tx *bbolt.Tx) error {
		return saveUndoTx(tx, undo)
	})
}

// GetUndo retrieves the undo record for a block. Returns (nil, nil) when not found.
func (db *DB) GetUndo(hash types.Hash32) (*types.BlockUndo, error) {
	var undo *types.BlockUndo
	err := db.bolt.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket(bucketUndo).Get(hash[:])
		if data == nil {
			return nil
		}
		undo = new(types.BlockUndo)
		return unmarshalJSON(data, undo)
	})
	if err != nil {
		return nil, fmt.Errorf("storage: GetUndo %s: %w", hash, err)
	}
	return undo, nil
}
