Use **one Go module at the repository root**. Go’s official guidance says placing `go.mod` at the repo root is the simplest module structure, and server-style projects should usually keep private implementation packages under `internal`. ([Go][1])

Already created a github repository, in one of the folders called UTXO Blockchain Node we are completing this project.

---

# Phase 1: Project Initialization & Dependency Management

You already have:

1. A GitHub repository named `BlockChain`.
2. Git already initialized at the repository root.
3. A project folder created inside it named `utxo-blockhain-node`.
4. This UTXO node is only one project inside the larger `BlockChain` repository.

Use this setup:

`BlockChain/utxo-blockhain-node/`

as the root of this specific Go project.

---

## 1. Enter the repository root

`git status`

Expected result:

Git should show the status of the `BlockChain` repository.

---

## 2. Sanity Check

Check if we are in the current folder and every system is okay. 

---

## 3. Initialize a separate Go module for this project

Since your main repository will contain multiple projects, this UTXO node should have its own `go.mod` inside its own folder.

From inside:

`BlockChain/utxo-blockchain-node/`

run:

`go mod init github.com/shaheer1300/BlockChain/utxo-blockchain-node`

Example module shape:

`github.com/shaheer1300/BlockChain/utxo-blockchain-node`

Do not initialize `go.mod` at the root of `BlockChain` unless the entire repository is one Go project. Since this repo will contain multiple projects, keep this Go module inside the UTXO project folder.

---

## 4. Confirm the project-level Go module exists

Run:

`dir`

You should see:

`go.mod`

Then run:

`go env GOMOD`

Expected result:

It should point to:

`BlockChain/utxo-blockchain-node/go.mod`

If it points to somewhere else, you are in the wrong folder.

---

## 5. Create the Go project directory structure

Inside `utxo-blockchain-node`, create these folders:

`cmd/node`

`internal/config`

`internal/types`

`internal/crypto`

`internal/wallet`

`internal/consensus`

`internal/chain`

`internal/mempool`

`internal/storage`

`internal/api`

`internal/p2p`

`internal/node`

`tests`

`scripts`

`docs`

Do not create `/pkg` yet.

Reason:

This project is a runnable blockchain node, not a reusable public library yet. Keep implementation inside `internal` so other projects in the larger repository cannot accidentally import private node logic.

---

## 6. Expected folder structure after Phase 1

Your repository should look like this:

`BlockChain/`

`BlockChain/.git/`

`BlockChain/utxo-blockchain-node/`

`BlockChain/utxo-blockchain-node/go.mod`

`BlockChain/utxo-blockchain-node/cmd/node/`

`BlockChain/utxo-blockchain-node/internal/config/`

`BlockChain/utxo-blockchain-node/internal/types/`

`BlockChain/utxo-blockchain-node/internal/crypto/`

`BlockChain/utxo-blockchain-node/internal/wallet/`

`BlockChain/utxo-blockchain-node/internal/consensus/`

`BlockChain/utxo-blockchain-node/internal/chain/`

`BlockChain/utxo-blockchain-node/internal/mempool/`

`BlockChain/utxo-blockchain-node/internal/storage/`

`BlockChain/utxo-blockchain-node/internal/api/`

`BlockChain/utxo-blockchain-node/internal/p2p/`

`BlockChain/utxo-blockchain-node/internal/node/`

`BlockChain/utxo-blockchain-node/tests/`

`BlockChain/utxo-blockchain-node/scripts/`

`BlockChain/utxo-blockchain-node/docs/`

---

## 7. Add initial dependencies

From inside:

`BlockChain/utxo-blockchain-node/`

install only the essential first dependencies:

`go get github.com/decred/dcrd/dcrec/secp256k1/v4`

`go get go.etcd.io/bbolt`

Then clean the module:

`go mod tidy`

Purpose of each dependency:

`github.com/decred/dcrd/dcrec/secp256k1/v4`

Used for Bitcoin-style secp256k1 private keys, public keys, signing, and signature verification.

`go.etcd.io/bbolt`

Used as the local embedded database for blocks, UTXOs, undo records, and chain metadata.

Do not add web frameworks, CLI frameworks, or libp2p yet.

Use the Go standard library first for HTTP, logging, testing, and basic command execution.

---

## 8. Create environment configuration files

Inside:

`BlockChain/utxo-blockchain-node/`

create:

`.env.example`

Add these required configuration names:

`NODE_ID`

`NETWORK_ID`

`HTTP_ADDR`

`DATA_DIR`

