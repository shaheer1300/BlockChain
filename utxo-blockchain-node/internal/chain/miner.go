package chain

import (
	"context"
	"errors"
	"fmt"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/crypto"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// MineBlock builds a candidate block extending parent and searches for a nonce
// whose resulting block hash satisfies the proof-of-work target. When
// powNibbles is 0 the very first nonce (0) always wins, making mining instant
// in development and tests.
//
// The coinbase transaction pays coinbaseValue to miner. extraTxs are appended
// after the coinbase in transaction order; the caller is responsible for
// ensuring they are already consensus-validated.
//
// MineBlock respects ctx — when the context is cancelled it returns a wrapped
// ctx.Err(). If the full uint64 nonce space is exhausted without a solution it
// returns an error (effectively impossible with any practical PoW target).
func MineBlock(
	ctx context.Context,
	parent *types.BlockHeader,
	miner types.Address,
	coinbaseValue types.Amount,
	extraTxs []types.Transaction,
	powNibbles int,
	now int64,
) (*types.Block, error) {
	// Build the ordered transaction list: coinbase first.
	txs := make([]types.Transaction, 0, 1+len(extraTxs))
	txs = append(txs, buildCoinbase(miner, coinbaseValue))
	txs = append(txs, extraTxs...)

	// Compute the Merkle root once — it is invariant across nonce iterations
	// because it depends only on TxIDs, which do not include the header.
	leaves := make([]types.Hash32, len(txs))
	for i := range txs {
		leaves[i] = txs[i].TxID()
	}

	var prevHash types.Hash32
	if parent != nil {
		prevHash = parent.BlockHash()
	}

	header := types.BlockHeader{
		Version:    1,
		PrevHash:   prevHash,
		MerkleRoot: crypto.MerkleRoot(leaves),
		Timestamp:  now,
		Nonce:      0,
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("chain: MineBlock: %w", ctx.Err())
		default:
		}

		if powSatisfied(header.BlockHash(), powNibbles) {
			return &types.Block{Header: header, Transactions: txs}, nil
		}

		if header.Nonce == ^uint64(0) {
			return nil, errors.New("chain: MineBlock: nonce space exhausted without valid solution")
		}
		header.Nonce++
	}
}

// buildCoinbase creates the single coinbase transaction paying value to miner.
// The input uses the sentinel outpoint (ZeroHash, CoinbaseInputIndex) as per
// the Bitcoin convention codified in types.CoinbaseInputIndex.
func buildCoinbase(miner types.Address, value types.Amount) types.Transaction {
	return types.Transaction{
		Version: 1,
		Inputs: []types.TxInput{{
			PrevOut: types.OutPoint{
				TxID:  types.ZeroHash,
				Index: types.CoinbaseInputIndex,
			},
		}},
		Outputs: []types.TxOutput{{Value: value, Recipient: miner}},
	}
}

// powSatisfied reports whether hash has at least nibbles leading zero hex
// nibbles. It uses a byte-level comparison because this runs in the innermost
// nonce loop; converting to hex and comparing string prefixes is ~5× slower.
//
//   - nibbles == 0: always returns true (no PoW constraint).
//   - Even nibbles: check nibbles/2 leading zero bytes.
//   - Odd nibbles: additionally require the high nibble of the next byte to be 0.
func powSatisfied(hash types.Hash32, nibbles int) bool {
	if nibbles == 0 {
		return true
	}
	fullBytes := nibbles / 2
	for i := 0; i < fullBytes && i < types.HashSize; i++ {
		if hash[i] != 0 {
			return false
		}
	}
	// For an odd nibble count the high nibble of byte [fullBytes] must be zero.
	if nibbles%2 == 1 && fullBytes < types.HashSize {
		if hash[fullBytes]>>4 != 0 {
			return false
		}
	}
	return true
}
