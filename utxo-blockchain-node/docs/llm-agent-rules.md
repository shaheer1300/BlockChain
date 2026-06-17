1. Do not use JSON for transaction IDs, block hashes, Merkle roots, or signature preimages.
2. Do not put consensus rules inside HTTP handlers.
3. Do not ignore returned errors.
4. Every milestone must include tests.
5. Reorg logic must use undo records.
6. Keep blockchain logic separate from API and P2P code.
7. Do not add new dependencies without explaining why.
8. Keep this project self-contained inside `utxo-blockchain-node`.