`MINER_ADDRESS`

`PEERS`

`LOG_LEVEL`

`POW_TARGET_PREFIX_ZEROES`

Use these default values conceptually:

`NODE_ID=node1`

`NETWORK_ID=localdev`

`HTTP_ADDR=127.0.0.1:8001`

`DATA_DIR=./data/node1`

`MINER_ADDRESS=`

`PEERS=`

`LOG_LEVEL=debug`

`POW_TARGET_PREFIX_ZEROES=4`

Do not commit a real `.env` file.

Only commit `.env.example`.

---

## 9. Add or update `.gitignore`

At the root of the `BlockChain` repository, or inside `utxo-blockchain-node`, ensure Git ignores generated local files.

Ignore these:

`bin/`

`data/`

`.env`

`*.exe`

`*.test`

`coverage.out`

`.DS_Store`

If the main `BlockChain` repository already has a `.gitignore`, update that one.

Recommended:

Keep one main `.gitignore` at the root of `BlockChain`.

---

## 10. Create initial documentation files

Inside:

`BlockChain/utxo-blockchain-node/docs/`

create these empty files for now:

`consensus.md`

`storage.md`

`api.md`

`reorgs.md`

`testing.md`

`llm-agent-rules.md`

In `llm-agent-rules.md`, document these rules:

1. Do not use JSON for transaction IDs, block hashes, Merkle roots, or signature preimages.
2. Do not put consensus rules inside HTTP handlers.
3. Do not ignore returned errors.
4. Every milestone must include tests.
5. Reorg logic must use undo records.
6. Keep blockchain logic separate from API and P2P code.
7. Do not add new dependencies without explaining why.
8. Keep this project self-contained inside `utxo-blockchain-node`.

---

## 11. Phase 1 LLM agent instruction

Give this instruction to your coding agent:

Implement only Phase 1 project initialization for the Go UTXO blockchain node inside the existing monorepo `BlockChain`. The project folder is `utxo-blockchain-node`. Create a separate Go module inside this folder, not at the repository root. Set up the standard Go directory structure with `cmd/node`, `internal`, `tests`, `scripts`, and `docs`. Add only the initial dependencies `secp256k1/v4` and `bbolt`. Create `.env.example`, update `.gitignore`, and create placeholder documentation files. Do not implement blockchain logic yet.

---

## 12. Phase 1 acceptance criteria

Phase 1 is complete when:

1. `BlockChain` remains the main Git repository.
2. `utxo-blockchain-node` exists as a subproject.
3. `utxo-blockchain-node/go.mod` exists.
4. `go env GOMOD` points to the project-level `go.mod`.
5. The standard folder structure exists.
6. `.env.example` exists.
7. `.env` is ignored by Git.
8. `data/` and `bin/` are ignored by Git.
9. Initial dependencies are added.
10. `go mod tidy` runs successfully.
11. `docs/llm-agent-rules.md` exists.
12. No actual blockchain logic has been implemented yet.

---

## 13. Final Phase 1 verification commands

Run these from inside:

`BlockChain/utxo-blockchain-node/`

Commands:

`go mod tidy`

`go test ./...`

`go list ./...`

Expected result:

1. `go mod tidy` finishes without error.
2. `go test ./...` succeeds or says there are no test files.
3. `go list ./...` lists your module packages.
4. Git shows the new project files ready to commit.

Then from the root of `BlockChain`, commit Phase 1:

`git add .`

`git commit -m "Initialize Go UTXO blockchain node project"`


# Phase 2: Core Architecture Blueprint

## 1. Package responsibilities

Use this separation:

`cmd/node`

Entry point only. It should parse config, initialize dependencies, start the node, and handle shutdown.

`internal/config`

Loads and validates configuration.

`internal/types`

Defines core blockchain data structures: transaction, input, output, outpoint, block header, block, UTXO, block index, undo data.

`internal/crypto`

Hashing, Merkle root, key generation, address creation, signing, signature verification.

`internal/wallet`

Simple local wallet functions: generate keypair, derive address, sign transaction input.

`internal/consensus`

Pure validation rules. No database writes. This package should validate transactions, blocks, proof-of-work, Merkle roots, coinbase rules, and amount rules.

`internal/storage`

Database layer. Owns bbolt access. Exposes repository-style methods for blocks, headers, UTXOs, undo records, chain metadata, and active chain index.

`internal/chain`

Chain state manager. Owns block import, connect block, disconnect block, fork-choice, reorg handling, active tip updates.

