import importlib
import logging

import simpy

from src.core.block import Block, genesis_block
from src.core.metrics import MetricsCollector
from src.core.network import Network
from src.core.node import Node, NodeRole
from src.core.message import MessageType


def require_bft_symbol(name):
    return getattr(importlib.import_module("src.consensus.bft"), name)


def make_bft(node_id="N0", node_ids=None, round_timeout=0.5, byzantine=False):
    TendermintBFT = require_bft_symbol("TendermintBFT")
    return TendermintBFT(
        node_id=node_id,
        node_ids=node_ids or ["N0", "N1", "N2", "N3"],
        round_timeout=round_timeout,
        txs_per_block=20,
        seed=42,
        byzantine=byzantine,
    )


def make_block(height=1, payload_hash="payload"):
    genesis = genesis_block()
    return Block(
        height=height,
        prev_hash=genesis.hash,
        payload_hash=payload_hash,
        timestamp=0.0,
        protocol_fields={"round": 1, "proposer": "N1"},
    )


def make_vote(sender, block, phase="PREVOTE", round_number=1, height=1):
    BFTVote = require_bft_symbol("BFTVote")
    return BFTVote(
        height=height,
        round=round_number,
        phase=phase,
        block_hash=block.hash,
        block=block,
        sender=sender,
    )


def test_proposer_selection_round_robin():
    bft = make_bft(node_ids=["N0", "N1", "N2", "N3"])

    assert bft.proposer_for_round(round=0) == "N0"
    assert bft.proposer_for_round(round=1) == "N1"
    assert bft.proposer_for_round(round=4) == "N0"
    assert bft.proposer_for_round(round=7) == "N3"


def test_quorum_threshold():
    bft_10 = make_bft(node_ids=[f"N{i}" for i in range(10)])
    bft_4 = make_bft(node_ids=["N0", "N1", "N2", "N3"])

    assert bft_10.quorum_size() == 7
    assert bft_4.quorum_size() == 3


def test_prevote_phase_accumulates_votes():
    bft = make_bft(node_id="N0")
    bft.round = 1
    bft.height = 1
    block = make_block()

    bft.on_message(bft.vote_message("N1", make_vote("N1", block, phase="PREVOTE")))
    bft.on_message(bft.vote_message("N2", make_vote("N2", block, phase="PREVOTE")))

    assert bft.phase != "PRECOMMIT"

    bft.on_message(bft.vote_message("N3", make_vote("N3", block, phase="PREVOTE")))

    assert bft.phase == "PRECOMMIT"
    assert bft.locked_block == block


def test_precommit_phase_commits_block():
    bft = make_bft(node_id="N0")
    bft.round = 1
    bft.height = 1
    block = make_block()

    for sender in ["N1", "N2", "N3"]:
        bft.on_message(bft.vote_message(sender, make_vote(sender, block, phase="PRECOMMIT")))

    assert bft.chain[-1] == block
    assert bft.chain[-1].height == 1


def test_equivocation_detection(caplog):
    bft = make_bft(node_id="N0")
    bft.round = 1
    bft.height = 1
    block = make_block(payload_hash="B")
    conflicting_block = make_block(payload_hash="B-prime")
    caplog.set_level(logging.WARNING)

    bft.on_message(bft.vote_message("N1", make_vote("N1", block, phase="PREVOTE")))
    bft.on_message(
        bft.vote_message(
            "N1",
            make_vote("N1", conflicting_block, phase="PREVOTE"),
        )
    )

    votes = bft.votes[(1, 1, "PREVOTE", block.hash)]
    assert "equivocation" in caplog.text
    assert len(votes) == 1
    assert "N1" in votes


def test_view_change_on_timeout():
    env = simpy.Environment()
    network = Network(env, mean_latency=0.05, latency_std=0.0, seed=42)
    metrics = MetricsCollector(sim_duration=1.0)
    node = Node(id="N0", role=NodeRole.HONEST)
    sent_messages = []
    bft = make_bft(node_id="N0")
    bft.round = 1

    for node_id in ["N0", "N1", "N2", "N3"]:
        network.register_node(node if node_id == "N0" else Node(id=node_id, role=NodeRole.HONEST))

    def record_send(sender_id, receiver_id, message):
        sent_messages.append(message)

    network.send = record_send
    env.process(bft.run(env, node, network, metrics))
    env.run(until=1.0)

    assert any(message.type == MessageType.TIMEOUT for message in sent_messages)
    assert bft.round == 2
    assert bft.proposer_for_round(bft.round) == "N2"
