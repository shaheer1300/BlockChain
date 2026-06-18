# Execution & Demo Guide

A step-by-step playbook for presenting, testing, and verifying the UTXO blockchain node.
Every command runs from the project root: `utxo-blockchain-node/`.

---

## Prerequisites

| Tool | Minimum version | How to check |
|------|----------------|--------------|
| Go | 1.22 | `go version` |
| PowerShell | 5.1 (Win) / 7+ (cross-platform) | `$PSVersionTable.PSVersion` |
| GNU Make *(optional)* | any | `make --version` |
| curl *(optional)* | any | `curl --version` |

> PowerShell is used for the node-launch scripts. `Invoke-WebRequest` is built in, so
> `curl` is optional — PowerShell equivalents are shown throughout.

---

## 1  Verify the codebase builds and all tests pass

```powershell
cd "D:\Github Projects\BlockChain\utxo-blockchain-node"

go fmt ./...
go vet ./...
go test ./... -count=1
```

**Expected output — every package shows `ok`:**

```
ok  internal/api
ok  internal/chain
ok  internal/config
ok  internal/consensus
ok  internal/crypto
ok  internal/mempool
ok  internal/p2p
ok  internal/storage
ok  internal/types
```

Or run the one-shot script:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test.ps1
```

---

## 2  Single-node smoke test

This is the quickest demo: one node, mine some blocks, inspect the chain.

### 2a  Start the node

```powershell
$env:NODE_ID    = "demo"
$env:HTTP_ADDR  = "127.0.0.1:8001"
$env:DATA_DIR   = "./data/demo"
$env:NETWORK_ID = "localdev"
$env:PEERS      = ""

go run ./cmd/node
```

Leave this terminal running. **Open a second terminal** for all API calls below.

### 2b  Health check

```powershell
Invoke-WebRequest http://127.0.0.1:8001/health | Select-Object -ExpandProperty Content
```

Expected:
```json
{"status":"ok","node_id":"demo","network":"localdev"}
```

### 2c  Chain status (before any blocks)

```powershell
Invoke-WebRequest http://127.0.0.1:8001/status | Select-Object -ExpandProperty Content
```

Expected:
```json
{"node_id":"demo","network":"localdev","height":null,"tip_hash":null}
```

`height` and `tip_hash` are `null` because the chain is empty before the genesis block is mined.

### 2d  Mine the genesis block

```powershell
Invoke-WebRequest -Method POST http://127.0.0.1:8001/mine | Select-Object -ExpandProperty Content
```

Expected (hash will differ each run):
```json
{"hash":"0000a3f...","height":1}
```

> Mining performs Proof-of-Work (4 leading zero hex nibbles by default).
> On a modern machine it takes under a second.

### 2e  Chain status (after genesis)

```powershell
Invoke-WebRequest http://127.0.0.1:8001/status | Select-Object -ExpandProperty Content
```

Expected — `height` is now 1, `tip_hash` matches the hash from step 2d:
```json
{"node_id":"demo","network":"localdev","height":1,"tip_hash":"0000a3f..."}
```

### 2f  Mine several more blocks

```powershell
1..4 | ForEach-Object {
    Invoke-WebRequest -Method POST http://127.0.0.1:8001/mine |
        Select-Object -ExpandProperty Content
}
```

Then check status again — `height` should be 5.

### 2g  Inspect a block by hash

Copy any hash from a mine response, then:

```powershell
$hash = "PASTE_HASH_HERE"
Invoke-WebRequest "http://127.0.0.1:8001/blocks/$hash" |
    Select-Object -ExpandProperty Content | ConvertFrom-Json | ConvertTo-Json -Depth 10
```

You will see the full block including:
- `header` — version, prev_hash, merkle_root, timestamp, bits, nonce
- `transactions` — coinbase tx with a UTXO output to the miner address

### 2h  Check UTXOs and balance for the miner address

The miner address is set via `MINER_ADDRESS`. If unset, the node generates one internally.
The `/status` response does not expose it directly; look inside the coinbase transaction's output.

```powershell
# Extract miner address from the last mined block's coinbase output
$status = Invoke-WebRequest http://127.0.0.1:8001/status | Select-Object -ExpandProperty Content | ConvertFrom-Json
$block  = Invoke-WebRequest "http://127.0.0.1:8001/blocks/$($status.tip_hash)" |
              Select-Object -ExpandProperty Content | ConvertFrom-Json
