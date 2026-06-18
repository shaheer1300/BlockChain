package consensus

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/crypto"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// maxFutureSeconds is the maximum number of seconds a block timestamp may
// exceed the current wall-clock time before the block is rejected as
// premature. Matches the Bitcoin convention of 2 hours.
const maxFutureSeconds int64 = 2 * 3600

// Sentinel errors returned by ValidateBlock. Callers use errors.Is to
// distinguish rejection reasons without parsing error strings.
var (
	// ErrNoTransactions is returned when a block contains no transactions.
	ErrNoTransactions = errors.New("consensus: block has no transactions")

	// ErrNoCoinbase is returned when the block's first transaction is not a
	// coinbase, or when the coinbase has no outputs.
	ErrNoCoinbase = errors.New("consensus: first transaction is not a coinbase")

	// ErrMultipleCoinbases is returned when any transaction after index 0
	// is also a coinbase.
	ErrMultipleCoinbases = errors.New("consensus: block contains multiple coinbase transactions")

	// ErrBadMerkleRoot is returned when the Merkle root computed from the
	// block's transactions does not match block.Header.MerkleRoot.
	ErrBadMerkleRoot = errors.New("consensus: merkle root mismatch")

	// ErrBadPoW is returned when the block hash does not satisfy the
	// required number of leading zero hex nibbles.
	ErrBadPoW = errors.New("consensus: block hash does not satisfy proof-of-work target")

	// ErrBadPrevHash is returned when block.Header.PrevHash differs from
	// the hash of the supplied parent header.
	ErrBadPrevHash = errors.New("consensus: previous-block hash mismatch")

	// ErrBadTimestamp is returned when the block timestamp is not strictly
	// greater than the parent's timestamp, or is more than maxFutureSeconds
	// seconds ahead of the supplied wall-clock time.
	ErrBadTimestamp = errors.New("consensus: block timestamp is invalid")

	// ErrCoinbaseOverpay is returned when the coinbase transaction claims
	// more value than the block subsidy plus the sum of transaction fees.
	ErrCoinbaseOverpay = errors.New("consensus: coinbase value exceeds block subsidy plus transaction fees")
)

// BlockValidationResult carries the outcome of a successful ValidateBlock
// call. It is non-nil only when ValidateBlock returns nil.
type BlockValidationResult struct {
	// TotalFees is the sum of fees from all non-coinbase transactions in
	// the validated block (inputs_total − outputs_total across all regular txs).
	TotalFees types.Amount
}

