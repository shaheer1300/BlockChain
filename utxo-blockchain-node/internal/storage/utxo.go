package storage

import (
	"fmt"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
	bbolt "go.etcd.io/bbolt"
)

// putUTXOTx is the transaction-scoped implementation shared by DB.PutUTXO
// and WriteTx.PutUTXO.
func putUTXOTx(tx *bbolt.Tx, utxo *types.UTXO) error {
	data, err := marshalJSON(utxo)
	if err != nil {
		return err
	}
	key := outpointKey(utxo.OutPoint)
	return tx.Bucket(bucketUTXOs).Put(key[:], data)
}

// deleteUTXOTx is the transaction-scoped implementation shared by DB.DeleteUTXO
// and WriteTx.DeleteUTXO.
func deleteUTXOTx(tx *bbolt.Tx, op types.OutPoint) error {
	key := outpointKey(op)
	return tx.Bucket(bucketUTXOs).Delete(key[:])
}

// PutUTXO inserts or updates a UTXO entry, keyed by its OutPoint.
func (db *DB) PutUTXO(utxo *types.UTXO) error {
	return db.bolt.Update(func(tx *bbolt.Tx) error {
		return putUTXOTx(tx, utxo)
	})
}

// GetUTXO retrieves a UTXO by its OutPoint. Returns (nil, nil) when not found.
func (db *DB) GetUTXO(op types.OutPoint) (*types.UTXO, error) {
	var utxo *types.UTXO
	key := outpointKey(op)
	err := db.bolt.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket(bucketUTXOs).Get(key[:])
		if data == nil {
			return nil
		}
		utxo = new(types.UTXO)
		return unmarshalJSON(data, utxo)
	})
	if err != nil {
		return nil, fmt.Errorf("storage: GetUTXO: %w", err)
	}
	return utxo, nil
}

// DeleteUTXO removes a UTXO entry. It is a no-op if the OutPoint is not
// present in the database.
func (db *DB) DeleteUTXO(op types.OutPoint) error {
	return db.bolt.Update(func(tx *bbolt.Tx) error {
		return deleteUTXOTx(tx, op)
	})
}

// GetUTXOsByAddress performs a full scan of the UTXO bucket and returns
// every entry whose Recipient address matches addr. It is O(n) in the total
// UTXO set size; use for API queries only, not for consensus hot paths.
func (db *DB) GetUTXOsByAddress(addr types.Address) ([]*types.UTXO, error) {
	var results []*types.UTXO
	err := db.bolt.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketUTXOs).ForEach(func(_, v []byte) error {
			var utxo types.UTXO
			if err := unmarshalJSON(v, &utxo); err != nil {
				return err
			}
			if utxo.Output.Recipient == addr {
				u := utxo
				results = append(results, &u)
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("storage: GetUTXOsByAddress: %w", err)
	}
	return results, nil
}
