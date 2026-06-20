package consensus

import (
	"errors"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/crypto"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// Sentinel errors returned by ValidateTx. Callers use errors.Is to
// distinguish rejection reasons without parsing error strings.
var (
	// ErrCoinbaseNotAllowed is returned when a coinbase transaction is
	// submitted to ValidateTx. Coinbase validation is handled separately
	// by block validation.
	ErrCoinbaseNotAllowed = errors.New("consensus: coinbase transaction not allowed here")

	// ErrNoInputs is returned when a transaction has zero inputs.
	ErrNoInputs = errors.New("consensus: transaction has no inputs")

	// ErrNoOutputs is returned when a transaction has zero outputs.
	ErrNoOutputs = errors.New("consensus: transaction has no outputs")

	// ErrZeroOutput is returned when any output has a value of zero.
	ErrZeroOutput = errors.New("consensus: output value must be greater than zero")

	// ErrDuplicateInput is returned when the same OutPoint appears more
	// than once in the input list.
	ErrDuplicateInput = errors.New("consensus: duplicate input outpoint")

	// ErrMissingUTXO is returned when an input references an outpoint
	// that does not exist in the UTXO view.
	ErrMissingUTXO = errors.New("consensus: missing UTXO for input")

	// ErrInvalidSignature is returned when an input's signature fails
	// cryptographic verification.
	ErrInvalidSignature = errors.New("consensus: invalid input signature")

	// ErrOutputsExceedInputs is returned when the total output value
	// exceeds the total input value (no inflation outside coinbase).
	ErrOutputsExceedInputs = errors.New("consensus: total outputs exceed total inputs")

	// ErrFeeOverflow is returned when the fee calculation itself
	// overflows a uint64 (inputs - outputs would underflow; this path is
	// guarded by SafeSub).
	ErrFeeOverflow = errors.New("consensus: fee calculation overflow")
)

// TxValidationResult carries the outcome of a successful ValidateTx call.
// It is non-nil only when ValidateTx returns nil.
type TxValidationResult struct {
	// Fee is inputs_total - outputs_total, guaranteed >= 0.
	Fee types.Amount
}

// ValidateTx validates a non-coinbase transaction against view. It
// performs, in order:
//
//  1. Structural checks (no coinbase, has inputs, has outputs, no zero
//     outputs).
//  2. Duplicate-input detection.
//  3. UTXO existence check for every input.
//  4. Signature verification for every input against the referenced output.
//  5. Arithmetic: overflow-safe sum of inputs and outputs, plus the rule
//     that outputs ≤ inputs.
//
// On success it returns a non-nil *TxValidationResult containing the fee.
// On failure it returns a nil result and a non-nil error (one of the
// sentinel errors above, possibly wrapped with fmt.Errorf %w).
func ValidateTx(tx *types.Transaction, view UTXOView) (*TxValidationResult, error) {
	// ── 1. Structural checks ───────────────────────────────────────────
	if tx.IsCoinbase() {
		return nil, ErrCoinbaseNotAllowed
	}
	if len(tx.Inputs) == 0 {
		return nil, ErrNoInputs
	}
	if len(tx.Outputs) == 0 {
		return nil, ErrNoOutputs
	}
	for i, out := range tx.Outputs {
		if out.Value == 0 {
			return nil, fmt.Errorf("%w: output %d", ErrZeroOutput, i)
		}
	}

	// ── 2. Duplicate-input detection ──────────────────────────────────
	seen := make(map[types.OutPoint]struct{}, len(tx.Inputs))
	for i, in := range tx.Inputs {
		if _, dup := seen[in.PrevOut]; dup {
			return nil, fmt.Errorf("%w: input %d (%s:%d)",
				ErrDuplicateInput, i, in.PrevOut.TxID, in.PrevOut.Index)
		}
		seen[in.PrevOut] = struct{}{}
	}

	// ── 3. UTXO existence + 4. Signature verification ─────────────────
	var inputTotal types.Amount
	for i, in := range tx.Inputs {
		utxo, err := view.GetUTXO(in.PrevOut)
		if err != nil {
			return nil, fmt.Errorf("consensus: UTXO lookup input %d: %w", i, err)
		}
		if utxo == nil {
			return nil, fmt.Errorf("%w: input %d (%s:%d)",
				ErrMissingUTXO, i, in.PrevOut.TxID, in.PrevOut.Index)
		}

		// P2PKH ownership check: the compressed public key stored in the
		// input must hash (HASH160) to the recipient address in the UTXO.
		// This prevents an attacker from supplying a different key that
		// produces a valid self-consistent signature.
		pub, err := secp256k1.ParsePubKey(in.PubKey)
		if err != nil {
			return nil, fmt.Errorf("%w: input %d: parse pubkey: %v", ErrInvalidSignature, i, err)
		}
		if crypto.PubKeyToAddress(pub) != utxo.Output.Recipient {
			return nil, fmt.Errorf("%w: input %d: pubkey does not match UTXO recipient", ErrInvalidSignature, i)
		}

		if err := crypto.VerifyInput(tx, i); err != nil {
			return nil, fmt.Errorf("%w: input %d: %v", ErrInvalidSignature, i, err)
		}

		next, err := inputTotal.SafeAdd(utxo.Output.Value)
		if err != nil {
			// Summing inputs overflowed uint64 — impossible with any
			// legitimate UTXO set but guard it anyway.
			return nil, fmt.Errorf("%w: input sum at index %d", ErrFeeOverflow, i)
		}
		inputTotal = next
	}

	// ── 5. Output sum and balance check ───────────────────────────────
	outputTotal, err := tx.TotalOutputValue()
	if err != nil {
		// types.ErrAmountOverflow from TotalOutputValue.
		return nil, fmt.Errorf("%w: output sum: %v", ErrFeeOverflow, err)
	}

	if outputTotal > inputTotal {
		return nil, fmt.Errorf("%w: inputs %d, outputs %d",
			ErrOutputsExceedInputs, inputTotal, outputTotal)
	}

	fee, err := inputTotal.SafeSub(outputTotal)
	if err != nil {
		// Should be unreachable given the outputTotal <= inputTotal check above.
		return nil, fmt.Errorf("%w: %v", ErrFeeOverflow, err)
	}

	return &TxValidationResult{Fee: fee}, nil
}
