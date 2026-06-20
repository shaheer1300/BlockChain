# Testing Guide

This document describes the testing strategy, test layout, and testing patterns used in the UTXO blockchain node.

## Overview

The project uses Go's built-in testing framework (`testing` package) with **table-driven tests** and **unit tests colocated with source code**. All tests are in `*_test.go` files within their respective packages.

**Test Statistics (after Milestone 12):**
- Total tests: 219
- All tests pass: ✅
- No external testing framework (stdlib only)
- Tests run with: `go test ./...`

---

## Test Layout

### Unit Tests (Colocated)

Each package has tests in the same directory:

```
internal/
├── types/
│   ├── hash.go
│   ├── hash_test.go
│   ├── transaction.go
│   ├── transaction_test.go
│   └── ...
├── crypto/
│   ├── hash.go
│   ├── keys.go
│   ├── sign.go
│   └── crypto_test.go
├── storage/
│   ├── db.go
│   ├── blocks.go
│   ├── storage_test.go
│   └── ...
└── ...
```

### Integration Tests

Large scenario tests that exercise multiple packages (chain reorgs, mempool validation, consensus rules) are placed in `*_test.go` files within the relevant package.

Example: `internal/chain/reorg_test.go` tests chain reorganization across storage, consensus, and mempool.

---

## Test Execution

### Run all tests
```bash
go test ./...
```

### Run tests for a specific package
```bash
go test ./internal/types
go test ./internal/chain
go test ./internal/p2p
```

### Run with verbose output
```bash
go test ./... -v
```

### Run tests excluding vendor
```bash
go test ./internal/... ./cmd/...
```

### Count total passing tests
```bash
go test ./... -v 2>&1 | Select-String "--- PASS" | Measure-Object | Select-Object Count
```

---

## Testing Patterns

### Table-Driven Tests

Used for testing multiple input/output combinations:

```go
func TestTxIDDeterminism(t *testing.T) {
    tests := []struct {
        name string
        tx   *types.Transaction
        want types.Hash32
    }{
        {
            name: "coinbase",
            tx: &types.Transaction{
                Version: 1,
                Inputs:  []types.TxInput{...},
                Outputs: []types.TxOutput{...},
            },
            want: ...,
        },
        // more test cases...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := tt.tx.TxID()
            if got != tt.want {
                t.Errorf("TxID() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Storage Tests (Temporary Directories)

Database tests use temporary directories that are cleaned up after the test:

```go
func TestDB_PutGetUTXO(t *testing.T) {
    dir := t.TempDir() // Go 1.15+, auto-cleanup
    db, err := storage.Open(dir)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer db.Close()
    
    // Test operations...
}
```

### Consensus Tests (In-Memory UTXO Views)

Consensus validation uses in-memory UTXO views to avoid disk I/O:

```go
func TestValidateTx_ValidP2PKH(t *testing.T) {
    utxoView := consensus.MapUTXOView{
        outpoint: &types.UTXO{Output: txOutput},
    }
    
    err := consensus.ValidateTx(tx, utxoView)
    if err != nil {
        t.Errorf("ValidateTx: %v", err)
    }
}
```

### Test Helpers

Common test utilities are factored into helper functions to reduce duplication:

```go
func sampleTx(value types.Amount) *types.Transaction {
    return &types.Transaction{
        Version: 1,
        Inputs: []types.TxInput{...},
        Outputs: []types.TxOutput{{Value: value, Recipient: addr}},
    }
}

