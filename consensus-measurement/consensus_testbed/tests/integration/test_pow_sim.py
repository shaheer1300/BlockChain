import logging
import importlib

import simpy

from src.core.metrics import MetricsCollector
from src.core.network import Network
from src.core.node import Node, NodeRole


logger = logging.getLogger(__name__)


def run_pow_sim(
    n_nodes=5,
    mean_latency=0.05,
    latency_std=0.0,
    target_block_time=10.0,
    txs_per_block=5,
    sim_duration=500.0,
    seed=42,
):
    ProofOfWork = getattr(importlib.import_module("src.consensus.pow"), "ProofOfWork")
    env = simpy.Environment()
    network = Network(env, mean_latency=mean_latency, latency_std=latency_std, seed=seed)
    metrics = MetricsCollector(sim_duration=sim_duration)
    nodes = [Node(id=f"N{i}", role=NodeRole.HONEST) for i in range(n_nodes)]
    peer_ids = [node.id for node in nodes]
    protocols = []

    for node in nodes:
        network.register_node(node)

    for index, node in enumerate(nodes):
        protocol = ProofOfWork(
            node_id=node.id,
            peer_ids=[peer_id for peer_id in peer_ids if peer_id != node.id],
            hash_fraction=1.0 / n_nodes,
            target_block_time=target_block_time,
            txs_per_block=txs_per_block,
            seed=seed + index,
        )
        for tx_index in range(10_000):
            protocol.mempool.add(f"{node.id}-tx{tx_index}")
        protocols.append(protocol)
        env.process(protocol.run(env, node, network, metrics))

    env.run(until=sim_duration)
    return nodes, protocols, metrics.snapshot()


def test_pow_blocks_mined():
    nodes, protocols, metrics = run_pow_sim(
        n_nodes=5,
        mean_latency=0.05,
        target_block_time=10.0,
        txs_per_block=5,
        seed=42,
        sim_duration=500.0,
    )

    total_canonical_blocks = sum(len(protocol.get_canonical_chain()) - 1 for protocol in protocols)

    assert metrics.message_count > 0
    assert total_canonical_blocks > 0
    assert max(protocol.get_canonical_chain()[-1].height for protocol in protocols) >= 30


def test_fork_rate_monotone_with_latency(caplog):
    caplog.set_level(logging.INFO)
    fork_rates = []

    for latency in [0.010, 0.100, 1.000]:
        _nodes, _protocols, metrics = run_pow_sim(
            n_nodes=10,
            mean_latency=latency,
            latency_std=0.0,
            target_block_time=10.0,
            txs_per_block=5,
            seed=42,
            sim_duration=2000.0,
        )
        fork_rates.append(metrics.fork_rate)

    logger.info(
        "PoW fork rates: latency=0.010 %.4f, latency=0.100 %.4f, latency=1.000 %.4f",
        fork_rates[0],
        fork_rates[1],
        fork_rates[2],
    )

    assert fork_rates[0] < fork_rates[1] < fork_rates[2]
    assert fork_rates[0] < 0.05
    assert fork_rates[2] > 0.05


def test_block_propagation_coverage():
    _nodes, protocols, _metrics = run_pow_sim(
        n_nodes=10,
        mean_latency=0.1,
        latency_std=0.02,
        target_block_time=10.0,
        txs_per_block=5,
        seed=42,
        sim_duration=300.0,
    )
    reference_chain = protocols[0].get_canonical_chain()
    height_five = next(block for block in reference_chain if block.height == 5)

    coverage = sum(
        1
        for protocol in protocols
        if any(block.hash == height_five.hash for block in protocol.get_canonical_chain())
    )

    assert coverage >= 8


def test_pow_throughput_bounded():
    _nodes, _protocols, metrics = run_pow_sim(
        n_nodes=5,
        mean_latency=0.1,
        latency_std=0.0,
        target_block_time=10.0,
        txs_per_block=20,
        seed=42,
        sim_duration=1000.0,
    )
    theoretical_max_tps = 20 / 10.0

    assert metrics.throughput_tps <= theoretical_max_tps * 1.05