`internal/mempool`

Stores unconfirmed transactions. Handles mempool validation, double-spend prevention, fee ordering, eviction, block inclusion cleanup, and reorg reinsertion.

`internal/api`

HTTP handlers. No consensus logic here. It should call node services.

`internal/p2p`

Initial simple peer gossip over HTTP. Later you can replace it with TCP or libp2p.

`internal/node`

Orchestrator. Wires together chain, mempool, storage, API, P2P, and mining.

---

## 2. Core data models

Your LLM agent should define these models in `internal/types`:

1. `Hash32`
2. `Amount`
3. `OutPoint`
4. `TxInput`
5. `TxOutput`
6. `Transaction`
7. `BlockHeader`
8. `Block`
9. `UTXO`
10. `BlockUndo`
11. `BlockIndex`
12. `ChainTip`
13. `MempoolEntry`

Important design rule:

Consensus hashes must use deterministic canonical encoding. Do **not** use JSON for `txid`, block hash, signature preimage, or Merkle root input.

Task for LLM agent:

“Create blockchain domain structs with small methods for TxID, BlockHash, IsCoinbase, TotalOutputValue, and canonical binary encoding. Keep JSON tags only for API responses, not consensus hashing.”

Acceptance criteria:

1. Same transaction always produces same txid.
2. Changing any transaction field changes txid.
3. Same block header always produces same block hash.
4. JSON encoding is not used for consensus hashes.

---

## 3. Storage buckets

Use bbolt buckets:

1. `blocks`
2. `headers`
3. `block_index`
4. `active_chain`
5. `utxos`
6. `undo`
7. `mempool_optional`
8. `chain_meta`

Repository methods required:

1. Save block.
2. Get block by hash.
3. Save header.
4. Get header by hash.
5. Save block index.
6. Get block index.
7. Get active hash by height.
8. Set active hash by height.
9. Get UTXO.
10. Put UTXO.
11. Delete UTXO.
12. Save undo.
13. Get undo.
14. Get best tip.
15. Set best tip.

Acceptance criteria:

1. Database opens and closes cleanly.
2. Required buckets are created on startup.
3. All writes return errors instead of panicking.
4. Restarting the app preserves stored chain metadata.

---

# Phase 3: Step-by-Step Implementation Roadmap

## Milestone 1: Bootable application skeleton

Goal:

A clean Go binary starts, loads config, logs startup, exposes `/health`, and shuts down cleanly.

Files to create:

1. `cmd/node/main.go`
2. `internal/config/config.go`
3. `internal/api/server.go`
4. `internal/node/node.go`

What to implement:

1. Main entry point.
2. Config loading.
3. HTTP server.
4. `/health` endpoint.
5. Graceful shutdown on Ctrl+C.
6. Basic logging using Go standard library first.

Verification:

1. `go run ./cmd/node`
2. Open `http://127.0.0.1:8001/health`
3. Expected result: HTTP 200 with node status.
4. Press Ctrl+C and confirm clean shutdown.

LLM instruction:

“Implement only the bootable skeleton. No blockchain logic yet. Keep packages small. Use only standard library unless absolutely necessary.”

---

## Milestone 2: Core types and deterministic encoding

Goal:

Define transactions, blocks, hashes, and deterministic binary encoding.

What to implement:

1. Transaction model.
2. Block model.
3. OutPoint model.
4. Hash type.
5. Canonical encoder.
6. TxID method.
7. BlockHash method.

Verification:

1. Unit test that same tx gives same txid.
2. Unit test that modified tx gives different txid.
3. Unit test that same header gives same block hash.
4. `go test ./...`

LLM instruction:

“Implement only core types and canonical encoding. Do not implement signatures, database, HTTP endpoints, or mining yet.”

---

## Milestone 3: Crypto primitives

Goal:

Implement hashing, Merkle root, wallet keys, addresses, signing, and verification.

What to implement:

1. SHA-256 helper.
2. Double SHA-256 helper.
3. Merkle root function.
4. secp256k1 private/public key generation.
5. Public-key-to-address function.
6. Transaction signature preimage function.
7. Sign input.
8. Verify input signature.

Verification:

1. Merkle root tests for empty, one leaf, two leaves, odd number of leaves.
2. Valid signature verifies.
3. Modified transaction fails verification.
4. Wrong public key fails verification.

LLM instruction:

“Implement cryptographic helpers with deterministic behavior and tests. Do not add chain state or mempool yet.”

---

## Milestone 4: In-memory UTXO set and transaction validation