func sampleBlock(height uint32, prevHash types.Hash32) *types.Block {
    return &types.Block{
        Header: types.BlockHeader{...},
        Transactions: []types.Transaction{...},
    }
}
```

---

## Test Coverage by Package

### internal/types (40 tests)

**Tested concepts:**
- Hash primitives (Hash32, Address) serialization and deserialization
- Canonical encoding determinism (binary, little-endian)
- TxID calculation: changes when tx content changes
- BlockHash calculation: changes when header changes
- OutPoint, TxInput, TxOutput structures
- UTXO, SpentOutput, BlockUndo structures
- BlockIndex with accumulated work (big.Int as decimal string)
- MempoolEntry fee and fee rate calculations

**Key invariants verified:**
- Deterministic: same input always produces same output
- Idempotent: encoding/decoding round-trip preserves data
- Canonical: all hashes use double-SHA256
- Immutable: no accidental data loss

### internal/crypto (21 tests)

**Tested concepts:**
- Hash functions (SHA256, RIPEMD160, double-SHA256)
- Merkle root calculation and verification
- ECDSA signature generation and verification
- Key pair generation (secp256k1)
- Deterministic nonce (RFC 6979)

**Key invariants verified:**
- Hash determinism: same input → same output
- Hash avalanche: small input change → completely different hash
- Signature validity: valid signature passes verification
- Signature failure: mutated message fails verification

### internal/storage (18 tests)

**Tested concepts:**
- Database initialization and bucket creation
- Block storage and retrieval
- Block header storage and retrieval
- UTXO put/get/delete operations
- Block index storage (with ChainTip)
- Undo record storage
- Mempool entry storage
- Persistence: data survives close/reopen cycle

**Key invariants verified:**
- ACID: all operations atomic within bbolt tx
- Collision-free: multiple keys don't overwrite each other
- Persistence: data survives process restart
- Not found: missing data returns nil not error

### internal/consensus (32 tests)

**Tested concepts:**
- Transaction validation rules (P2PKH ownership, output sum, input/output counts)
- Block validation rules (6-stage: merkle, subsidy, timestamp, POW, coinbase, double-spend)
- Coinbase rules: only in first tx, correct subsidy amount
- Double-spend detection: both in-block and with mempool
- Fee calculation and sanity checks

**Key invariants verified:**
- Valid tx passes all checks
- Invalid tx fails with specific error (not generic)
- Coinbase overpay rejected
- Double-spend rejected
- Malformed blocks rejected

### internal/chain (30 tests)

**Tested concepts:**
- Genesis block initialization
- Linear block import (extend active chain)
- Invalid parent rejection (orphan handling)
- UTXO updates on block connect
- Undo records on block connect
- Reorg from shorter chain to heavier side chain
- Fork point detection (common ancestor)
- Atomic block-to-UTXO synchronization
- Work calculation and comparison
- Idempotent reorg: same reorg twice → same result

**Key invariants verified:**
- Linear extend: simple case works
- Reorg trigger: automatic on heavier branch
- Atomicity: disconnect/connect together or not at all
- Fork detection: correct common ancestor
- Work accumulation: correct total work calculation

### internal/mempool (14 tests)

**Tested concepts:**
- Accept valid transaction
- Reject duplicate transaction (by TxID)
- Reject mempool double-spend (conflicting input)
- Remove mined transactions on block connect
- Revalidation after reorg (re-add displaced txs)
- Eviction when full
- Fee rate sorting

**Key invariants verified:**
- No duplicate TxIDs in mempool
- No conflicting inputs (UTXO spent twice)
- Mempool cleared of mined txs
- Mempool revalidated after reorg

### internal/api (29 tests)

**Tested concepts:**
- Health endpoint returns node info
- Status endpoint returns chain height/tip
- GetBlock endpoint retrieves block by hash
- GetUTXOs endpoint lists UTXOs by address
- GetBalance endpoint computes balance
- GetMempool endpoint lists pending txs
- GetPeers endpoint lists connected peers
- SubmitTx endpoint accepts transaction
- P2P endpoints (POST /p2p/tx, POST /p2p/block)
- Error responses (400 bad request, 404 not found, 503 unavailable)
- Content-Type validation (application/json)
- JSON codec round-trip

**Key invariants verified:**
- All endpoints respond with 200 OK on success
- Invalid JSON returns 400 Bad Request
- Missing data returns 404 Not Found
- No service returns 503 Service Unavailable
- All responses are application/json

### internal/p2p (17 tests)

**Tested concepts:**
- Broadcast reaches all configured peers
- Duplicate suppression (seen-cache)
- Graceful timeout handling (peer down)
- Callback invocation on receipt
- Re-broadcast of received messages
- Suppression of re-broadcast on error (orphan)
- HTTP endpoint validity (204 No Content)
- Invalid JSON rejection

**Key invariants verified:**
- All peers receive broadcast
- Duplicate not sent twice
- Peer timeout doesn't crash node
- Received message propagates to callbacks
- Invalid block doesn't re-broadcast

---

## Test Debugging

### Run a single test
```bash
go test -run TestTxIDDeterminism ./internal/types
```

### Run tests matching a pattern
```bash
go test -run "TestValidateTx" ./internal/consensus
```

### Enable verbose logging
```bash
go test ./... -v
```

### Get test coverage (by line)
```bash
go test ./... -cover
```

### Get detailed coverage report
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Run with race detector
```bash
go test -race ./...
```

---

## Test Maintenance

### When adding a new feature:
1. Add test cases covering the happy path
2. Add test cases covering error paths
3. Update relevant table-driven tests
4. Verify all tests pass before commit

### When fixing a bug:
1. Add a failing test case that reproduces the bug
2. Fix the bug
3. Verify the test now passes
4. Add edge case tests to prevent regression

### When refactoring:
1. Run full test suite before changes
2. Make refactoring changes
3. Run full test suite after changes
4. Verify same test count and all pass

---

## Continuous Integration

All tests must pass before merging to main:

```bash
# Format check
go fmt ./...

# Lint check
go vet ./...

# Test execution
go test ./... -count=1
```

Expected output:
```
ok      github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/api      1.766s
ok      github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/chain    1.936s
ok      github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/config   1.177s
ok      github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/consensus 1.235s
ok      github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/crypto   1.221s
ok      github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/mempool  1.348s
ok      github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/p2p      3.043s
ok      github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/storage  1.614s
ok      github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types    1.299s
```

---

## Resources

- [Go testing package](https://golang.org/pkg/testing/)
- [Go test command](https://golang.org/cmd/go/#hdr-Test_packages)
- [Table-driven tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [Go best practices](https://golang.org/doc/effective_go)