// ValidateBlock validates block against its parent header and the UTXO set
// represented by baseView. The block is not connected to the chain during
// this call — baseView must reflect the chain state immediately before the
// block would be applied.
//
// Validation order:
//  1. Header chain-linkage (PrevHash, Timestamp vs. parent, Timestamp vs. wall clock).
//  2. Transaction list structure (non-empty, coinbase at index 0, no extra coinbases).
//  3. Merkle root (recomputed from TxID of every transaction).
//  4. Proof-of-work (block hash satisfies leading-zero-nibble requirement).
//  5. Each non-coinbase transaction validated via ValidateTx against an
//     OverlayUTXOView that reflects intra-block UTXO changes.
//  6. Coinbase value ≤ subsidy + totalFees.
//
// Parameters:
//   - parent: the header of the block this block extends. Pass nil for the
//     genesis block (skips PrevHash and Timestamp-vs-parent checks).
//   - baseView: UTXO state just before this block; never mutated by this call.
//   - height: block height this block would occupy (passed to UTXO records;
//     reserved for future coinbase-maturity enforcement).
//   - subsidy: the block reward for this height (e.g., 50 coins).
//   - powNibbles: required number of leading zero hex characters in the block
//     hash; pass 0 to skip the check (useful in unit tests).
//   - now: current wall-clock time as a Unix timestamp (seconds). In
//     production, pass time.Now().Unix().
func ValidateBlock(
	block *types.Block,
	parent *types.BlockHeader,
	baseView UTXOView,
	height uint32,
	subsidy types.Amount,
	powNibbles int,
	now int64,
) (*BlockValidationResult, error) {

	// ── 1. Header chain-linkage checks ───────────────────────────────
	if parent != nil {
		if block.Header.PrevHash != parent.BlockHash() {
			return nil, fmt.Errorf("%w: want %s, got %s",
				ErrBadPrevHash, parent.BlockHash(), block.Header.PrevHash)
		}
		if block.Header.Timestamp <= parent.Timestamp {
			return nil, fmt.Errorf("%w: block timestamp %d must be strictly greater than parent %d",
				ErrBadTimestamp, block.Header.Timestamp, parent.Timestamp)
		}
	}
	if block.Header.Timestamp > now+maxFutureSeconds {
		return nil, fmt.Errorf("%w: timestamp %d is more than %d seconds ahead of wall clock %d",
			ErrBadTimestamp, block.Header.Timestamp, maxFutureSeconds, now)
	}

	// ── 2. Transaction list structural checks ────────────────────────
	if len(block.Transactions) == 0 {
		return nil, ErrNoTransactions
	}
	if !block.Transactions[0].IsCoinbase() {
		return nil, fmt.Errorf("%w: transaction at index 0", ErrNoCoinbase)
	}
	for i := 1; i < len(block.Transactions); i++ {
		if block.Transactions[i].IsCoinbase() {
			return nil, fmt.Errorf("%w: transaction at index %d", ErrMultipleCoinbases, i)
		}
	}

	// ── 3. Merkle root check ─────────────────────────────────────────
	leaves := make([]types.Hash32, len(block.Transactions))
	for i := range block.Transactions {
		leaves[i] = block.Transactions[i].TxID()
	}
	if computed := crypto.MerkleRoot(leaves); computed != block.Header.MerkleRoot {
		return nil, fmt.Errorf("%w: computed %s, header has %s",
			ErrBadMerkleRoot, computed, block.Header.MerkleRoot)
	}

	// ── 4. Proof-of-work check ───────────────────────────────────────
	if err := checkPoW(block.Header.BlockHash(), powNibbles); err != nil {
		return nil, err
	}

	// ── 5. Validate non-coinbase transactions with overlay ────────────
	// OverlayUTXOView reflects UTXOs created and spent within the block
	// without touching the underlying database or the caller's baseView.
	overlay := NewOverlayUTXOView(baseView)
	var totalFees types.Amount

	for i := 1; i < len(block.Transactions); i++ {
		tx := &block.Transactions[i]
		res, err := ValidateTx(tx, overlay)
		if err != nil {
			return nil, fmt.Errorf("consensus: block tx at index %d: %w", i, err)
		}

		// Accumulate fees (overflow-safe).
		next, feeErr := totalFees.SafeAdd(res.Fee)
		if feeErr != nil {
			return nil, fmt.Errorf("%w: total fee overflow at tx index %d", ErrFeeOverflow, i)
		}
		totalFees = next

		// Apply this transaction's UTXO changes to the overlay so that
		// later transactions in the same block can spend its outputs
		// (and cannot re-spend its inputs).
		txID := tx.TxID()
		for _, in := range tx.Inputs {
			overlay.SpendUTXO(in.PrevOut)
		}
		for idx, out := range tx.Outputs {
			op := types.OutPoint{TxID: txID, Index: uint32(idx)}
			overlay.AddUTXO(&types.UTXO{
				OutPoint: op,
				Output:   out,
				Height:   height,
				Coinbase: false,
			})
		}
	}

	// ── 6. Coinbase value check ───────────────────────────────────────
	coinbase := &block.Transactions[0]
	cbValue, err := coinbase.TotalOutputValue()
	if err != nil {
		// types.ErrNoOutputs propagates when the coinbase has no outputs.
		return nil, fmt.Errorf("%w: coinbase output sum: %v", ErrNoCoinbase, err)
	}
	maxCbValue, err := subsidy.SafeAdd(totalFees)
	if err != nil {
		return nil, fmt.Errorf("%w: subsidy plus fees overflow", ErrFeeOverflow)
	}
	if cbValue > maxCbValue {
		return nil, fmt.Errorf("%w: coinbase claims %d, maximum allowed is %d (subsidy %d + fees %d)",
			ErrCoinbaseOverpay, cbValue, maxCbValue, subsidy, totalFees)
	}

	return &BlockValidationResult{TotalFees: totalFees}, nil
}

// checkPoW verifies that hash has at least nibbles leading zero hex characters.
// nibbles == 0 skips the check entirely (any hash passes).
func checkPoW(hash types.Hash32, nibbles int) error {
	if nibbles == 0 {
		return nil
	}
	hashHex := hex.EncodeToString(hash[:])
	if !strings.HasPrefix(hashHex, strings.Repeat("0", nibbles)) {
		return fmt.Errorf("%w: hash %s does not have %d leading zero hex nibbles",
			ErrBadPoW, hashHex, nibbles)
	}
	return nil
}
