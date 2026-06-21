from pathlib import Path
import sys

import pytest

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from src.core.block import genesis_block
from src.core.network import Network
from src.core.node import Node, NodeRole
from src.core.simulation import SimulationEnvironment


@pytest.fixture
def sim_env():
    env = SimulationEnvironment(duration=100.0)
    network = Network(env.env, mean_latency=0.05, latency_std=0.0, seed=42)
    nodes = [Node(id=f"N{i}", role=NodeRole.HONEST) for i in range(5)]
    for node in nodes:
        network.register_node(node)
    env.network = network
    env.nodes = nodes
    return env


@pytest.fixture
def genesis():
    return genesis_block()
