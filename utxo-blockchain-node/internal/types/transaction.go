package types

import (
	"bytes"
	"errors"
	"fmt"
)

// CoinbaseInputIndex is the sentinel index used in a coinbase
// transaction's single input to indicate "no previous output". Matches
// Bitcoin's 0xFFFFFFFF convention.
const CoinbaseInputIndex uint32 = 0xFFFFFFFF

// ErrNoOutputs is returned by TotalOutputValue when a transaction has no
// outputs.
var ErrNoOutputs = errors.New("types: transaction has no outputs")

// OutPoint uniquely identifies a transaction output by the producing
// transaction's ID and the output's index within that transaction.
type OutPoint struct {
	TxID  Hash32 `json:"txid"`
	Index uint32 `json:"index"`
}

// IsCoinbaseSentinel reports whether op is the (ZeroHash, 0xFFFFFFFF)
// outpoint used by coinbase inputs.
func (op OutPoint) IsCoinbaseSentinel() bool {
	return op.TxID.IsZero() && op.Index == CoinbaseInputIndex
}

func (op OutPoint) canonicalEncode(e *canonicalEncoder) {
	e.writeHash(op.TxID)
	e.writeUint32(op.Index)
}

// TxInput consumes a previously-unspent transaction output. Signature
// and PubKey are kept as opaque byte slices in this layer; their
// structure and verification are owned by internal/crypto and
// internal/consensus.
type TxInput struct {
	PrevOut   OutPoint `json:"prev_out"`
	Signature []byte   `json:"signature"`
	PubKey    []byte   `json:"pubkey"`
}

func (in TxInput) canonicalEncode(e *canonicalEncoder) {
	in.PrevOut.canonicalEncode(e)
	e.writeVarBytes(in.Signature)
	e.writeVarBytes(in.PubKey)
}

// TxOutput assigns a value to a recipient address. The recipient
// representation is HASH160 of the receiving public key.
type TxOutput struct {
	Value     Amount  `json:"value"`
	Recipient Address `json:"recipient"`
}

func (out TxOutput) canonicalEncode(e *canonicalEncoder) {
	e.writeUint64(uint64(out.Value))
	e.writeAddress(out.Recipient)
}

// Transaction is the consensus transaction structure. Its canonical
// encoding is the preimage of TxID.
type Transaction struct {
	Version  uint32     `json:"version"`
	Inputs   []TxInput  `json:"inputs"`
	Outputs  []TxOutput `json:"outputs"`
	LockTime uint32     `json:"locktime"`
}

// CanonicalEncode returns the deterministic byte representation used as
// the preimage of TxID. The same Transaction always produces the same
// byte sequence on any platform.
func (t *Transaction) CanonicalEncode() ([]byte, error) {
	var buf bytes.Buffer
	e := newCanonicalEncoder(&buf)
	e.writeUint32(t.Version)
	e.writeLen(len(t.Inputs))
	for i := range t.Inputs {
		t.Inputs[i].canonicalEncode(e)
	}
	e.writeLen(len(t.Outputs))
	for i := range t.Outputs {
		t.Outputs[i].canonicalEncode(e)
	}
	e.writeUint32(t.LockTime)
	if err := e.Err(); err != nil {
		return nil, fmt.Errorf("types: canonical encode transaction: %w", err)
	}
	return buf.Bytes(), nil
}

// TxID returns the double-SHA-256 of the canonical encoding.
//
// The only failure mode of CanonicalEncode is a transaction whose
// length-prefixed fields exceed MaxCanonicalSliceLen (32 MiB). Such a
// transaction is unreachable from any legitimate code path — it cannot
// be propagated, mined, or stored. Panicking here keeps the API ergonomic
// for the 100% of callers that operate on valid transactions.
func (t *Transaction) TxID() Hash32 {
	encoded, err := t.CanonicalEncode()
	if err != nil {
		panic(fmt.Sprintf("types: TxID on un-encodable transaction: %v", err))
	}
	return doubleSHA256(encoded)
}

// IsCoinbase reports whether t is a coinbase transaction: exactly one
// input whose previous outpoint is the coinbase sentinel.
func (t *Transaction) IsCoinbase() bool {
	return len(t.Inputs) == 1 && t.Inputs[0].PrevOut.IsCoinbaseSentinel()
}

// TotalOutputValue sums the values of all outputs, returning
// ErrAmountOverflow on wrap or ErrNoOutputs if t has none.
func (t *Transaction) TotalOutputValue() (Amount, error) {
	if len(t.Outputs) == 0 {
		return 0, ErrNoOutputs
	}
	var total Amount
	for i := range t.Outputs {
		next, err := total.SafeAdd(t.Outputs[i].Value)
		if err != nil {
			return 0, err
		}
		total = next
	}
	return total, nil
}
