package api

import (
	"context"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// ── shared diagnostic types ───────────────────────────────────────────────────

// BlockSummary is a compact description of a block used by GET /blocks.
// Full block content is available via GET /blocks/{hash}.
type BlockSummary struct {
	Height     uint32       `json:"height"`
	Hash       types.Hash32 `json:"hash"`
	PrevHash   types.Hash32 `json:"prev_hash"`
	MerkleRoot types.Hash32 `json:"merkle_root"`
	Timestamp  int64        `json:"timestamp"`
	Nonce      uint64       `json:"nonce"`
	TxCount    int          `json:"tx_count"`
}

// ── demo (educational frontend) types ────────────────────────────────────────

// DemoWalletInfo is a single wallet entry returned by GET /demo/wallets.
// The address is hex-encoded; the private key is NOT exposed.
type DemoWalletInfo struct {
	Name    string        `json:"name"`
	Address types.Address `json:"address"`
	Balance types.Amount  `json:"balance"`
}

// DemoCreateWalletRequest is the body of POST /demo/wallets.
type DemoCreateWalletRequest struct {
	Name string `json:"name"`
}

// DemoTxRequest is the body of POST /demo/tx.
type DemoTxRequest struct {
	FromWallet string        `json:"from_wallet"`
	ToAddress  types.Address `json:"to_address"`
	Amount     types.Amount  `json:"amount"`
	Fee        types.Amount  `json:"fee"`
}

// DemoTxResult is the response from POST /demo/tx.
type DemoTxResult struct {
	TxID    types.Hash32 `json:"txid"`
	Inputs  []DemoInput  `json:"inputs"`
	Outputs []DemoOutput `json:"outputs"`
}

// DemoInput is a single selected UTXO shown in the tx-builder explanation.
type DemoInput struct {
	TxID   types.Hash32  `json:"txid"`
	Index  uint32        `json:"index"`
	Value  types.Amount  `json:"value"`
	Owner  types.Address `json:"owner"`
}

// DemoOutput is a single tx output as planned by the demo builder.
type DemoOutput struct {
	Value     types.Amount  `json:"value"`
	Recipient types.Address `json:"recipient"`
	Purpose   string        `json:"purpose"` // "payment" | "change"
}

// DemoDoubleSpendRequest is the body of POST /demo/double-spend.
// The handler builds two transactions both spending the same UTXOs but
// paying different recipients, submits them in order, and returns the
// outcome of each so the UI can visualise the rejection.
type DemoDoubleSpendRequest struct {
	FromWallet  string        `json:"from_wallet"`
	ToAddressA  types.Address `json:"to_address_a"`
	ToAddressB  types.Address `json:"to_address_b"`
	Amount      types.Amount  `json:"amount"`
	Fee         types.Amount  `json:"fee"`
}

// DemoDoubleSpendResult describes the outcome of each of the two
// conflicting transactions.
type DemoDoubleSpendResult struct {
	First  DemoSubmitOutcome `json:"first"`
	Second DemoSubmitOutcome `json:"second"`
}

// DemoSubmitOutcome captures the result of attempting to submit one tx.
type DemoSubmitOutcome struct {
	TxID     types.Hash32 `json:"txid"`
	Accepted bool         `json:"accepted"`
	Reason   string       `json:"reason,omitempty"`
}

// DemoMineRequest is the optional body of POST /demo/mine.
// When MinerWallet is non-empty the coinbase pays that wallet's address;
// otherwise the configured MINER_ADDRESS is used.
type DemoMineRequest struct {
	MinerWallet string `json:"miner_wallet"`
}

// DemoStateResponse is a one-shot snapshot powering the GET /demo/state
// endpoint — convenient for the UI to refresh after every action.
type DemoStateResponse struct {
	Status   StatusResult     `json:"status"`
	Wallets  []DemoWalletInfo `json:"wallets"`
	UTXOs    []*types.UTXO    `json:"utxos"`
	Mempool  []*types.MempoolEntry `json:"mempool"`
	Blocks   []BlockSummary   `json:"blocks"`
}

// ── interfaces ────────────────────────────────────────────────────────────────

// DiagnosticServices exposes read-only queries that are useful for clients
// (CLIs, dashboards, frontends) but not strictly required by consensus or
// the gossip protocol.
type DiagnosticServices interface {
	// GetAllUTXOs returns every unspent output in the chain.
	GetAllUTXOs() ([]*types.UTXO, error)

	// ListBlocks returns the most recent blocks (newest first), up to limit.
	ListBlocks(limit int) ([]BlockSummary, error)
}

// DemoServices powers the educational frontend. Implementations live in
// internal/node and are wired up by Server.New. Demo endpoints are exposed
// only when this interface is non-nil.
type DemoServices interface {
	// DemoState bundles every dataset the UI needs in a single call.
	DemoState() (*DemoStateResponse, error)

	// DemoListWallets returns the current wallets with computed balances.
	DemoListWallets() ([]DemoWalletInfo, error)

	// DemoCreateWallet generates a new keypair under name.
	DemoCreateWallet(name string) (DemoWalletInfo, error)

	// DemoBuildAndSubmitTx greedily selects UTXOs owned by FromWallet,
	// builds a signed transaction paying ToAddress, and submits it to
	// the mempool. The returned DemoTxResult mirrors what the UI should
	// display.
	DemoBuildAndSubmitTx(req DemoTxRequest) (*DemoTxResult, error)

	// DemoDoubleSpend builds two transactions spending the same UTXOs and
	// submits them in order, returning the outcome of each.
	DemoDoubleSpend(req DemoDoubleSpendRequest) (*DemoDoubleSpendResult, error)

	// DemoMine mines one block. When MinerWallet is supplied the coinbase
	// pays that wallet. When the chain is uninitialised the call doubles
	// as InitGenesis using MinerWallet as the genesis miner.
	DemoMine(ctx context.Context, req DemoMineRequest) (*MineResult, error)

	// DemoReset clears the mempool, drops on-disk chain data, and forgets
	// every demo wallet. The caller should restart the node-services
	// process afterwards if it relies on long-lived references.
	DemoReset() error
}
