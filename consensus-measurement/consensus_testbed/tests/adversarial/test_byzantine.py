import importlib
import logging

import simpy

from src.consensus.bft import BFTVote, TendermintBFT
from src.core.block import Block, genesis_block
from src.core.metrics import MetricsCollector
from src.core.network import Network
from src.core.node import NodeRole


def require_equivocating_node():
    return getattr(importlib.import_module("src.adversarial.byzantine"), "EquivocatingNode")


def run_byzantine_bft(f_byzantine):
    EquivocatingNode = require_equivocating_node()
    env = simpy.Environment()
    network = Network(env, mean_latency=0.05, latency_std=0.0, seed=42)
    metrics = MetricsCollector(sim_duration=500.0)
    node_ids = [f"N{i}" for i in range(10)]
    nodes = []
    protocols = []

    for index, node_id in enumerate(node_ids):
        if index < f_byzantine:
            node = EquivocatingNode(id=node_id, peers=node_ids, byzantine_strategy="split_prevote")
        else:
            node = EquivocatingNode(id=node_id, role=NodeRole.HONEST, peers=node_ids)
        nodes.append(node)
        network.register_node(node)

    for index, node in enumerate(nodes):
        protocol = TendermintBFT(
            node_id=node.id,
            node_ids=node_ids,
            round_timeout=0.5,
            txs_per_block=20,
            seed=42 + index,
            byzantine=node.role == NodeRole.BYZANTINE,
        )
        protocols.append(protocol)
        env.process(protocol.run(env, node, network, metrics))

    env.run(until=500.0)
    return nodes, protocols, metrics.snapshot(), metrics


def honest_protocols(protocols):
    return [protocol for protocol in protocols if not protocol.byzantine]


def make_vote(sender, block, block_hash=None):
    return BFTVote(
        height=1,
        round=1,
        phase="PREVOTE",
        block_hash=block_hash or block.hash,
        block=block,
        sender=sender,
    )


def test_bft_safe_with_max_byzantine():
    _nodes, protocols, snapshot, _metrics = run_byzantine_bft(f_byzantine=3)
    honest = honest_protocols(protocols)

    assert snapshot.safety_violations == 0
    assert all(len(protocol.chain) - 1 >= 10 for protocol in honest)


def test_bft_unsafe_beyond_threshold(caplog):
    caplog.set_level(logging.WARNING)
    _nodes, protocols, snapshot, _metrics = run_byzantine_bft(f_byzantine=4)
    honest_commits = min(len(protocol.chain) - 1 for protocol in honest_protocols(protocols))
    failure_detected = snapshot.safety_violations > 0 or honest_commits < 2

    logging.getLogger(__name__).warning(
        "BFT beyond-threshold failure mode: safety_violations=%s honest_commits=%s",
        snapshot.safety_violations,
        honest_commits,
    )

    assert failure_detected


def test_equivocating_votes_logged(caplog):
    bft = TendermintBFT(
        node_id="N0",
        node_ids=["N0", "N1", "N2", "N3"],
        round_timeout=0.5,
        txs_per_block=20,
        seed=42,
    )
    bft.round = 1
    bft.height = 1
    genesis = genesis_block()
    block = Block(1, genesis.hash, "B", 1.0, {"round": 1})
    conflicting_block = Block(1, genesis.hash, "B-prime", 1.0, {"round": 1})
    caplog.set_level(logging.WARNING)

    bft.on_message(bft.vote_message("N1", make_vote("N1", block)))
    bft.on_message(bft.vote_message("N1", make_vote("N1", conflicting_block)))

    assert "equivocation" in caplog.text
    assert "N1" in caplog.text