Goal:

Validate signed transactions against a UTXO view.

What to implement:

1. UTXO view interface.
2. In-memory UTXO map implementation.
3. Transaction validator.
4. Duplicate input detection.
5. Missing UTXO detection.
6. Signature verification against referenced output.
7. Output sum and input sum validation.
8. Overflow protection.
9. Fee calculation.

Verification:

1. Valid transaction passes.
2. Missing UTXO fails.
3. Duplicate input fails.
4. Wrong signature fails.
5. Output greater than input fails.
6. Zero output fails.
7. Overflow fails.
8. Fee is calculated correctly.

LLM instruction:

“Implement transaction validation as pure logic. It must not depend on bbolt, HTTP, P2P, or global state.”

---

## Milestone 5: Block validation and proof-of-work

Goal:

Validate block structure, Merkle root, coinbase, and proof-of-work.

What to implement:

1. Coinbase transaction rules.
2. Merkle root validation.
3. Fixed easy proof-of-work target.
4. Block timestamp sanity rule.
5. Block-level transaction validation using temporary UTXO overlay.
6. Coinbase reward plus fee limit.

Verification:

1. Valid block passes.
2. Bad Merkle root fails.
3. Bad proof-of-work fails.
4. Missing coinbase fails.
5. Multiple coinbase transactions fail.
6. Coinbase overpay fails.
7. Block with double-spend fails.

LLM instruction:

“Implement block validation without persistent storage. Use an in-memory UTXO view and a temporary overlay for intra-block changes.”

---

## Milestone 6: Persistent storage with bbolt

Goal:

Persist blocks, headers, UTXOs, undo records, and chain metadata.

What to implement:

1. Database open/close.
2. Bucket initialization.
3. Block repository.
4. Header repository.
5. UTXO repository.
6. Undo repository.
7. Chain metadata repository.
8. Atomic update helper.

Verification:

1. Store and load block.
2. Store and load UTXO.
3. Delete UTXO.
4. Store and load undo.
5. Store and load best tip.
6. Restart test confirms persistence.

LLM instruction:

“Implement storage only. Do not implement reorg logic yet. Every storage method must return errors clearly.”

---

## Milestone 7: Chain manager and block connection

Goal:

Import blocks and update the active UTXO set.

What to implement:

1. Genesis initialization.
2. Block import path.
3. Parent lookup.
4. Connect block.
5. Spend old UTXOs.
6. Add new UTXOs.
7. Save undo records.
8. Save block index.
9. Update active chain.
10. Update best tip.

Verification:

1. Genesis is created once.
2. Valid block connects.
3. UTXO set changes correctly.
4. Undo record exists.
5. Restart preserves tip and UTXO set.

LLM instruction:

“Implement only linear-chain block import. Do not implement side chains or reorgs yet.”

---

## Milestone 8: Mining

Goal:

Mine valid blocks locally.

What to implement:

1. Candidate block builder.
2. Coinbase transaction creation.
3. Mempool transaction selection placeholder.
4. Nonce loop.
5. Proof-of-work check.
6. Submit mined block through normal import path.

Verification:

1. Node mines one block.
2. Height increases.
3. Miner receives coinbase UTXO.
4. Mined block survives restart.
5. Invalid mined block is impossible under normal miner path.

LLM instruction:

“Implement simple mining with a fixed easy target. Use the same block import path used for received blocks.”

---

## Milestone 9: Mempool

Goal:

Accept unconfirmed transactions and include them in mined blocks.

What to implement:

1. Mempool transaction map.
2. OutPoint spend index.
3. Fee calculation.
4. Min fee policy.
5. Duplicate transaction rejection.
6. Mempool double-spend rejection.
7. Transaction selection for mining.
8. Remove mined transactions.
9. Remove transactions conflicting with a newly connected block.

Verification:

1. Valid tx enters mempool.
2. Duplicate tx is rejected.
3. Double-spend is rejected.
4. Mined tx is removed.
5. Conflicting tx is removed after block import.
6. Mempool never has two txs spending the same OutPoint.

LLM instruction:

“Implement mempool as policy logic, not consensus logic. Mempool may reject valid-but-low-fee transactions; consensus validation must remain separate.”

---

## Milestone 10: HTTP API

Goal:

Expose node operations through clean HTTP endpoints.

Endpoints:

1. `GET /health`
2. `GET /status`
3. `GET /blocks/{hash}`
4. `GET /utxos/{address}`
5. `GET /balance/{address}`
6. `GET /mempool`
7. `POST /tx`
8. `POST /mine`
9. `GET /peers`

