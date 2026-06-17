# A UTXO Blockchain Node, From Scratch

*Core objective.* Implement a minimal but correct Bitcoin-style blockchain: a UTXO-model ledger with Proof-of-Work, Merkle-tree block commitments, transaction validation, a mempool, fork-choice (heaviest chain), and peer-to-peer block/transaction gossip between several nodes.

*Target stack & concepts.* Rust or Go (signals systems seriousness); libp2p or raw TCP for the gossip layer; ECDSA/secp256k1 for signatures; SHA-256 and Merkle trees for commitments. Concepts demonstrated: the UTXO state model, cryptographic hashing and digital signatures, Merkle proofs, Nakamoto consensus, and fork resolution.

*System design challenge.* The hard parts aren't the crypto primitives — they're the distributed-systems problems your enterprise background makes you credible on: designing the mempool eviction policy, handling reorgs without corrupting the UTXO set, reasoning about eventual consistency across gossiping peers, and bounding state growth. Building UTXO rather than the easier account model is a deliberate signal that you understand *Bitcoin's* design, not just "a blockchain."