$addr   = $block.transactions[0].outputs[0].recipient
Write-Host "Miner address: $addr"

# List UTXOs owned by the miner
Invoke-WebRequest "http://127.0.0.1:8001/utxos/$addr" |
    Select-Object -ExpandProperty Content | ConvertFrom-Json | ConvertTo-Json -Depth 10

# Get total balance
Invoke-WebRequest "http://127.0.0.1:8001/balance/$addr" |
    Select-Object -ExpandProperty Content
```

Expected balance response:
```json
{"address":"<addr>","balance":250000000}
```

Balance = 5 blocks × 50,000,000 sats initial subsidy (halves every 210,000 blocks).

### 2i  Stop the node and restart — state must persist

Stop the first terminal (`Ctrl+C`), restart it:

```powershell
go run ./cmd/node
```

Immediately call `/status` again. The `height` and `tip_hash` must be the same as before shutdown, proving the chain survives restart via bbolt.

---

## 3  Mempool and transaction demo

Requires a node running (step 2a).

### 3a  Check empty mempool

```powershell
Invoke-WebRequest http://127.0.0.1:8001/mempool | Select-Object -ExpandProperty Content
```

Expected: `{"count":0,"entries":[]}`

### 3b  Mine a block to fund the miner

```powershell
Invoke-WebRequest -Method POST http://127.0.0.1:8001/mine | Select-Object -ExpandProperty Content
```

### 3c  Submit a transaction

A signed P2PKH transaction requires a key and signing logic; the `POST /tx` endpoint
accepts the canonical JSON-encoded `Transaction` struct. In a production demo this is
done with a wallet tool. For now, mine a block that includes mempool txs in step 3d.

*(For a scripted demo, use the test helper `signedTx` directly in a Go `_test.go` file,
or call the API from an integration test — see `internal/chain` and `internal/mempool`.)*

### 3d  Mine mempool transactions into a block

Once one or more transactions are in the mempool:

```powershell
Invoke-WebRequest -Method POST http://127.0.0.1:8001/mine | Select-Object -ExpandProperty Content
```

Call `/mempool` after — entries that were mined must disappear (`"count":0`).

---

## 4  3-node devnet (peer gossip demo)

This is the full demo: mine on one node, watch the block propagate to the other two.

### 4a  Open three PowerShell terminals

**Terminal 1 — node1:**
```powershell
cd "D:\Github Projects\BlockChain\utxo-blockchain-node"
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run-node1.ps1
```

**Terminal 2 — node2:**
```powershell
cd "D:\Github Projects\BlockChain\utxo-blockchain-node"
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run-node2.ps1
```

**Terminal 3 — node3:**
```powershell
cd "D:\Github Projects\BlockChain\utxo-blockchain-node"
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run-node3.ps1
```

Devnet ports:

| Node | HTTP port | Peers |
|------|-----------|-------|
| node1 | 8001 | 8002, 8003 |
| node2 | 8002 | 8001, 8003 |
| node3 | 8003 | 8001, 8002 |

All three share `NETWORK_ID=localdev`.

### 4b  Confirm all three nodes are up

**Terminal 4 (or any shell):**
```powershell
Invoke-WebRequest http://127.0.0.1:8001/health | Select-Object -ExpandProperty Content
Invoke-WebRequest http://127.0.0.1:8002/health | Select-Object -ExpandProperty Content
Invoke-WebRequest http://127.0.0.1:8003/health | Select-Object -ExpandProperty Content
```

### 4c  Mine on node1 only

```powershell
Invoke-WebRequest -Method POST http://127.0.0.1:8001/mine | Select-Object -ExpandProperty Content
```

Note the returned hash, e.g. `{"hash":"0000abc...","height":1}`.

### 4d  Verify gossip reached node2 and node3

```powershell
Invoke-WebRequest http://127.0.0.1:8002/status | Select-Object -ExpandProperty Content
Invoke-WebRequest http://127.0.0.1:8003/status | Select-Object -ExpandProperty Content
```

**Expected — both show the same `height` and `tip_hash` as node1:**
```json
{"node_id":"node2","network":"localdev","height":1,"tip_hash":"0000abc..."}
{"node_id":"node3","network":"localdev","height":1,"tip_hash":"0000abc..."}
```

The block traveled: node1 → POST /p2p/block on node2 and node3 → imported by each.

### 4e  Mine several blocks and watch all nodes converge

```powershell
1..5 | ForEach-Object {
    Invoke-WebRequest -Method POST http://127.0.0.1:8001/mine |
        Select-Object -ExpandProperty Content
    Start-Sleep -Milliseconds 200
}
```

Then confirm all three nodes agree:
```powershell
@(8001,8002,8003) | ForEach-Object {
    $r = Invoke-WebRequest "http://127.0.0.1:$_/status" | Select-Object -ExpandProperty Content | ConvertFrom-Json
    Write-Host "$($r.node_id): height=$($r.height)  tip=$($r.tip_hash)"
}
```

All three lines must show identical `height` and `tip_hash`.

### 4f  Check peer lists

```powershell
Invoke-WebRequest http://127.0.0.1:8001/peers | Select-Object -ExpandProperty Content
```

Expected:
```json
{"peers":["http://127.0.0.1:8002","http://127.0.0.1:8003"]}
```

---

## 5  Persistence across restart (devnet)

1. Stop all three nodes (`Ctrl+C` in each terminal).
2. Restart only node1 (`scripts/run-node1.ps1`).
3. Call `/status` on node1 — height must be the same as before shutdown.
4. Restart node2 and node3.
5. Mine another block on node1 — node2 and node3 must receive it and reach height+1.

---

## 6  Clean up

Remove all local data and build artifacts:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/clean.ps1
```

