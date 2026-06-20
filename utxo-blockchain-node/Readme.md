# A UTXO Blockchain Node, From Scratch

*Core objective.* Implement a minimal but correct Bitcoin-style blockchain: a UTXO-model ledger with Proof-of-Work, Merkle-tree block commitments, transaction validation, a mempool, fork-choice (heaviest chain), and peer-to-peer block/transaction gossip between several nodes.

*Target stack & concepts.* Rust or Go (signals systems seriousness); libp2p or raw TCP for the gossip layer; ECDSA/secp256k1 for signatures; SHA-256 and Merkle trees for commitments. Concepts demonstrated: the UTXO state model, cryptographic hashing and digital signatures, Merkle proofs, Nakamoto consensus, and fork resolution.

*System design challenge.* The hard parts aren't the crypto primitives — they're the distributed-systems problems your enterprise background makes you credible on: designing the mempool eviction policy, handling reorgs without corrupting the UTXO set, reasoning about eventual consistency across gossiping peers, and bounding state growth. Building UTXO rather than the easier account model is a deliberate signal that you understand *Bitcoin's* design, not just "a blockchain."

## Quick demo start

Open two PowerShell terminals:

1. Terminal 1: start the blockchain node. This is just an example, you can change the env variables as you see fit
   ```powershell
   cd "D:\Github Projects\BlockChain\utxo-blockchain-node"
   $env:DEMO_MODE = "1"
   $env:HTTP_ADDR = "127.0.0.1:8001"
   $env:DATA_DIR = "./data/demo"
   $env:POW_TARGET_PREFIX_ZEROES = "2"
   go run ./cmd/node
   ```
2. Terminal 2: start the frontend.
   ```powershell
   cd "D:\Github Projects\BlockChain\utxo-blockchain-node\web"
   npm install
   npm run dev
   ```

Then open <http://localhost:5173> in your browser.

# Architecture

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

## Demo Screenshots

When you deploy this will be the first screen you will see.

<img width="1892" height="906" alt="image" src="https://github.com/user-attachments/assets/b743f2cf-3028-428f-8694-a0e8ff42d032" />

Click on initialise demo button to initialise the block chain. 

<img width="1912" height="907" alt="image" src="https://github.com/user-attachments/assets/6769ea3b-54cc-490d-b47e-cb56bd3ae65b" />

You can play around with it and see how mempool, block chain, and transactions. It will also let you create a double spend attempt. If you break the system I implemented raise an issue and let me know.  
