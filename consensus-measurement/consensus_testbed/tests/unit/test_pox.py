import importlib
from collections import Counter

from src.core.block import Block


def require_pox_symbol(name):
    return getattr(importlib.import_module("src.consensus.pox"), name)


def test_pox_miner_selection_proportional():
    ProofOfTransfer = require_pox_symbol("ProofOfTransfer")
    bids = [10.0, 20.0, 30.0, 40.0, 50.0]
    winners = [
        ProofOfTransfer._select_winner(bids=bids, seed=42 + election)
        for election in range(2000)
    ]
    counts = Counter(winners)

    miner_0_rate = counts[0] / 2000
    miner_4_rate = counts[4] / 2000

    assert 0.06 <= miner_0_rate <= 0.14
    assert 0.29 <= miner_4_rate <= 0.37


def test_stacker_reward_calculation():
    compute_stacker_rewards = require_pox_symbol("compute_stacker_rewards")

    rewards = compute_stacker_rewards(locked_stx=[100, 200, 300], winning_bid=60.0)

    assert rewards[0] == 10.0
    assert rewards[1] == 20.0
    assert rewards[2] == 30.0
    assert sum(rewards) == 60.0


def test_pox_block_anchored_to_base_block():
    create_pox_block = require_pox_symbol("create_pox_block")
    base_block = Block(
        height=12,
        prev_hash="0" * 64,
        payload_hash="base",
        timestamp=120.0,
        protocol_fields={},
    )
    object.__setattr__(base_block, "hash", "abc")

    pox_block = create_pox_block(
        height=1,
        prev_hash="0" * 64,
        base_block=base_block,
        miner_id="M0",
        tx_count=20,
        timestamp=121.0,
    )

    assert pox_block.protocol_fields["anchor_hash"] == "abc"
    assert pox_block.protocol_fields["base_height"] == base_block.height


def test_pox_finality_requires_k_confirmations():
    create_pox_block = require_pox_symbol("create_pox_block")
    is_final = require_pox_symbol("is_final")
    base_block = Block(
        height=100,
        prev_hash="0" * 64,
        payload_hash="base",
        timestamp=1000.0,
        protocol_fields={},
    )
    pox_block = create_pox_block(
        height=1,
        prev_hash="0" * 64,
        base_block=base_block,
        miner_id="M0",
        tx_count=20,
        timestamp=1001.0,
    )

    assert is_final(pox_block, current_base_height=105, k_confirmations=6) is False
    assert is_final(pox_block, current_base_height=106, k_confirmations=6) is True
    assert is_final(pox_block, current_base_height=200, k_confirmations=6) is True


def test_zero_bid_excluded_from_selection():
    ProofOfTransfer = require_pox_symbol("ProofOfTransfer")
    bids = [0.0, 10.0, 20.0]

    winners = [
        ProofOfTransfer._select_winner(bids=bids, seed=42 + election)
        for election in range(100)
    ]

    assert 0 not in winners
