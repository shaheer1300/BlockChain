package crypto

import (
	"errors"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ErrInvalidSignature is returned when a signature fails to parse or verify.
var ErrInvalidSignature = errors.New("crypto: invalid signature")

// SigHashPreimage builds the canonical byte string that is hashed to
// produce the signature digest for input at index inputIndex.
//
// Preimage layout (little-endian binary, matching the canonical encoder):
//
//	tx.Version        uint32
//	len(tx.Inputs)    uint32
//	for each input:
//	    prevOut.TxID    [32]byte
//	    prevOut.Index   uint32
//	    (signature and pubkey fields are excluded from the preimage)
//	len(tx.Outputs)   uint32
//	for each output:
//	    value           uint64
//	    recipient       [20]byte
//	tx.LockTime       uint32
//	inputIndex        uint32   (disambiguates inputs within the same tx)
//
// The preimage intentionally excludes the Signature and PubKey fields of
// every input so that signing and verifying do not depend on each other's
// output.
func SigHashPreimage(tx *types.Transaction, inputIndex int) ([]byte, error) {
	if inputIndex < 0 || inputIndex >= len(tx.Inputs) {
		return nil, fmt.Errorf("crypto: SigHashPreimage: input index %d out of range [0,%d)",
			inputIndex, len(tx.Inputs))
	}
	// The preimage has the same structure as CanonicalEncode but with
	// signature/pubkey replaced by the index suffix. Rather than
	// duplicating the encoder internals we encode a scratch transaction
	// that has zeroed Signature/PubKey fields, then append the index.
	scratch := types.Transaction{
		Version:  tx.Version,
		Inputs:   make([]types.TxInput, len(tx.Inputs)),
		Outputs:  tx.Outputs,
		LockTime: tx.LockTime,
	}
	for i, in := range tx.Inputs {
		scratch.Inputs[i] = types.TxInput{
			PrevOut:   in.PrevOut,
			Signature: nil,
			PubKey:    nil,
		}
	}
	base, err := scratch.CanonicalEncode()
	if err != nil {
		return nil, fmt.Errorf("crypto: SigHashPreimage: encode: %w", err)
	}

	// Append the input index (4-byte little-endian) so inputs within the
	// same transaction always produce distinct preimages.
	preimage := make([]byte, len(base)+4)
	copy(preimage, base)
	preimage[len(base)] = byte(inputIndex)
	preimage[len(base)+1] = byte(inputIndex >> 8)
	preimage[len(base)+2] = byte(inputIndex >> 16)
	preimage[len(base)+3] = byte(inputIndex >> 24)
	return preimage, nil
}

// SignInput signs the preimage for inputIndex with priv using ECDSA on
// secp256k1 with a double-SHA-256 digest (standard Bitcoin signing hash).
// Returns the DER-encoded signature.
func SignInput(tx *types.Transaction, inputIndex int, priv *PrivateKey) ([]byte, error) {
	preimage, err := SigHashPreimage(tx, inputIndex)
	if err != nil {
		return nil, err
	}
	hash := DoubleSHA256(preimage)
	sig := ecdsa.Sign(priv, hash[:])
	return sig.Serialize(), nil
}

// VerifyInput verifies the DER-encoded signature and uncompressed/compressed
// public key bytes stored in tx.Inputs[inputIndex] against the canonical
// preimage. Returns nil on success, ErrInvalidSignature (or a wrapped error)
// on failure.
func VerifyInput(tx *types.Transaction, inputIndex int) error {
	if inputIndex < 0 || inputIndex >= len(tx.Inputs) {
		return fmt.Errorf("crypto: VerifyInput: input index %d out of range", inputIndex)
	}
	in := tx.Inputs[inputIndex]

	sig, err := ecdsa.ParseDERSignature(in.Signature)
	if err != nil {
		return fmt.Errorf("%w: parse DER signature: %v", ErrInvalidSignature, err)
	}

	pub, err := secp256k1.ParsePubKey(in.PubKey)
	if err != nil {
		return fmt.Errorf("%w: parse public key: %v", ErrInvalidSignature, err)
	}

	preimage, err := SigHashPreimage(tx, inputIndex)
	if err != nil {
		return err
	}
	hash := DoubleSHA256(preimage)

	if !sig.Verify(hash[:], pub) {
		return fmt.Errorf("%w: signature does not match", ErrInvalidSignature)
	}
	return nil
}

// ensure the secp256k1 package is referenced so go mod tidy retains it.
var _ *secp256k1.FieldVal
