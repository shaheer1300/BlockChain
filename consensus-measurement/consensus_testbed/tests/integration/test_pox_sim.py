import importlib

import simpy

from src.core.metrics import MetricsCollector
from src.core.network import Network
from src.core.node import Node, NodeRole


def run_pox_sim(
    n_miners=3,
    n_stackers=2,
    base_block_time=10.0,
    k_confirmations=3,
    txs_per_pox_block=20,
    mean_latency=0.05,
    latency_std=0.0,
    sim_duration=500.0,
    seed=42,
):
    pox_module = importlib.import_module("src.consensus.pox")
    BaseChain = getattr(pox_module, "BaseChain")
    ProofOfTransfer = getattr(pox_module, "ProofOfTransfer")
    env = simpy.Environment()
    network = Network(env, mean_latency=mean_latency, latency_std=latency_std, seed=seed)
    metrics = MetricsCollector(sim_duration=sim_duration)
    miner_ids = [f"M{i}" for i in range(n_miners)]
    stacker_locked_stx = [float((i + 1) * 100) for i in range(n_stackers)]
    bids = [float((i + 1) * 10) for i in range(n_miners)]
    nodes = [Node(id=miner_id, role=NodeRole.HONEST) for miner_id in miner_ids]
    protocols = []

    for node in nodes:
        network.register_node(node)

    base_chain = BaseChain(
        base_block_time=base_block_time,
        subscriber_ids=miner_ids,
        seed=seed,
    )
    env.process(base_chain.run(env, network, metrics))

    for i, node in enumerate(nodes):
        protocol = ProofOfTransfer(
            node_id=node.id,
            miner_ids=miner_ids,
            bid_amount=bids[i],
            all_bids=bids,
            stacker_locked_stx=stacker_locked_stx,
            base_block_time=base_block_time,
            k_confirmations=k_confirmations,
            txs_per_pox_block=txs_per_pox_block,
            seed=seed,
        )
        protocols.append(protocol)
        env.process(protocol.run(env, node, network, metrics))

    env.run(until=sim_duration)
    return base_chain, protocols, metrics.snapshot(), metrics


def test_pox_produces_anchored_blocks():
    base_chain, protocols, _snapshot, _metrics = run_pox_sim(
        n_miners=3,
        n_stackers=2,
        base_block_time=10.0,
        k_confirmations=3,
        mean_latency=0.05,
        sim_duration=500.0,
        seed=42,
    )
    base_hashes = {block.hash for block in base_chain.blocks}
    pox_blocks = protocols[0].get_canonical_chain()

    assert len(pox_blocks) >= 40
    assert all(block.protocol_fields["anchor_hash"] in base_hashes for block in pox_blocks)


def test_stacker_total_rewards_equal_total_bids():
    _base_chain, _protocols, _snapshot, metrics = run_pox_sim(
        n_miners=3,
        n_stackers=2,
        base_block_time=10.0,
        k_confirmations=3,
        mean_latency=0.05,
        sim_duration=500.0,
        seed=42,
    )

    assert abs(metrics.pox_total_bids_paid() - metrics.pox_total_rewards_received()) < 0.001


def test_pox_throughput_bounded_by_base_block_time():
    _base_chain, _protocols, snapshot, _metrics = run_pox_sim(
        n_miners=5,
        n_stackers=3,
        base_block_time=10.0,
        k_confirmations=3,
        txs_per_pox_block=20,
        sim_duration=1000.0,
        seed=42,
    )
    theoretical_max_tps = 20 / 10.0

    assert snapshot.throughput_tps <= theoretical_max_tps * 1.05


def test_pox_finality_latency_proportional_to_k():
    _base_a, _protocols_a, snapshot_a, _metrics_a = run_pox_sim(
        n_miners=3,
        n_stackers=2,
        base_block_time=10.0,
        k_confirmations=3,
        sim_duration=500.0,
        seed=42,
    )
    _base_b, _protocols_b, snapshot_b, _metrics_b = run_pox_sim(
        n_miners=3,
        n_stackers=2,
        base_block_time=10.0,
        k_confirmations=6,
        sim_duration=500.0,
        seed=42,
    )
    ratio = snapshot_b.mean_finality_latency_s / snapshot_a.mean_finality_latency_s

    assert 1.8 <= ratio <= 2.2
