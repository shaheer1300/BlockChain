import importlib
import re

import numpy as np

from src.core.block import genesis_block


HEX_64_RE = re.compile(r"^[0-9a-f]{64}$")


def require_pow_symbol(name):
    return getattr(importlib.import_module("src.consensus.pow"), name)


def require_mempool():
    return getattr(importlib.import_module("src.core.mempool"), "Mempool")


def make_block(height, prev_hash, payload_hash):
    PoWBlock = require_pow_symbol("PoWBlock")
    return PoWBlock(
        height=height,
        prev_hash=prev_hash,
        payload_hash=payload_hash,
        timestamp=float(height),
        protocol_fields={"difficulty": 4},
    )


def test_pow_block_valid_structure():
    PoWBlock = require_pow_symbol("PoWBlock")
    genesis = genesis_block()

    block = PoWBlock(
        height=1,
        prev_hash=genesis.hash,
        payload_hash="tx123",
        timestamp=100.0,
        protocol_fields={"difficulty": 4},
    )

    assert HEX_64_RE.match(block.hash)
    assert block.height == 1
    assert block.protocol_fields["difficulty"] == 4


def test_fork_choice_selects_heaviest_chain():
    fork_choice = require_pow_symbol("fork_choice")
    genesis = genesis_block()
    b1a = make_block(1, genesis.hash, "a1")
    b2a = make_block(2, b1a.hash, "a2")
    b3a = make_block(3, b2a.hash, "a3")
    b1b = make_block(1, genesis.hash, "b1")
    b2b = make_block(2, b1b.hash, "b2")
    b3b = make_block(3, b2b.hash, "b3")
    b4b = make_block(4, b3b.hash, "b4")
    chain_a = [genesis, b1a, b2a, b3a]
    chain_b = [genesis, b1b, b2b, b3b, b4b]

    result = fork_choice([chain_a, chain_b])

    assert result == chain_b


def test_fork_choice_tie_selects_first_seen():
    fork_choice = require_pow_symbol("fork_choice")
    genesis = genesis_block()
    b1a = make_block(1, genesis.hash, "a1")
    b2a = make_block(2, b1a.hash, "a2")
    b3a = make_block(3, b2a.hash, "a3")
    b1b = make_block(1, genesis.hash, "b1")
    b2b = make_block(2, b1b.hash, "b2")
    b3b = make_block(3, b2b.hash, "b3")
    chain_a = [genesis, b1a, b2a, b3a]
    chain_b = [genesis, b1b, b2b, b3b]

    result = fork_choice([chain_a, chain_b])

    assert result == chain_a


def test_mempool_add_and_select():
    Mempool = require_mempool()
    mempool = Mempool(max_size=10, seed=42)

    for i in range(12):
        mempool.add(f"tx{i}")
    selected = mempool.select(n=5)

    assert len(mempool) == 10
    assert len(selected) == 5
    assert all(tx_id in mempool for tx_id in selected)


def test_mempool_remove_confirmed():
    Mempool = require_mempool()
    mempool = Mempool(max_size=10, seed=42)

    for i in range(5):
        mempool.add(f"tx{i}")
    mempool.remove_confirmed(["tx1", "tx3"])

    assert "tx1" not in mempool
    assert "tx3" not in mempool
    assert len(mempool) == 3


def test_transaction_count_per_block():
    ProofOfWork = require_pow_symbol("ProofOfWork")
    miner = ProofOfWork(
        node_id="N0",
        peer_ids=[],
        hash_fraction=1.0,
        target_block_time=10.0,
        txs_per_block=10,
        seed=42,
    )
    for i in range(10):
        miner.mempool.add(f"tx{i}")

    block = miner.mine_block(timestamp=1.0)

    assert block.protocol_fields["tx_count"] == 10


def test_mining_time_distribution():
    ProofOfWork = require_pow_symbol("ProofOfWork")
    pow_miner = ProofOfWork(
        node_id="N0",
        peer_ids=[],
        hash_fraction=0.5,
        target_block_time=10.0,
        txs_per_block=5,
        seed=42,
    )

    times = np.array([pow_miner._sample_mining_time() for _ in range(1000)])

    assert 18.0 <= times.mean() <= 22.0
    assert times.min() > 0
