package types

// MempoolEntry is a transaction that has been accepted into the local
// mempool but not yet included in a block. FeeRate (fee per byte) is
// precomputed so the mempool's selection routine can sort entries in
// O(1) per comparison instead of recomputing on each look.
type MempoolEntry struct {
	Tx      Transaction `json:"tx"`
	TxID    Hash32      `json:"txid"`
	Fee     Amount      `json:"fee"`
	Size    uint32      `json:"size"`
	FeeRate uint64      `json:"fee_rate"`
	AddedAt int64       `json:"added_at"`
}
