package storage

import (
	"fmt"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
	bbolt "go.etcd.io/bbolt"
)

// SaveBlock persists a full block keyed by its hash.
func (db *DB) SaveBlock(block *types.Block) error {
	data, err := marshalJSON(block)
	if err != nil {
		return err
	}
	hash := block.BlockHash()
	return db.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketBlocks).Put(hash[:], data)
	})
}

// GetBlock retrieves a block by hash. Returns (nil, nil) when not found.
func (db *DB) GetBlock(hash types.Hash32) (*types.Block, error) {
	var block *types.Block
	err := db.bolt.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket(bucketBlocks).Get(hash[:])
		if data == nil {
			return nil
		}
		block = new(types.Block)
		return unmarshalJSON(data, block)
	})
	if err != nil {
		return nil, fmt.Errorf("storage: GetBlock %s: %w", hash, err)
	}
	return block, nil
}

// SaveHeader persists a block header keyed by its hash.
func (db *DB) SaveHeader(header *types.BlockHeader) error {
	data, err := marshalJSON(header)
	if err != nil {
		return err
	}
	hash := header.BlockHash()
	return db.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketHeaders).Put(hash[:], data)
	})
}

// GetHeader retrieves a block header by hash. Returns (nil, nil) when not found.
func (db *DB) GetHeader(hash types.Hash32) (*types.BlockHeader, error) {
	var header *types.BlockHeader
	err := db.bolt.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket(bucketHeaders).Get(hash[:])
		if data == nil {
			return nil
		}
		header = new(types.BlockHeader)
		return unmarshalJSON(data, header)
	})
	if err != nil {
		return nil, fmt.Errorf("storage: GetHeader %s: %w", hash, err)
	}
	return header, nil
}
