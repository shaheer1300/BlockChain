from functools import lru_cache
import importlib

import simpy

from src.consensus.bft import TendermintBFT
from src.core.metrics import MetricsCollector
from src.core.network import Network
from src.core.node import Node, NodeRole


def require_partitioner():
    return getattr(importlib.import_module("src.adversarial.partitioner"), "NetworkPartitioner")


@lru_cache(maxsize=1)
def run_pow_partition_scenario():
    NetworkPartitioner = require_partitioner()
    node_ids = [f"N{i}" for i in range(20)]
    return NetworkPartitioner.simulate_pow_partition(
        node_ids=node_ids,
        group1=[f"N{i}" for i in range(10)],
        group2=[f"N{i}" for i in range(10, 20)],
        partition_start=500.0,
        partition_end=1500.0,
        target_block_time=10.0,
        seed=42,
    )


def run_bft_partition_scenario(heal=False):
    NetworkPartitioner = require_partitioner()
    end_time = 400.0 if heal else 250.0
    env = simpy.Environment()
    network = Network(env, mean_latency=0.02, latency_std=0.0, seed=42)
    metrics = MetricsCollector(sim_duration=end_time)
    node_ids = [f"N{i}" for i in range(10)]
    nodes = [Node(id=node_id, role=NodeRole.HONEST) for node_id in node_ids]
    protocols = []

    for node in nodes:
        network.register_node(node)
    for index, node in enumerate(nodes):
        protocol = TendermintBFT(
            node_id=node.id,
            node_ids=node_ids,
            round_timeout=0.5,
            txs_per_block=20,
            seed=42 + index,
        )
        protocols.append(protocol)
        env.process(protocol.run(env, node, network, metrics))

    partitioner = NetworkPartitioner(env, network)
    partitioner.schedule_partition(node_ids[:5], node_ids[5:], start_time=50.0)
    if heal:
        partitioner.schedule_heal(heal_time=250.0)
    env.run(until=end_time)
    return protocols, metrics.snapshot(), metrics


def assert_bft_chains_agree(protocols):
    min_len = min(len(protocol.chain) for protocol in protocols)
    for height in range(min_len):
        expected_hash = protocols[0].chain[height].hash
        assert all(protocol.chain[height].hash == expected_hash for protocol in protocols)


def test_pow_liveness_preserved_under_partition():
    result = run_pow_partition_scenario()
    group1 = [f"N{i}" for i in range(10)]
    group2 = [f"N{i}" for i in range(10, 20)]

    assert max(result["final"][node_id] for node_id in group1) > max(result["baseline"][node_id] for node_id in group1)
    assert max(result["final"][node_id] for node_id in group2) > max(result["baseline"][node_id] for node_id in group2)


def test_pow_consistency_broken_under_partition():
    result = run_pow_partition_scenario()

    assert result["tips"]["group1"] != result["tips"]["group2"]


def test_bft_stalls_under_5050_partition():
    _protocols, snapshot, metrics = run_bft_partition_scenario(heal=False)

    assert metrics.bft_commits_before(50.0) > 0
    assert metrics.bft_commits_between(50.0, 250.0) == 0
    assert snapshot.safety_violations == 0


def test_bft_resumes_after_heal():
    protocols, _snapshot, metrics = run_bft_partition_scenario(heal=True)

    assert metrics.bft_commits_between(250.0, 400.0) >= 5
    assert_bft_chains_agree(protocols)
