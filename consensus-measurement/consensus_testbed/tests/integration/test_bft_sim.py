import importlib
import logging

import simpy

from src.core.metrics import MetricsCollector
from src.core.network import Network
from src.core.node import Node, NodeRole


logger = logging.getLogger(__name__)


def run_bft_sim(
    n_nodes=4,
    f_byzantine=0,
    round_timeout=0.5,
    mean_latency=0.05,
    latency_std=0.0,
    sim_duration=200.0,
    seed=42,
    partition_at=None,
    heal_at=None,
):
    TendermintBFT = getattr(importlib.import_module("src.consensus.bft"), "TendermintBFT")
    env = simpy.Environment()
    network = Network(env, mean_latency=mean_latency, latency_std=latency_std, seed=seed)
    metrics = MetricsCollector(sim_duration=sim_duration)
    node_ids = [f"N{i}" for i in range(n_nodes)]
    nodes = []
    protocols = []

    for i, node_id in enumerate(node_ids):
        role = NodeRole.BYZANTINE if i < f_byzantine else NodeRole.HONEST
        node = Node(id=node_id, role=role)
        nodes.append(node)
        network.register_node(node)

    for i, node in enumerate(nodes):
        protocol = TendermintBFT(
            node_id=node.id,
            node_ids=node_ids,
            round_timeout=round_timeout,
            txs_per_block=20,
            seed=seed + i,
            byzantine=node.role == NodeRole.BYZANTINE,
        )
        protocols.append(protocol)
        env.process(protocol.run(env, node, network, metrics))

    if partition_at is not None:
        group1 = node_ids[: n_nodes // 2]
        group2 = node_ids[n_nodes // 2 :]

        def partitioner():
            yield env.timeout(partition_at)
            network.apply_partition(group1, group2)
            if heal_at is not None:
                yield env.timeout(heal_at - partition_at)
                network.heal_partition()

        env.process(partitioner())

    env.run(until=sim_duration)
    return nodes, protocols, metrics.snapshot(), metrics


def honest_protocols(protocols):
    return [protocol for protocol in protocols if not protocol.byzantine]


def assert_chains_agree(protocols):
    min_len = min(len(protocol.chain) for protocol in protocols)
    for height in range(min_len):
        expected_hash = protocols[0].chain[height].hash
        assert all(protocol.chain[height].hash == expected_hash for protocol in protocols)


def test_bft_commits_blocks():
    _nodes, protocols, _snapshot, _metrics = run_bft_sim(
        n_nodes=4,
        round_timeout=0.5,
        mean_latency=0.05,
        sim_duration=200.0,
        seed=42,
    )

    assert all(len(protocol.chain) - 1 >= 10 for protocol in protocols)
    assert_chains_agree(protocols)


def test_bft_safety_with_f_lt_n_over_3_byzantine():
    _nodes, protocols, snapshot, _metrics = run_bft_sim(
        n_nodes=10,
        f_byzantine=3,
        round_timeout=0.5,
        mean_latency=0.05,
        sim_duration=500.0,
        seed=42,
    )
    honest = honest_protocols(protocols)

    assert snapshot.safety_violations == 0
    assert_chains_agree(honest)


def test_bft_liveness_stalls_under_even_partition(caplog):
    caplog.set_level(logging.INFO)
    _nodes, protocols, snapshot, metrics = run_bft_sim(
        n_nodes=10,
        round_timeout=0.5,
        mean_latency=0.02,
        sim_duration=150.0,
        seed=42,
        partition_at=50.0,
    )
    baseline = metrics.bft_commits_before(50.0)
    during_partition = metrics.bft_commits_between(50.0, 150.0)
    logger.info("BFT partition commits: before=%s during=%s", baseline, during_partition)

    assert baseline > 0
    assert during_partition == 0
    assert snapshot.liveness_failures > 0


def test_bft_liveness_resumes_after_heal():
    _nodes, protocols, _snapshot, metrics = run_bft_sim(
        n_nodes=10,
        round_timeout=0.5,
        mean_latency=0.02,
        sim_duration=250.0,
        seed=42,
        partition_at=50.0,
        heal_at=150.0,
    )
    after_heal = metrics.bft_commits_between(150.0, 250.0)

    assert after_heal >= 5


def test_bft_message_complexity_quadratic():
    message_rates = []
    for n_nodes in [4, 10]:
        _nodes, _protocols, snapshot, metrics = run_bft_sim(
            n_nodes=n_nodes,
            round_timeout=0.5,
            mean_latency=0.01,
            sim_duration=8.0,
            seed=42,
        )
        committed = max(1, metrics.bft_total_commits())
        message_rates.append(snapshot.message_count / committed)

    ratio = message_rates[1] / message_rates[0]

    assert 4.0 <= ratio <= 10.0
