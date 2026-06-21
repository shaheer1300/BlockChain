"""
tests/unit/test_core.py

Full test suite for Phase 1. Write all tests before any implementation.
"""

import re

import numpy as np
import pytest

from src.consensus.base import ConsensusProtocol
from src.core.block import Block, genesis_block
from src.core.message import Message, MessageType
from src.core.metrics import MetricsCollector
from src.core.network import Network
from src.core.node import Node, NodeRole
from src.core.simulation import SimulationEnvironment


HEX_64_RE = re.compile(r"^[0-9a-f]{64}$")


class RecordingInbox(list):
    def __init__(self, env):
        super().__init__()
        self.env = env
        self.arrivals = []

    def append(self, message):
        self.arrivals.append((message, self.env.now))
        super().append(message)


def test_simulation_clock_advances():
    env = SimulationEnvironment(duration=500.0)

    env.run()

    assert env.now == 500.0


def test_network_message_delivery():
    env = SimulationEnvironment(duration=1.0)
    network = Network(env.env, mean_latency=0.1, latency_std=0.0, seed=42)
    node_a = Node(id="A", role=NodeRole.HONEST)
    node_b = Node(id="B", role=NodeRole.HONEST)
    node_b.inbox = RecordingInbox(env.env)
    network.register_node(node_a)
    network.register_node(node_b)
    message = Message(
        type=MessageType.GENERIC,
        sender="A",
        receiver="B",
        payload="hello",
        sent_at=0.0,
    )

    network.send("A", "B", message)
    env.run()

    assert len(node_b.inbox) == 1
    assert node_b.inbox[0].payload == "hello"
    assert node_b.inbox.arrivals[0][1] == pytest.approx(0.1, abs=0.01)


def test_network_latency_distribution():
    env = SimulationEnvironment(duration=2.0)
    network = Network(env.env, mean_latency=0.1, latency_std=0.02, seed=42)
    node_a = Node(id="A", role=NodeRole.HONEST)
    node_b = Node(id="B", role=NodeRole.HONEST)
    node_b.inbox = RecordingInbox(env.env)
    network.register_node(node_a)
    network.register_node(node_b)

    def schedule_messages():
        for i in range(500):
            sent_at = i * 0.001
            yield env.env.timeout(sent_at - env.env.now)
            network.send(
                "A",
                "B",
                Message(
                    type=MessageType.GENERIC,
                    sender="A",
                    receiver="B",
                    payload=i,
                    sent_at=sent_at,
                ),
            )

    env.env.process(schedule_messages())
    env.run()
    actual_latencies = np.array(
        [arrival_time - message.sent_at for message, arrival_time in node_b.inbox.arrivals]
    )

    assert len(actual_latencies) == 500
    assert 0.095 <= actual_latencies.mean() <= 0.105
    assert np.mean((0.04 <= actual_latencies) & (actual_latencies <= 0.16)) >= 0.95
    assert np.all(actual_latencies >= 0.0)


def test_network_partition_blocks_cross_group_messages():
    env = SimulationEnvironment(duration=1.0)
    network = Network(env.env, mean_latency=0.05, latency_std=0.0, seed=42)
    nodes = {node_id: Node(id=node_id, role=NodeRole.HONEST) for node_id in "ABCDEF"}
    for node in nodes.values():
        network.register_node(node)
    network.apply_partition(group1=["A", "B", "C"], group2=["D", "E", "F"])

    network.send(
        "A",
        "D",
        Message(MessageType.GENERIC, sender="A", receiver="D", payload="cross", sent_at=0.0),
    )
    network.send(
        "A",
        "B",
        Message(MessageType.GENERIC, sender="A", receiver="B", payload="inside", sent_at=0.0),
    )
    env.run()

    assert nodes["D"].inbox == []
    assert len(nodes["B"].inbox) == 1


def test_network_partition_heal():
    env = SimulationEnvironment(duration=1.0)
    network = Network(env.env, mean_latency=0.05, latency_std=0.0, seed=42)
    node_a = Node(id="A", role=NodeRole.HONEST)
    node_d = Node(id="D", role=NodeRole.HONEST)
    network.register_node(node_a)
    network.register_node(node_d)
    network.apply_partition(group1=["A"], group2=["D"])

    def scenario():
        yield env.env.timeout(0.1)
        network.send(
            "A",
            "D",
            Message(MessageType.GENERIC, sender="A", receiver="D", payload="dropped", sent_at=0.1),
        )
        yield env.env.timeout(0.4)
        network.heal_partition()
        yield env.env.timeout(0.1)
        network.send(
            "A",
            "D",
            Message(MessageType.GENERIC, sender="A", receiver="D", payload="delivered", sent_at=0.6),
        )

    env.env.process(scenario())
    env.run()

    assert len(node_d.inbox) == 1
    assert node_d.inbox[0].payload == "delivered"
    assert node_d.inbox[0].sent_at == pytest.approx(0.6)


def test_metrics_throughput():
    collector = MetricsCollector(sim_duration=100.0)

    for i in range(50):
        collector.record_tx_confirmed(tx_id=i, confirmed_at=float(i))

    assert collector.snapshot().throughput_tps == 0.5


def test_metrics_finality_latency():
    collector = MetricsCollector(sim_duration=100.0)

    collector.record_tx_submitted(tx_id="tx1", submitted_at=10.0)
    collector.record_tx_submitted(tx_id="tx2", submitted_at=20.0)
    collector.record_tx_confirmed(tx_id="tx1", confirmed_at=40.0)
    collector.record_tx_confirmed(tx_id="tx2", confirmed_at=70.0)

    assert collector.snapshot().mean_finality_latency_s == 40.0


def test_metrics_fork_rate():
    collector = MetricsCollector(sim_duration=100.0)

    for _ in range(10):
        collector.record_block_mined(block_id="B1")
    collector.record_block_orphaned(block_id="B3")
    collector.record_block_orphaned(block_id="B7")

    assert collector.snapshot().fork_rate == 0.2


def test_block_hash_deterministic():
    block = Block(
        height=1,
        prev_hash="0" * 64,
        payload_hash="abc123",
        timestamp=100.0,
        protocol_fields={},
    )
    identical = Block(
        height=1,
        prev_hash="0" * 64,
        payload_hash="abc123",
        timestamp=100.0,
        protocol_fields={},
    )
    changed = Block(
        height=2,
        prev_hash="0" * 64,
        payload_hash="abc123",
        timestamp=100.0,
        protocol_fields={},
    )

    assert HEX_64_RE.match(block.hash)
    assert identical.hash == block.hash
    assert changed.hash != block.hash


def test_genesis_block():
    genesis = genesis_block()

    assert genesis.height == 0
    assert genesis.prev_hash == "0" * 64
    assert HEX_64_RE.match(genesis.hash)
    assert genesis_block() == genesis


def test_consensus_protocol_is_abstract():
    with pytest.raises(TypeError):
        ConsensusProtocol()