This deletes `bin/`, `data/` (all node databases), and any stray `node.exe`.

---

## 7  Run automated tests (full quality gate)

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test.ps1
```

Or individually:

```powershell
go fmt ./...                  # formatting
go vet ./...                  # static analysis
go test ./... -count=1        # all 219 tests, no caching
go test ./... -count=5        # repeat 5x to catch flakiness
go test -race ./...           # race detector
```

### Test count by package

| Package | Tests |
|---------|------:|
| `internal/types` | 40 |
| `internal/crypto` | 21 |
| `internal/storage` | 18 |
| `internal/consensus` | 32 |
| `internal/chain` | 30 |
| `internal/mempool` | 14 |
| `internal/api` | 29 |
| `internal/p2p` | 17 |
| **Total** | **219** |

---

## 8  Build a standalone binary

```powershell
go build -o bin/node.exe ./cmd/node
```

Run it the same way as `go run`, just faster:

```powershell
$env:NODE_ID   = "demo"
$env:HTTP_ADDR = "127.0.0.1:8001"
$env:DATA_DIR  = "./data/demo"
.\bin\node.exe
```

---

## 9  API reference (quick lookup)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness probe — returns `{"status":"ok"}` |
| `GET` | `/status` | Current chain height and tip hash |
| `GET` | `/blocks/{hash}` | Full block by 64-char hex hash |
| `GET` | `/utxos/{address}` | All UTXOs owned by a 40-char hex address |
| `GET` | `/balance/{address}` | Total spendable balance for an address |
| `GET` | `/mempool` | Pending transactions sorted by fee rate |
| `GET` | `/peers` | Configured peer URLs |
| `POST` | `/mine` | Mine one block from the current mempool |
| `POST` | `/tx` | Submit a signed transaction to the mempool |
| `POST` | `/p2p/tx` | Internal — receive a gossipped transaction from a peer |
| `POST` | `/p2p/block` | Internal — receive a gossipped block from a peer |

---

## 10  What each demo proves

| Demo step | Feature proven |
|-----------|---------------|
| `go test ./... -count=1` | All 219 unit tests pass; no regressions |
| Mine genesis, check status | PoW, block storage, chain tip update |
| Restart and re-check status | bbolt persistence survives process restart |
| Check balance after 5 blocks | UTXO accounting, coinbase subsidy, address indexing |
| 3-node gossip (`4c → 4d`) | HTTP peer gossip, block propagation, deduplication |
| All 3 nodes same tip (`4e`) | Eventual consistency, fork-choice alignment |
| `clean.ps1` then `test.ps1` | Reproducible build from scratch |
