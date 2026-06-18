package storage

import (
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
	bbolt "go.etcd.io/bbolt"
)

// WriteTx is a write-transaction handle that groups multiple storage
// operations into a single atomic bbolt write transaction. If any method
// returns an error, or if the function passed to DB.Update returns an error,
// bbolt rolls back every write made within that call. On success bbolt
// commits all changes in one fsync.
//
// WriteTx is not safe for concurrent use and must not escape the function
// passed to DB.Update.
type WriteTx struct {
	tx *bbolt.Tx
}

// Update executes fn inside a single bbolt write transaction. If fn returns
// a non-nil error, bbolt rolls back all changes; otherwise it commits them
// atomically. The WriteTx passed to fn is valid only for the duration of
// the call.
func (db *DB) Update(fn func(*WriteTx) error) error {
	return db.bolt.Update(func(tx *bbolt.Tx) error {
		return fn(&WriteTx{tx: tx})
	})
}

// SaveBlock persists a full block within the transaction.
func (w *WriteTx) SaveBlock(block *types.Block) error {
	return saveBlockTx(w.tx, block)
}

// SaveHeader persists a block header within the transaction.
func (w *WriteTx) SaveHeader(header *types.BlockHeader) error {
	return saveHeaderTx(w.tx, header)
}

// PutUTXO inserts or updates a UTXO entry within the transaction.
func (w *WriteTx) PutUTXO(utxo *types.UTXO) error {
	return putUTXOTx(w.tx, utxo)
}

// DeleteUTXO removes a UTXO entry within the transaction. It is a no-op if
// the OutPoint is absent.
func (w *WriteTx) DeleteUTXO(op types.OutPoint) error {
	return deleteUTXOTx(w.tx, op)
}

// SaveBlockIndex persists a BlockIndex entry within the transaction.
func (w *WriteTx) SaveBlockIndex(idx *types.BlockIndex) error {
	return saveBlockIndexTx(w.tx, idx)
}

// SetActiveHash records the canonical block hash at a given chain height
// within the transaction.
func (w *WriteTx) SetActiveHash(height uint32, hash types.Hash32) error {
	return setActiveHashTx(w.tx, height, hash)
}

// SetBestTip persists the current best chain tip within the transaction.
func (w *WriteTx) SetBestTip(tip *types.ChainTip) error {
	return setBestTipTx(w.tx, tip)
}

// SaveUndo persists the undo record for a block within the transaction.
func (w *WriteTx) SaveUndo(undo *types.BlockUndo) error {
	return saveUndoTx(w.tx, undo)
}

// GetUTXO reads a UTXO within the transaction. Reads are consistent with any
// writes already made in the same transaction — a PutUTXO followed by GetUTXO
// on the same outpoint returns the just-written value, and a DeleteUTXO
// followed by GetUTXO returns nil. This is used by connectBlock to read UTXOs
// for undo-record construction before deleting them.
func (w *WriteTx) GetUTXO(op types.OutPoint) (*types.UTXO, error) {
	key := outpointKey(op)
	data := w.tx.Bucket(bucketUTXOs).Get(key[:])
	if data == nil {
		return nil, nil
	}
	var utxo types.UTXO
	if err := unmarshalJSON(data, &utxo); err != nil {
		return nil, err
	}
	return &utxo, nil
}

// GetBlock reads a full block within the transaction. Returns (nil, nil) when
// not found. Used by disconnectBlockTx during reorg to load blocks for UTXO
// reversal and undo reconstruction.
func (w *WriteTx) GetBlock(hash types.Hash32) (*types.Block, error) {
	data := w.tx.Bucket(bucketBlocks).Get(hash[:])
	if data == nil {
		return nil, nil
	}
	var block types.Block
	if err := unmarshalJSON(data, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

// GetUndo reads the undo record for blockHash within the transaction.
// Returns (nil, nil) when not found. Used by disconnectBlockTx during reorg.
func (w *WriteTx) GetUndo(hash types.Hash32) (*types.BlockUndo, error) {
	data := w.tx.Bucket(bucketUndo).Get(hash[:])
	if data == nil {
		return nil, nil
	}
	var undo types.BlockUndo
	if err := unmarshalJSON(data, &undo); err != nil {
		return nil, err
	}
	return &undo, nil
}

// DeleteActiveHash removes the canonical block hash entry at height from the
// active-chain index. Called during block disconnection when a reorg removes
// a height from the active chain before the new branch occupies it.
func (w *WriteTx) DeleteActiveHash(height uint32) error {
	key := heightKey(height)
	return w.tx.Bucket(bucketActiveChain).Delete(key[:])
}