Implementation rules:

1. Handlers parse requests.
2. Handlers call services.
3. Handlers return JSON.
4. Handlers do not validate consensus directly.
5. All errors return structured JSON error responses.

Verification:

1. `/status` returns height and tip.
2. `/mine` mines block.
3. `/balance/{address}` reflects coinbase reward.
4. `/tx` accepts valid transaction.
5. Invalid requests return correct HTTP errors.

LLM instruction:

“Implement HTTP handlers as thin adapters. Do not put blockchain rules in handlers.”

---

## Milestone 11: Fork-choice and reorgs

Goal:

Support side chains and switch to the chain with greatest cumulative work.

What to implement:

1. Store side-chain blocks.
2. Track cumulative chain work.
3. Detect when side branch beats active chain.
4. Find fork point.
5. Disconnect active blocks back to fork.
6. Restore spent UTXOs from undo.
7. Connect new branch blocks.
8. Update active chain index.
9. Revalidate mempool after reorg.

Verification:

1. Build chain A with 2 blocks.
2. Build chain B from same parent with 3 blocks.
3. Node switches to chain B.
4. UTXO set matches chain B.
5. Old chain outputs are removed.
6. Disconnected transactions are reconsidered for mempool.
7. Crash-safe atomicity is preserved as much as bbolt transaction boundaries allow.

LLM instruction:

“Implement reorgs carefully. The active UTXO set must never become a mix of old and new chain state.”

---

## Milestone 12: Simple HTTP peer gossip

Goal:

Run multiple local nodes that share blocks and transactions.

What to implement:

1. Peer list from config.
2. Broadcast transaction to peers.
3. Broadcast block to peers.
4. Receive transaction endpoint.
5. Receive block endpoint.
6. Seen-cache to avoid infinite rebroadcast.
7. Basic request timeout.
8. Parent-missing handling.

Verification:

1. Start node1, node2, node3 on different ports.
2. Submit tx to node1.
3. Tx appears in node2 and node3 mempools.
4. Mine block on node2.
5. Block appears on node1 and node3.
6. All nodes converge to same tip.

LLM instruction:

“Implement simple HTTP gossip only. Do not implement libp2p yet. Use this phase to prove node behavior first.”

---

## Milestone 13: Documentation and developer workflow

Goal:

Make the project easy for future LLM agents and humans to continue.

Create these docs:

1. `docs/consensus.md`
2. `docs/storage.md`
3. `docs/api.md`
4. `docs/reorgs.md`
5. `docs/testing.md`
6. `docs/llm-agent-rules.md`

`docs/llm-agent-rules.md` must say:

1. Do not change consensus encoding without updating tests.
2. Do not put consensus rules in API handlers.
3. Do not use JSON for txid or block hash.
4. Do not skip error handling.
5. Every milestone must add tests.
6. Reorg logic must use undo records.
7. Keep packages small and dependency direction clean.

---

# Phase 4: Idiomatic Go Testing & Verification

Go has built-in unit testing through files ending in `_test.go`, test functions, and the `go test` command. ([Go][6])

## 1. Testing strategy

Use this test layout:

1. Unit tests next to each package.
2. Integration tests in `tests`.
3. No external test framework initially.
4. Use table-driven tests.
5. Use temporary directories for database tests.
6. Use in-memory UTXO views for consensus tests.

---

## 2. Required tests by package

`internal/crypto`

1. Hash determinism.
2. Merkle root correctness.
3. Signature verification.
4. Signature failure after mutation.

`internal/types`

1. Canonical encoding determinism.
2. TxID changes when tx changes.
3. Block hash changes when header changes.

`internal/consensus`

1. Valid tx.
2. Invalid tx.
3. Valid block.
4. Invalid block.
5. Coinbase overpay.
6. Double-spend in block.

`internal/storage`

1. Open DB.
2. Create buckets.
3. Put/get/delete UTXO.
4. Put/get block.
5. Persist after close/reopen.

`internal/chain`

1. Genesis init.
2. Linear block import.
3. Invalid parent rejection.
4. UTXO update after connect.
5. Undo record after connect.
6. Reorg from shorter active chain to heavier side chain.

`internal/mempool`

1. Accept valid tx.
2. Reject duplicate tx.
3. Reject mempool double spend.
4. Remove mined tx.
5. Revalidate after reorg.

`internal/api`

