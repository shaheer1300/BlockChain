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
