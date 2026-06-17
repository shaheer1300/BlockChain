package types

// UTXO is a live, unspent transaction output indexed by its outpoint.
// Height is the block height at which the producing transaction was
// confirmed; Coinbase records whether the producing transaction was a
// coinbase (used later to enforce coinbase maturity).
type UTXO struct {
	OutPoint OutPoint `json:"outpoint"`
	Output   TxOutput `json:"output"`
	Height   uint32   `json:"height"`
	Coinbase bool     `json:"coinbase"`
}

// SpentOutput captures the state of a UTXO at the moment it was spent so
// the chain manager can reverse a block during a reorg. It mirrors UTXO
// today but is kept distinct because its semantic role differs and the
// two are likely to diverge (e.g. SpentInBlockHeight).
type SpentOutput struct {
	OutPoint OutPoint `json:"outpoint"`
	Output   TxOutput `json:"output"`
	Height   uint32   `json:"height"`
	Coinbase bool     `json:"coinbase"`
}

// BlockUndo holds the data needed to disconnect a previously-connected
// block. Spent is appended in the order outputs were consumed during
// connection; the chain manager must iterate it in reverse when undoing.
type BlockUndo struct {
	BlockHash Hash32        `json:"block_hash"`
	Spent     []SpentOutput `json:"spent"`
}