1. Health endpoint.
2. Status endpoint.
3. Mine endpoint.
4. Submit tx endpoint.
5. Error response format.

`internal/p2p`

1. Peer broadcast success.
2. Timeout handling.
3. Duplicate message prevention.
4. Invalid peer response handling.

---

## 3. Milestone verification commands

After every milestone, run:

1. `go fmt ./...`
2. `go test ./...`
3. `go vet ./...`
4. `go mod tidy`
5. `go run ./cmd/node`

The `go` command provides standard operations such as build, clean, fmt, get, test, vet, and env for managing Go source code. ([Go Packages][7])

---

## 4. Definition of done for each milestone

A milestone is complete only when:

1. The app compiles.
2. `go test ./...` passes.
3. No package has circular dependencies.
4. No consensus logic exists in API handlers.
5. Errors are returned, not ignored.
6. Public functions have clear names.
7. The milestone documentation is updated.

---

# Phase 5: Build and Run Automation

## 1. Add a Makefile

Create a `Makefile` with these targets:

1. `fmt`
2. `tidy`
3. `test`
4. `vet`
5. `build`
6. `run`
7. `clean`
8. `node1`
9. `node2`
10. `node3`
11. `devnet`

Expected behavior:

1. `make fmt` runs formatting.
2. `make tidy` cleans module dependencies.
3. `make test` runs all tests.
4. `make vet` runs static checks.
5. `make build` creates a binary in `bin`.
6. `make run` runs one local node.
7. `make clean` removes binaries and temporary data.
8. `make devnet` starts or documents how to start three nodes locally.

LLM instruction:

“Create a simple Makefile for Windows-friendly Go commands. Avoid complex Bash-only syntax if the user is on Windows.”

---

## 2. Add scripts

Create these scripts:

1. `scripts/run-node1.ps1`
2. `scripts/run-node2.ps1`
3. `scripts/run-node3.ps1`
4. `scripts/clean.ps1`
5. `scripts/test.ps1`

Each node script should set:

1. Different `NODE_ID`
2. Different `HTTP_ADDR`
3. Different `DATA_DIR`
4. Same `NETWORK_ID`
5. Peer addresses for the other nodes

Verification:

1. Run node1.
2. Run node2.
3. Run node3.
4. Mine on node1.
5. Confirm node2 and node3 receive the block.

---

# Final Build Order

Follow this exact order:

1. Initialize Go module.
2. Create directories.
3. Build bootable HTTP skeleton.
4. Add config loader.
5. Add core blockchain structs.
6. Add canonical encoding.
7. Add hashing and Merkle root.
8. Add wallet/signature logic.
9. Add in-memory UTXO validation.
10. Add block validation.
11. Add bbolt storage.
12. Add chain manager.
13. Add mining.
14. Add mempool.
15. Add HTTP API.
16. Add reorg support.
17. Add HTTP peer gossip.
18. Add local 3-node devnet scripts.
19. Add documentation.
20. Add final integration tests.

---

# Final MVP checklist

Your project is successful when it can do all of this:

1. Start a node from `cmd/node`.
2. Load config from environment.
3. Create or load a local database.
4. Create genesis block.
5. Generate wallet keys.
6. Mine blocks.
7. Store blocks and UTXOs.
8. Restart without losing state.
9. Create signed UTXO transactions.
10. Validate transactions.
11. Add transactions to mempool.
12. Mine mempool transactions.
13. Run three nodes locally.
14. Gossip transactions.
15. Gossip blocks.
16. Create a fork.
17. Switch to the heavier chain.
18. Reorg without corrupting the UTXO set.
19. Pass `go test ./...`.
20. Explain the system clearly through `docs/consensus.md`, `docs/storage.md`, and `docs/reorgs.md`.

[1]: https://go.dev/doc/modules/managing-source?utm_source=chatgpt.com "Managing module source"
[2]: https://go.dev/doc/tutorial/create-module?utm_source=chatgpt.com "Tutorial: Create a Go module"
[3]: https://go.dev/doc/go1.4?utm_source=chatgpt.com "Go 1.4 Release Notes"
[4]: https://pkg.go.dev/go.etcd.io/bbolt?utm_source=chatgpt.com "bbolt"
[5]: https://go.dev/doc/modules/managing-dependencies?utm_source=chatgpt.com "Managing dependencies"
[6]: https://go.dev/doc/tutorial/add-a-test?utm_source=chatgpt.com "Add a test"
[7]: https://pkg.go.dev/cmd/go?utm_source=chatgpt.com "go command - cmd/go"
