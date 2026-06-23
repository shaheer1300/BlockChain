## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Research Context](#2-research-context)
3. [Architecture & Core Abstractions](#3-architecture--core-abstractions)
4. [Repository Structure](#4-repository-structure)
5. [Global Implementation Rules](#5-global-implementation-rules)
6. [Phase 1 — Simulation Core & Network Layer](#6-phase-1--simulation-core--network-layer)
7. [Phase 2 — Proof of Work Consensus](#7-phase-2--proof-of-work-consensus)
8. [Phase 3 — BFT Consensus (Tendermint-style)](#8-phase-3--bft-consensus-tendermint-style)
9. [Phase 4 — Proof of Transfer (PoX)](#9-phase-4--proof-of-transfer-pox)
10. [Phase 5 — Adversarial Scenarios](#10-phase-5--adversarial-scenarios)
11. [Phase 6 — Experiment Runner & Analysis](#11-phase-6--experiment-runner--analysis)
12. [Running the Full Suite](#12-running-the-full-suite)
13. [Expected Research Outputs](#13-expected-research-outputs)


This README is the **sole source of truth** for this project.

---

## 1. Project Overview

This project is a **discrete-event simulation testbed** that implements three distinct blockchain consensus mechanisms from first principles and subjects each to a battery of controlled experiments.

**What it produces:**
- A Python simulation engine (built on SimPy) capable of modeling hundreds of nodes exchanging messages over a configurable network.
- Three pluggable consensus protocol implementations: Proof of Work (PoW), Tendermint BFT, and Proof of Transfer (PoX).
- An adversarial scenario harness: network partitions, Byzantine nodes, selfish miners, variable message delay.
- A statistical experiment runner that executes multi-trial experiments, computes confidence intervals, and generates publication-quality plots.

**What it is not:**
- Not a real blockchain implementation. Cryptographic operations are stubbed with deterministic hashes. The goal is behavioral fidelity, not production security.
- Not a distributed system. All nodes run in the same Python process via SimPy coroutines. Network effects are modeled probabilistically.

---

## 2. Research Context

**The three research claims this testbed must produce empirical evidence for:**

| Claim | Expected Observation |
|---|---|
| C1: Fork rate in PoW scales with `network_latency / block_time` | Fork rate increases monotonically as latency increases; Pearson r > 0.95 |
| C2: BFT (Tendermint) achieves deterministic finality but sacrifices liveness under partition | Zero committed blocks during a 50/50 partition; instant finality otherwise |
| C3: PoX inherits PoW's liveness properties and anchoring latency | PoX finality = k × base_block_time; throughput bounded by base chain's block rate |

Every phase in this project exists to produce the infrastructure needed to test these three claims.

---

## 3. Architecture & Core Abstractions

### 3.1 Simulation Model

The engine uses **SimPy** (discrete-event simulation). Every entity — nodes, the network, protocol timers — is a SimPy `Process`. Time is in **simulated seconds** (floats). There is no real-clock dependency.

```
SimulationEnvironment (wraps simpy.Environment)
    │
    ├── Network  (message routing, latency model, partition controller)
    │       └── MessageQueue per (sender, receiver) pair
    │
    ├── Node[]  (one per simulated peer; runs consensus process)
    │       ├── ConsensusProtocol  (pluggable: PoW | BFT | PoX)
    │       ├── Mempool
    │       └── LocalChain
    │
    └── MetricsCollector  (records events; computes derived metrics at end)
```

### 3.2 The Three Abstraction Contracts

Every implementation in this project must respect these three interfaces. Define them as Python Abstract Base Classes (ABCs) in Phase 1. All consensus implementations in Phases 2–4 must subclass them.

**`ConsensusProtocol` (ABC)**
```
run(env, node, network, metrics) -> SimPy generator
on_message(msg) -> None
get_canonical_chain() -> List[Block]
```

**`Block` (dataclass)**
```
height: int
prev_hash: str
payload_hash: str       # deterministic hash of transactions
timestamp: float        # simulated time
protocol_fields: dict   # protocol-specific (e.g., nonce for PoW, round for BFT)
hash: str               # SHA-256(height | prev_hash | payload_hash | protocol_fields)
```

**`MetricsSnapshot` (dataclass)**
```
throughput_tps: float           # confirmed txns / sim_duration
mean_finality_latency_s: float  # average time from tx submit to finality
fork_rate: float                # orphaned_blocks / total_mined_blocks
safety_violations: int          # double-commits detected
liveness_failures: int          # consensus stalls detected
message_count: int              # total messages sent
```

---

## 4. Repository Structure

Create this exact directory layout at the start of Phase 1. All files listed under `src/` and `tests/` must exist by the end of their respective phases.

```
consensus_testbed/
├── README.md
├── requirements.txt
├── pyproject.toml
├── config/
│   ├── pow_default.yaml
│   ├── bft_default.yaml
│   └── pox_default.yaml
├── src/
│   ├── __init__.py
│   ├── core/
│   │   ├── __init__.py
│   │   ├── simulation.py       # SimPy wrapper, SimulationEnvironment
│   │   ├── network.py          # Network, PartitionController, latency model
│   │   ├── node.py             # Node base class, NodeRole enum
│   │   ├── message.py          # Message dataclass, MessageType enum
│   │   ├── block.py            # Block dataclass, genesis_block()
│   │   └── metrics.py          # MetricsCollector, MetricsSnapshot
│   ├── consensus/
│   │   ├── __init__.py
│   │   ├── base.py             # ConsensusProtocol ABC
│   │   ├── pow.py              # ProofOfWork
│   │   ├── bft.py              # TendermintBFT
│   │   └── pox.py              # ProofOfTransfer
│   ├── adversarial/
│   │   ├── __init__.py
│   │   ├── byzantine.py        # ByzantineNode, EquivocatingNode
│   │   ├── partitioner.py      # NetworkPartitioner (scheduled partition events)
│   │   └── selfish_miner.py    # SelfishMiner (PoW strategy)
│   ├── experiments/
│   │   ├── __init__.py
│   │   ├── runner.py           # ExperimentRunner, ExperimentConfig
│   │   └── scenarios.py        # Predefined scenario factory functions
│   └── analysis/
│       ├── __init__.py
│       ├── stats.py            # compute_ci(), summarize()
│       └── visualizer.py       # generate_comparison_plots()
├── tests/
│   ├── conftest.py             # Shared fixtures: env, small_network, genesis
│   ├── unit/
│   │   ├── test_core.py        # Phase 1 tests
│   │   ├── test_pow.py         # Phase 2 unit tests
│   │   ├── test_bft.py         # Phase 3 unit tests
│   │   └── test_pox.py         # Phase 4 unit tests
│   ├── integration/
│   │   ├── test_pow_sim.py     # Phase 2 integration tests
│   │   ├── test_bft_sim.py     # Phase 3 integration tests
│   │   └── test_pox_sim.py     # Phase 4 integration tests
│   └── adversarial/
│       ├── test_partition.py   # Phase 5 partition tests
│       ├── test_byzantine.py   # Phase 5 Byzantine tests
│       └── test_selfish.py     # Phase 5 selfish mining tests
├── scripts/
│   ├── run_experiment.py       # CLI: python scripts/run_experiment.py --protocol pow
│   └── generate_report.py      # Generates plots and summary CSV
└── results/
    └── .gitkeep
```

---

## 5. Global Implementation Rules

These rules apply to every phase without exception. The LLM must follow them before writing any code.

### 5.1 Dependency Manifest

The `requirements.txt` must contain exactly:
```
simpy==4.1.1
numpy>=1.26
scipy>=1.12
pandas>=2.2
matplotlib>=3.8
seaborn>=0.13
pyyaml>=6.0
pytest>=8.0
pytest-cov>=5.0
click>=8.1
```

### 5.2 Test-First Mandate

**No implementation file may be written before its corresponding test file.** The workflow for every step is:
1. Write the test file in full.
2. Run `pytest <test_file> -v`. All tests must **fail** (not error) at this point — failing means the test ran and asserted incorrectly, not that there was an import error.
3. Write the implementation.
4. Run `pytest <test_file> -v`. All tests must **pass**.

### 5.3 Deterministic Randomness

Every stochastic element — mining time, network latency jitter, Byzantine behavior — must accept a `seed: int` parameter. Use `numpy.random.default_rng(seed)` throughout. The default seed for all tests is `42`.

### 5.4 Configuration Contract

Every experiment is driven by a YAML config. All configurable parameters must be readable from the YAML files in `config/`. Hard-coded magic numbers in `src/` are not permitted. Default values belong in the YAML files.

### 5.5 Simulated Time Units

All time values in the codebase are in **simulated seconds** (Python `float`). No real-time `time.sleep()` calls are permitted anywhere.

### 5.6 Logging

Use Python's `logging` module (not `print`). Set level `INFO` for simulation milestones (block mined, round completed) and `DEBUG` for message-level events. Tests should not produce log output unless a test explicitly checks log content.

---

## 6. Phase 1 — Simulation Core & Network Layer

---

> **📋 AGENT CONTEXT — Phase 1**
>
> **Prior work:** None. This is the first phase.
>
> **Your task:** Build the simulation foundation. Every subsequent phase depends on what you build here. Take precision seriously.
>
> **Constraint:** Do not implement any consensus logic. If you find yourself writing anything related to block mining, voting rounds, or fork choice, stop — that belongs in a later phase.
>
> **Required reading before starting:** Review the three abstraction contracts in Section 3.2. Your ABCs here must exactly match those signatures.

---

### Goal

Deliver a working SimPy-based simulation environment with a configurable network model, message abstraction, and metrics collection — all fully tested.

### Deliverables

- `src/core/simulation.py`
- `src/core/network.py`
- `src/core/node.py`
- `src/core/message.py`
- `src/core/block.py`
- `src/core/metrics.py`
- `src/consensus/base.py`
- `tests/conftest.py`
- `tests/unit/test_core.py`
- `requirements.txt`, `pyproject.toml`

---

### Step 1.1 — Write Tests First

Create `tests/unit/test_core.py` with the following tests in full **before writing any `src/` code**.

Each test function is specified with its setup, action, and required assertions. Do not add extra logic — implement exactly what is described.

```python
"""
tests/unit/test_core.py

Full test suite for Phase 1. Write all tests before any implementation.
"""

# TEST 1: test_simulation_clock_advances
# Purpose: Verify the SimPy environment advances time correctly.
# Setup: Create a SimulationEnvironment with duration=500.0
# Action: Run the environment to completion.
# Assert: env.now == 500.0

# TEST 2: test_network_message_delivery
# Purpose: Verify a message sent from node A to node B is delivered.
# Setup:
#   - Create a network with mean_latency=0.1, latency_std=0.0 (deterministic)
#   - Create two nodes: node_a (id="A"), node_b (id="B")
#   - Register both nodes with the network
#   - node_b has a message inbox list []
# Action:
#   - At sim time 0, node_a sends Message(type=GENERIC, sender="A", receiver="B",
#     payload="hello", sent_at=0.0) to node_b
#   - Run simulation for duration=1.0
# Assert:
#   - node_b.inbox contains exactly one message
#   - The message payload == "hello"
#   - The message is delivered at sim time approximately 0.1 (within ±0.01)

# TEST 3: test_network_latency_distribution
# Purpose: Verify latency samples follow a Gaussian distribution.
# Setup:
#   - Network with mean_latency=0.1, latency_std=0.02, seed=42
#   - Two nodes: "A", "B"
# Action:
#   - Schedule 500 messages from A to B, each at sim_time=i*0.001 for i in range(500)
#   - Record arrival times. Compute actual_latencies = arrival_time - sent_time
# Assert:
#   - mean(actual_latencies) is within 5% of 0.1 (i.e., in [0.095, 0.105])
#   - 95% of actual_latencies fall within [0.04, 0.16]
#   - No latency is negative

# TEST 4: test_network_partition_blocks_cross_group_messages
# Purpose: Verify partitioned nodes cannot communicate.
# Setup:
#   - Network with mean_latency=0.05, latency_std=0.0
#   - Nodes: ["A", "B", "C", "D", "E", "F"], all registered
#   - Partition: group1=["A","B","C"], group2=["D","E","F"]
# Action:
#   - At t=0: node "A" sends a message to node "D"
#   - At t=0: node "A" sends a message to node "B"
#   - Run simulation for duration=1.0
# Assert:
#   - node "D".inbox is empty (cross-partition message was dropped)
#   - node "B".inbox has exactly one message (intra-partition message delivered)

# TEST 5: test_network_partition_heal
# Purpose: Verify messages flow again after partition heals.
# Setup:
#   - Network with mean_latency=0.05, latency_std=0.0
#   - Nodes: ["A", "D"] registered
#   - Partition: group1=["A"], group2=["D"] applied at t=0
# Action:
#   - At t=0.1: node "A" sends message to "D"  → should be dropped
#   - At t=0.5: heal the partition
#   - At t=0.6: node "A" sends message to "D"  → should be delivered
#   - Run simulation for duration=1.0
# Assert:
#   - node "D".inbox has exactly one message
#   - That message was sent at t=0.6 (payload distinguishes them)

# TEST 6: test_metrics_throughput
# Purpose: Verify throughput calculation.
# Setup:
#   - MetricsCollector(sim_duration=100.0)
# Action:
#   - Call collector.record_tx_confirmed(tx_id=i, confirmed_at=float(i)) for i in range(50)
# Assert:
#   - collector.snapshot().throughput_tps == 50 / 100.0 == 0.5

# TEST 7: test_metrics_finality_latency
# Purpose: Verify finality latency is averaged correctly.
# Setup:
#   - MetricsCollector(sim_duration=100.0)
# Action:
#   - record_tx_submitted(tx_id="tx1", submitted_at=10.0)
#   - record_tx_submitted(tx_id="tx2", submitted_at=20.0)
#   - record_tx_confirmed(tx_id="tx1", confirmed_at=40.0)   # latency = 30.0
#   - record_tx_confirmed(tx_id="tx2", confirmed_at=70.0)   # latency = 50.0
# Assert:
#   - collector.snapshot().mean_finality_latency_s == 40.0  # (30+50)/2

# TEST 8: test_metrics_fork_rate
# Purpose: Verify fork rate calculation.
# Setup:
#   - MetricsCollector(sim_duration=100.0)
# Action:
#   - record_block_mined(block_id="B1")  ×10   (10 blocks total)
#   - record_block_orphaned(block_id="B3")
#   - record_block_orphaned(block_id="B7")
# Assert:
#   - collector.snapshot().fork_rate == 2 / 10 == 0.2

# TEST 9: test_block_hash_deterministic
# Purpose: Verify Block hash is consistent and unique.
# Setup: Create Block(height=1, prev_hash="0"*64, payload_hash="abc123",
#   timestamp=100.0, protocol_fields={})
# Assert:
#   - block.hash is a 64-character hex string
#   - Creating an identical Block produces the same hash
#   - Changing any field (e.g., height=2) produces a different hash

# TEST 10: test_genesis_block
# Purpose: Verify genesis block has correct structure.
# Action: Call genesis = genesis_block()
# Assert:
#   - genesis.height == 0
#   - genesis.prev_hash == "0" * 64
#   - genesis.hash is a valid 64-char hex string
#   - genesis_block() called twice returns equal (but not necessarily identical) objects

# TEST 11: test_consensus_protocol_is_abstract
# Purpose: Verify ConsensusProtocol cannot be instantiated directly.
# Action: Attempt to instantiate ConsensusProtocol()
# Assert: Raises TypeError
```

### Step 1.2 — Implement

After all tests are written and confirmed to fail (not error), implement the following in order. Do not skip ahead.

**`src/core/message.py`**
Define a `MessageType` enum with values: `GENERIC`, `BLOCK_ANNOUNCE`, `TX_ANNOUNCE`, `VOTE`, `PROPOSAL`, `TIMEOUT`. Define a frozen `Message` dataclass with fields: `type: MessageType`, `sender: str`, `receiver: str`, `payload: Any`, `sent_at: float`.

**`src/core/block.py`**
Define a frozen `Block` dataclass matching the contract in Section 3.2. The `hash` field is computed in `__post_init__` as `hashlib.sha256(canonical_string.encode()).hexdigest()` where `canonical_string = f"{height}|{prev_hash}|{payload_hash}|{timestamp}|{json.dumps(protocol_fields, sort_keys=True)}"`. Define `genesis_block()` as a module-level function returning a `Block` with height=0 and all fields set to their zero values.

**`src/core/metrics.py`**
Define `MetricsCollector` with internal dicts tracking submitted and confirmed tx timestamps, plus counters for blocks mined and orphaned. The `snapshot()` method computes and returns a `MetricsSnapshot`. Do not compute metrics incrementally — compute them all in `snapshot()` at the end.

**`src/core/network.py`**
Define a `Network` class wrapping the SimPy environment. Key design decisions:
- `register_node(node)` adds a node to the routing table.
- `send(sender_id, receiver_id, message)` schedules delivery using `env.process(self._deliver(message, latency))` where latency is sampled from `N(mean_latency, latency_std)` clipped to a minimum of 0.
- `apply_partition(group1: List[str], group2: List[str])` sets an internal partition state. Messages crossing the partition are silently dropped (not delayed).
- `heal_partition()` clears partition state.
- Node inboxes are lists accessed via `node.inbox.append(message)`.

**`src/core/node.py`**
Define a `NodeRole` enum with values: `HONEST`, `BYZANTINE`. Define a `Node` dataclass with fields: `id: str`, `role: NodeRole`, `inbox: List[Message]` (default empty list). This is a dumb container — all logic lives in the consensus protocol.

**`src/core/simulation.py`**
Define `SimulationEnvironment` as a thin wrapper around `simpy.Environment`. It holds the network and a list of nodes. `run(duration)` calls `env.run(until=duration)`.

**`src/consensus/base.py`**
Define `ConsensusProtocol` as an ABC with three abstract methods matching Section 3.2. Mark all three with `@abstractmethod`.

**`tests/conftest.py`**
Define these shared pytest fixtures:
- `sim_env`: Returns a `SimulationEnvironment` with 5 nodes (ids "N0"–"N4") registered on a network with mean_latency=0.05, latency_std=0.0, seed=42.
- `genesis`: Returns `genesis_block()`.

### Acceptance Criteria — Phase 1

All of the following must be true before proceeding to Phase 2:

- [ ] `pytest tests/unit/test_core.py -v` passes with **0 failures, 0 errors**
- [ ] `pytest --co tests/unit/test_core.py` lists exactly **11 test items**
- [ ] `python -c "from src.consensus.base import ConsensusProtocol"` succeeds with no errors
- [ ] `python -c "from src.core.block import genesis_block; b = genesis_block(); assert len(b.hash) == 64"` succeeds
- [ ] No `print()` statements exist in any `src/` file

### Phase 1 Handoff

At the end of Phase 1, the following exist and all tests pass:
```
src/core/  (6 files, all implemented)
src/consensus/base.py
tests/conftest.py
tests/unit/test_core.py  (11 tests, all green)
requirements.txt
pyproject.toml
config/  (3 empty YAML stubs)
```

---

## 7. Phase 2 — Proof of Work Consensus

---

> **📋 AGENT CONTEXT — Phase 2**
>
> **Prior phases complete:** Phase 1 (Simulation Core). Run `pytest tests/unit/test_core.py` before starting — it must be all green.
>
> **Do not modify:** Anything in `src/core/`, `src/consensus/base.py`, or `tests/unit/test_core.py`.
>
> **You are implementing:** A Nakamoto-style Proof of Work consensus protocol.
>
> **Simplification permitted:** Real SHA-256 mining is too slow for simulation. Model mining time as a Poisson process: if a miner has fraction `f` of total hash power and the network's target block time is `T` seconds, that miner's time to next block is drawn from `Exponential(mean = T / f)`. Block hashes are pre-computed using hashlib, not mined by brute force.

---

### Goal

Implement a Nakamoto PoW protocol in which N nodes mine blocks independently, propagate them over the simulated network, and resolve forks using the heaviest-chain rule. Measure throughput, fork rate, and finality latency as a function of network latency.

### Deliverables

- `src/consensus/pow.py`
- `src/core/mempool.py`
- `tests/unit/test_pow.py`
- `tests/integration/test_pow_sim.py`
- `config/pow_default.yaml`

---

### Step 2.1 — Write Tests First

Create both test files in full before any implementation.

**`tests/unit/test_pow.py`** — Unit tests for PoW data structures and logic:

```python
# TEST 1: test_pow_block_valid_structure
# Setup: Create PoWBlock(height=1, prev_hash=genesis.hash,
#   payload_hash="tx123", timestamp=100.0,
#   protocol_fields={"difficulty": 4})
# Assert:
#   - block.hash is a 64-char hex string
#   - block.height == 1
#   - block.protocol_fields["difficulty"] == 4

# TEST 2: test_fork_choice_selects_heaviest_chain
# Setup: Build two chains from genesis:
#   chain_a = [genesis, b1a, b2a, b3a]          (height 3)
#   chain_b = [genesis, b1b, b2b, b3b, b4b]     (height 4)
# Action: result = fork_choice([chain_a, chain_b])
# Assert: result == chain_b

# TEST 3: test_fork_choice_tie_selects_first_seen
# Setup: Two chains of equal height 3, chain_a registered first.
# Action: result = fork_choice([chain_a, chain_b])
# Assert: result == chain_a

# TEST 4: test_mempool_add_and_select
# Setup: Mempool(max_size=10, seed=42)
# Action: Add 12 transactions with tx_ids ["tx0".."tx11"]
# Assert:
#   - len(mempool) == 10   (max_size respected)
#   - mempool.select(n=5) returns 5 tx_ids
#   - All returned tx_ids are in the mempool

# TEST 5: test_mempool_remove_confirmed
# Setup: Mempool with tx_ids ["tx0".."tx4"] added
# Action: mempool.remove_confirmed(["tx1", "tx3"])
# Assert:
#   - "tx1" not in mempool
#   - "tx3" not in mempool
#   - len(mempool) == 3

# TEST 6: test_transaction_count_per_block
# Setup: PoW config with txs_per_block=10
# Create a PoW miner with 10 pending txs in mempool
# Action: miner mines one block
# Assert: block.protocol_fields["tx_count"] == 10

# TEST 7: test_mining_time_distribution
# Purpose: Verify mining time follows correct exponential distribution.
# Setup: seed=42, hash_fraction=0.5, target_block_time=10.0
# Action: Sample 1000 mining times from PoW._sample_mining_time()
# Assert:
#   - mean(times) is within 10% of (10.0 / 0.5) = 20.0
#     i.e., in [18.0, 22.0]
#   - min(times) > 0
```

**`tests/integration/test_pow_sim.py`** — Integration tests for the full PoW simulation:

```python
# TEST 8: test_pow_blocks_mined
# Purpose: Verify blocks are actually produced in simulation.
# Setup:
#   - 5 nodes, equal hash power (each 0.2 fraction)
#   - mean_latency=0.05, target_block_time=10.0, txs_per_block=5
#   - seed=42, sim_duration=500.0
# Action: Run simulation, collect metrics.
# Assert:
#   - metrics.message_count > 0
#   - Total blocks in all nodes' canonical chains > 0
#   - At least one node has canonical chain height >= 30
#     (500s / 10s_per_block = 50 expected; 30 is a conservative lower bound)

# TEST 9: test_fork_rate_monotone_with_latency
# Purpose: Verify the research claim C1 from Section 2.
# Setup: 10 nodes, equal hash power, target_block_time=10.0, txs_per_block=5, seed=42
# Run three separate simulations (sim_duration=2000.0 each):
#   Exp A: mean_latency=0.010  (10ms)
#   Exp B: mean_latency=0.100  (100ms)
#   Exp C: mean_latency=1.000  (1000ms)
# Collect fork_rate from metrics.snapshot() for each.
# Assert:
#   - fork_rate_A < fork_rate_B < fork_rate_C   (strict monotone)
#   - fork_rate_A < 0.05   (< 5% at 10ms latency)
#   - fork_rate_C > 0.05   (> 5% at 1000ms latency)

# TEST 10: test_block_propagation_coverage
# Purpose: Verify a mined block reaches most peers.
# Setup:
#   - 10 nodes, mean_latency=0.1, latency_std=0.02
#   - Run PoW simulation for sim_duration=300.0, seed=42
# After simulation:
#   - Identify the block at height 5 in node N0's chain (call its hash H)
#   - Count how many nodes have a block with hash H in their canonical chain
# Assert: count >= 8 (80% of 10 nodes have the block)

# TEST 11: test_pow_throughput_bounded
# Purpose: Verify throughput does not exceed theoretical maximum.
# Setup: 5 nodes, target_block_time=10.0, txs_per_block=20
#   mean_latency=0.1, sim_duration=1000.0, seed=42
# theoretical_max_tps = txs_per_block / target_block_time = 2.0
# Assert:
#   - metrics.throughput_tps <= theoretical_max_tps * 1.05  (5% tolerance for rounding)
```

### Step 2.2 — Implement

**`src/core/mempool.py`**
Implement `Mempool(max_size: int, seed: int)`. When `add(tx_id: str)` is called and the pool is full, evict a random transaction (not the incoming one). Implement `select(n: int) -> List[str]` to return n tx_ids without removal. Implement `remove_confirmed(tx_ids: List[str])`.

**`src/consensus/pow.py`**
Implement `ProofOfWork(ConsensusProtocol)` with:
- `hash_fraction: float` — this node's share of total network hash power.
- `target_block_time: float` — network-wide target seconds between blocks.
- `txs_per_block: int` — max transactions per block.
- `run()` generator: loop forever — sample mining time, `yield env.timeout(mining_time)`, build block, add to local chain, announce to all peers via `BLOCK_ANNOUNCE` message.
- `on_message(msg)` handler: on `BLOCK_ANNOUNCE`, validate the block is a valid extension (prev_hash matches tip), apply fork choice, relay to other peers (gossip).
- `get_canonical_chain()` returns the current heaviest chain.

**Block validation rules for PoW:**
1. `block.prev_hash` must match the hash of the current chain tip.
2. `block.height` must equal `current_tip.height + 1`.
3. `block.hash` must be a valid 64-char hex string.
4. Block must not already be in the chain (no duplicates).

**`config/pow_default.yaml`**
```yaml
pow:
  n_nodes: 10
  target_block_time: 10.0
  txs_per_block: 20
  mean_latency: 0.100
  latency_std: 0.020
  sim_duration: 2000.0
  seed: 42
```

### Acceptance Criteria — Phase 2

- [ ] `pytest tests/unit/test_pow.py -v` passes with **0 failures**
- [ ] `pytest tests/integration/test_pow_sim.py -v` passes with **0 failures**
- [ ] `pytest tests/unit/test_core.py -v` still passes (no regressions)
- [ ] Fork rates from TEST 9 are logged at INFO level so values are visible in CI output
- [ ] No consensus logic exists in `src/core/` (it all lives in `src/consensus/pow.py`)

### Phase 2 Handoff

```
src/consensus/pow.py     (PoW protocol)
src/core/mempool.py      (Mempool)
tests/unit/test_pow.py   (7 unit tests, all green)
tests/integration/test_pow_sim.py  (4 integration tests, all green)
config/pow_default.yaml
```
All prior tests remain green.

---

## 8. Phase 3 — BFT Consensus (Tendermint-style)

---

> **📋 AGENT CONTEXT — Phase 3**
>
> **Prior phases complete:** Phase 1 and Phase 2. All 22 tests are green.
>
> **Do not modify:** Anything in `src/core/`, `src/consensus/base.py`, `src/consensus/pow.py`.
>
> **You are implementing:** A simplified Tendermint BFT protocol. You are NOT implementing the full Tendermint v0.34 spec. Implement the minimum faithful model of the safety/liveness properties.
>
> **Key simplification:** Use a round-robin proposer selection (deterministic, based on round number mod n_nodes). Do not implement VRF-based selection.

---

### Goal

Implement a Tendermint-style BFT consensus protocol with two message phases (Prevote and Precommit), a timeout-based view-change, and provable safety under f < n/3 Byzantine nodes.

### Deliverables

- `src/consensus/bft.py`
- `tests/unit/test_bft.py`
- `tests/integration/test_bft_sim.py`
- `config/bft_default.yaml`

---

### Step 3.1 — Write Tests First

**`tests/unit/test_bft.py`** — Unit tests for BFT state machine logic:

```python
# TEST 1: test_proposer_selection_round_robin
# Setup: BFT with node_ids=["N0","N1","N2","N3"] (n=4)
# Assert:
#   - proposer_for_round(round=0) == "N0"
#   - proposer_for_round(round=1) == "N1"
#   - proposer_for_round(round=4) == "N0"   (wraps around)
#   - proposer_for_round(round=7) == "N3"

# TEST 2: test_quorum_threshold
# Setup: BFT with n_nodes=10
# Assert:
#   - quorum_size() == 7   (ceil(2 * 10 / 3) = 7)
# Setup: BFT with n_nodes=4
# Assert:
#   - quorum_size() == 3   (ceil(2 * 4 / 3) = 3)

# TEST 3: test_prevote_phase_accumulates_votes
# Setup: BFT node N0, round=1, height=1, n_nodes=4 (quorum=3)
# Action: N0 receives Prevote messages for block B from ["N1", "N2"]
# Assert: N0 has NOT yet advanced to Precommit (only 2 of 3 needed)
# Action: N0 receives Prevote from "N3" for block B
# Assert: N0 advances to Precommit phase for block B

# TEST 4: test_precommit_phase_commits_block
# Setup: BFT node N0, round=1, height=1, n_nodes=4 (quorum=3)
# Action: Simulate receiving 3 Precommit messages for block B
# Assert: N0 commits block B (local chain height becomes 1)

# TEST 5: test_equivocation_detection
# Setup: BFT node N0, round=1, height=1, n_nodes=4
# Action: Receive Prevote from "N1" for block B
#         Receive Prevote from "N1" for block B'  (different block, same round/height)
# Assert:
#   - N0 detects equivocation (logs a WARNING containing "equivocation")
#   - N0 does NOT count both votes (N1's second vote is discarded)

# TEST 6: test_view_change_on_timeout
# Setup: BFT node N0 is not the proposer in round 1. Timeout=0.5s simulated.
# Action: Run simulation for 1.0 simulated seconds without a proposal arriving.
# Assert:
#   - N0 sends a TIMEOUT message
#   - N0 advances to round 2
#   - N0 is now waiting for the proposer of round 2
```

**`tests/integration/test_bft_sim.py`** — Integration tests for the full BFT simulation:

```python
# TEST 7: test_bft_commits_blocks
# Setup: 4 nodes, all HONEST, n_nodes=4, round_timeout=0.5,
#   mean_latency=0.05, sim_duration=200.0, seed=42
# Assert:
#   - All 4 nodes have committed at least 10 blocks
#   - All 4 nodes agree on the same block hash at each height
#     (i.e., node_0.chain[h].hash == node_k.chain[h].hash for all h and k)

# TEST 8: test_bft_safety_with_f_lt_n_over_3_byzantine
# Setup: n=10, f=3 Byzantine equivocating nodes, round_timeout=0.5,
#   mean_latency=0.05, sim_duration=500.0, seed=42
# Byzantine behavior: send conflicting Prevote messages (vote for two different blocks)
# Assert:
#   - metrics.safety_violations == 0
#   - All 7 honest nodes agree on committed blocks at every height

# TEST 9: test_bft_liveness_stalls_under_even_partition
# Setup: n=10, no Byzantine nodes, round_timeout=0.5,
#   mean_latency=0.02, sim_duration=200.0, seed=42
# Action:
#   - Run for 50 simulated seconds (establish baseline: some blocks committed)
#   - Apply partition: group1=["N0".."N4"], group2=["N5".."N9"] at t=50
#   - Run until t=150 (100 seconds of partition)
# Assert:
#   - blocks_committed_during_partition == 0
#     (measure committed blocks between t=50 and t=150)
#   - metrics.liveness_failures > 0

# TEST 10: test_bft_liveness_resumes_after_heal
# Continuation of TEST 9 setup and partition.
# Action: Heal partition at t=150. Run until t=250.
# Assert:
#   - At least 5 new blocks committed between t=150 and t=250

# TEST 11: test_bft_message_complexity_quadratic
# Purpose: Verify O(n²) message scaling.
# Run BFT simulation for n=[4, 10] with 10 rounds each, seed=42.
# Record messages_per_round for each n.
# Assert:
#   - msgs_per_round(n=10) / msgs_per_round(n=4) is in [4.0, 10.0]
#     (exact ratio is 6.25 for pure O(n²), but allow range for implementation variation)
```

### Step 3.2 — Implement

**`src/consensus/bft.py`**
Implement `TendermintBFT(ConsensusProtocol)`. The state machine has these phases: `PROPOSE → PREVOTE → PRECOMMIT → COMMIT`. One complete cycle is one *round*. A new *height* begins after a commit.

Key implementation guidance:
- Use `MessageType.PROPOSAL`, `MessageType.VOTE` (with `protocol_fields` distinguishing `PREVOTE` vs `PRECOMMIT`).
- The proposer in round r is `node_ids[r % n]`.
- A round advances to Prevote when the proposer's proposal is received (or timeout fires).
- Prevote quorum (≥ ceil(2n/3)) advances to Precommit.
- Precommit quorum commits the block.
- Timeout fires after `round_timeout` simulated seconds with no quorum. The node sends a TIMEOUT message and increments the round.
- Equivocation detection: if a node receives two different votes from the same sender in the same round/phase, log a warning and discard the second vote.
- Track `committed_at` timestamps on each block for finality latency metrics.

**`config/bft_default.yaml`**
```yaml
bft:
  n_nodes: 10
  f_byzantine: 0
  round_timeout: 0.5
  txs_per_block: 20
  mean_latency: 0.050
  latency_std: 0.010
  sim_duration: 1000.0
  seed: 42
```

### Acceptance Criteria — Phase 3

- [ ] `pytest tests/unit/test_bft.py -v` passes with **0 failures**
- [ ] `pytest tests/integration/test_bft_sim.py -v` passes with **0 failures**
- [ ] `pytest tests/unit/ tests/integration/test_pow_sim.py -v` passes with **0 regressions**
- [ ] TEST 8 safety assertion (`safety_violations == 0`) passes with `f=3` Byzantine nodes
- [ ] TEST 9 verifies `blocks_committed_during_partition == 0` (logged at INFO level)

### Phase 3 Handoff

```
src/consensus/bft.py
tests/unit/test_bft.py           (6 unit tests, all green)
tests/integration/test_bft_sim.py (5 integration tests, all green)
config/bft_default.yaml
```
All prior 22 tests remain green.

---

## 9. Phase 4 — Proof of Transfer (PoX)

---

> **📋 AGENT CONTEXT — Phase 4**
>
> **Prior phases complete:** Phases 1, 2, 3. All 33 tests green.
>
> **Do not modify:** Anything in `src/core/`, `src/consensus/base.py`, `src/consensus/pow.py`, `src/consensus/bft.py`.
>
> **You are implementing:** Proof of Transfer (PoX) — the consensus mechanism used by the Stacks blockchain. This is a two-layer system: a *base chain* (modeled as a simplified PoW chain) and a *PoX chain* (the Stacks-like layer) anchored to it.
>
> **Conceptual model:** In each base-chain block, PoX miners *bid* by committing base tokens (BTC analogues). The winning miner is selected proportionally to their bid. The winner earns the right to produce the corresponding PoX block. STX stackers (who have locked tokens) receive the winning bid as yield.

---

### Goal

Implement a two-layer PoX simulation. The base chain produces blocks at a regular interval. PoX miners bid per base block slot. A winner is selected, mines the PoX block, and stackers receive rewards. Finality of PoX blocks requires k base-chain confirmations.

### Deliverables

- `src/consensus/pox.py`
- `tests/unit/test_pox.py`
- `tests/integration/test_pox_sim.py`
- `config/pox_default.yaml`

---

### Step 4.1 — Write Tests First

**`tests/unit/test_pox.py`**

```python
# TEST 1: test_pox_miner_selection_proportional
# Purpose: Verify winner selection is proportional to bids.
# Setup: 5 miners with bids = [10.0, 20.0, 30.0, 40.0, 50.0] (total=150.0)
#   seed=42, n_elections=2000
# Action: Run 2000 winner selections using PoX._select_winner(bids, seed)
# Assert (proportional within ±4%):
#   - miner_0 wins in [6%, 14%]   (expected 10/150 ≈ 6.67%)
#   - miner_4 wins in [29%, 37%]  (expected 50/150 ≈ 33.33%)

# TEST 2: test_stacker_reward_calculation
# Setup: 3 stackers with locked_stx = [100, 200, 300], winning_bid = 60.0
# Action: rewards = compute_stacker_rewards(locked_stx=[100,200,300], winning_bid=60.0)
# Assert:
#   - rewards[0] == 10.0   (60 × 100/600)
#   - rewards[1] == 20.0   (60 × 200/600)
#   - rewards[2] == 30.0   (60 × 300/600)
#   - sum(rewards) == 60.0 (exactly, no floating-point leakage)

# TEST 3: test_pox_block_anchored_to_base_block
# Setup: Create a PoXBlock anchored to base_block with hash "abc"
# Assert:
#   - pox_block.protocol_fields["anchor_hash"] == "abc"
#   - pox_block.protocol_fields["base_height"] matches base_block.height

# TEST 4: test_pox_finality_requires_k_confirmations
# Setup: PoX config with k_confirmations=6
#   Create PoXBlock mined at base_height=100
# Assert:
#   - is_final(pox_block, current_base_height=105) == False
#   - is_final(pox_block, current_base_height=106) == True
#   - is_final(pox_block, current_base_height=200) == True

# TEST 5: test_zero_bid_excluded_from_selection
# Setup: 3 miners with bids = [0.0, 10.0, 20.0]
# Action: Run 100 winner selections
# Assert: miner_0 (bid=0.0) is never selected
```

**`tests/integration/test_pox_sim.py`**

```python
# TEST 6: test_pox_produces_anchored_blocks
# Setup: 3 PoX miners, 2 stackers, base_block_time=10.0,
#   k_confirmations=3, mean_latency=0.05, sim_duration=500.0, seed=42
# Assert:
#   - At least 40 PoX blocks produced (500s / 10s = 50 expected; 40 is conservative)
#   - Every PoX block's anchor_hash matches a real base block hash

# TEST 7: test_stacker_total_rewards_equal_total_bids
# Same setup as TEST 6.
# After simulation:
#   total_bids_paid = sum of all winning bids
#   total_rewards_received = sum of all stacker rewards across all blocks
# Assert: abs(total_bids_paid - total_rewards_received) < 0.001  (float precision)

# TEST 8: test_pox_throughput_bounded_by_base_block_time
# Setup: 5 PoX miners, base_block_time=10.0, txs_per_pox_block=20,
#   sim_duration=1000.0, seed=42
# theoretical_max_tps = txs_per_pox_block / base_block_time = 2.0
# Assert:
#   - metrics.throughput_tps <= theoretical_max_tps * 1.05

# TEST 9: test_pox_finality_latency_proportional_to_k
# Run two simulations:
#   Sim A: k_confirmations=3, base_block_time=10.0, sim_duration=500.0
#   Sim B: k_confirmations=6, base_block_time=10.0, sim_duration=500.0
# Assert:
#   - mean_finality_latency(B) is approximately 2x mean_finality_latency(A)
#     (within 20% tolerance)
```

### Step 4.2 — Implement

**`src/consensus/pox.py`**
Implement two classes:

`BaseChain`: A simplified PoW-like chain that produces blocks at `base_block_time` intervals (deterministic, no difficulty). It is not a full PoW simulation — it is a reliable clock for the PoX layer. On each new base block, it emits a `BLOCK_ANNOUNCE` that PoX miners listen to.

`ProofOfTransfer(ConsensusProtocol)`: Each node has a `bid_amount: float` and a `stacked_stx: float`. On receiving a base block announcement:
1. All miners submit their bids.
2. One winner is selected proportionally using `numpy.random.Generator.choice` with weights.
3. The winner mines the PoX block, anchoring it to the base block's hash.
4. Stacker rewards are computed and recorded in metrics.
5. PoX block finality is gated on base chain depth: a PoX block becomes final when the base chain is k blocks deeper than the anchor height.

**`config/pox_default.yaml`**
```yaml
pox:
  n_miners: 5
  n_stackers: 3
  base_block_time: 10.0
  k_confirmations: 6
  txs_per_pox_block: 20
  mean_latency: 0.050
  latency_std: 0.010
  sim_duration: 2000.0
  seed: 42
```

### Acceptance Criteria — Phase 4

- [ ] `pytest tests/unit/test_pox.py -v` passes with **0 failures**
- [ ] `pytest tests/integration/test_pox_sim.py -v` passes with **0 failures**
- [ ] TEST 7's reward accounting assertion is exact to 3 decimal places
- [ ] `pytest tests/unit/ -v` passes with **0 regressions** (all 19 unit tests green)

### Phase 4 Handoff

```
src/consensus/pox.py
tests/unit/test_pox.py             (5 unit tests, all green)
tests/integration/test_pox_sim.py  (4 integration tests, all green)
config/pox_default.yaml
```
All prior 38 tests remain green.

---

## 10. Phase 5 — Adversarial Scenarios

---

> **📋 AGENT CONTEXT — Phase 5**
>
> **Prior phases complete:** Phases 1–4. All 47 tests green.
>
> **Do not modify:** Anything in `src/core/`, `src/consensus/`.
>
> **You are implementing:** The adversarial scenario harness. This requires new node subclasses and network event controllers that *wrap* existing consensus implementations without changing them.
>
> **Design constraint:** Byzantine behavior must be injectable via configuration (`NodeRole.BYZANTINE` + a `byzantine_strategy` parameter), not hardcoded. Honest nodes must not change behavior when Byzantine nodes are present — only the Byzantine nodes behave differently.

---

### Goal

Implement three adversarial scenarios and verify that each consensus mechanism behaves according to its theoretical predictions under stress.

### Deliverables

- `src/adversarial/byzantine.py`
- `src/adversarial/partitioner.py`
- `src/adversarial/selfish_miner.py`
- `tests/adversarial/test_partition.py`
- `tests/adversarial/test_byzantine.py`
- `tests/adversarial/test_selfish.py`

---

### Step 5.1 — Write Tests First

**`tests/adversarial/test_partition.py`**

```python
# TEST 1: test_pow_liveness_preserved_under_partition
# Setup: PoW, n=20 nodes (10 per partition half), seed=42,
#   mean_latency=0.1, target_block_time=10.0, sim_duration=2000.0
# Action:
#   - Run until t=500 (baseline: record canonical_height of each node)
#   - Apply partition: group1=N0..N9, group2=N10..N19 at t=500
#   - Run until t=1500 (1000s of partition)
# Assert:
#   - At t=1500, group1 nodes have mined additional blocks (height increased)
#   - At t=1500, group2 nodes have mined additional blocks (height increased)
#   - (Both sides are live — liveness is preserved under PoW partition)

# TEST 2: test_pow_consistency_broken_under_partition
# Continuation of TEST 1.
# After partition (t=1500):
# Assert:
#   - group1's canonical tip hash != group2's canonical tip hash
#   - This is expected — partition breaks consistency in PoW

# TEST 3: test_bft_stalls_under_5050_partition
# Setup: BFT, n=10, no Byzantine, round_timeout=0.5, mean_latency=0.02
# Action:
#   - Record blocks committed at t<50
#   - Apply 50/50 partition at t=50
#   - Record blocks committed between t=50 and t=250
# Assert:
#   - blocks_in_partition_window == 0
#   - No safety violation (no two honest nodes committed conflicting blocks)

# TEST 4: test_bft_resumes_after_heal
# Continuation of TEST 3.
# Action: Heal partition at t=250. Run until t=400.
# Assert:
#   - At least 5 new blocks committed between t=250 and t=400
#   - All nodes agree on block hashes at all heights
```

**`tests/adversarial/test_byzantine.py`**

```python
# TEST 5: test_bft_safe_with_max_byzantine
# Setup: BFT, n=10, f=3 equivocating Byzantine nodes, round_timeout=0.5,
#   mean_latency=0.05, sim_duration=500.0, seed=42
# Byzantine strategy: send PREVOTE for block B to nodes N0..N4,
#   and PREVOTE for block B' (different) to nodes N5..N9
# Assert:
#   - metrics.safety_violations == 0
#   - Honest nodes committed at least 10 blocks

# TEST 6: test_bft_unsafe_beyond_threshold
# Setup: BFT, n=10, f=4 equivocating Byzantine nodes (exceeds n/3=3.33)
#   round_timeout=0.5, mean_latency=0.05, sim_duration=500.0, seed=42
# Assert:
#   - safety_violations > 0 OR liveness stalls completely (< 2 blocks committed)
#   - (Either outcome is acceptable — the protocol is expected to fail)
#   - Log the observed failure mode at WARNING level

# TEST 7: test_equivocating_votes_logged
# Setup: BFT node N0 receiving votes in a single round.
# Action: Inject two conflicting Prevotes from node N1 (same round, different blocks)
# Assert:
#   - Python logging captured a WARNING containing both "equivocation" and "N1"
```

**`tests/adversarial/test_selfish.py`**

```python
# TEST 8: test_selfish_miner_earns_disproportionate_share
# Purpose: Demonstrate selfish mining gives revenue beyond honest hash share.
# Setup:
#   - 9 honest PoW nodes with equal hash power (hash_fraction = 0.067 each, total=0.6)
#   - 1 selfish miner with hash_fraction=0.4 (40% of total)
#   - mean_latency=0.1, target_block_time=10.0, sim_duration=5000.0, seed=42
# Selfish miner strategy:
#   - When selfish miner mines a block, do NOT announce it immediately.
#   - Keep it private. Continue mining on the secret chain.
#   - When the honest network mines a competing block (catching up),
#     release the withheld block(s) immediately to win the fork.
# After simulation:
#   - Count blocks in canonical chain attributed to selfish miner vs honest nodes
#   - selfish_share = selfish_blocks_in_canonical / total_canonical_blocks
# Assert:
#   - selfish_share > 0.40  (earns more than honest hash fraction)
#   (Expected: ~50-60% for 40% hash power under selfish mining)

# TEST 9: test_honest_miner_earns_fair_share
# Same setup as TEST 8 but WITHOUT selfish behavior (all honest).
# Assert:
#   - Each honest miner's canonical block share is in [0.04, 0.10]
#     (expected ≈ 0.067 for equal hash power; allow ±3% for variance)
```

### Step 5.2 — Implement

**`src/adversarial/partitioner.py`**
Implement `NetworkPartitioner(env, network)`. Expose `schedule_partition(group1, group2, start_time)` and `schedule_heal(heal_time)` as SimPy-scheduled events. These call `network.apply_partition()` and `network.heal_partition()` at the scheduled simulation times.

**`src/adversarial/byzantine.py`**
Implement `EquivocatingNode(Node)`. Override message dispatch: when sending a vote, send vote-for-B to the first half of peers and vote-for-B' to the second half. The `B'` block is constructed with a different `payload_hash` (any deterministic modification of `B.payload_hash`).

**`src/adversarial/selfish_miner.py`**
Implement `SelfishMiner` as a variant of `ProofOfWork`. Maintain a `private_chain` list. On mining a block, add to `private_chain` instead of broadcasting. On receiving a competing honest block that ties or exceeds the private chain, broadcast the entire private chain immediately. Then resume honest behavior until the next private block.

### Acceptance Criteria — Phase 5

- [ ] `pytest tests/adversarial/ -v` passes with **0 failures**
- [ ] TEST 2 (PoW consistency broken) passes — the two partition halves have diverged chains
- [ ] TEST 8 selfish miner share is logged at INFO level (value must be visible in output)
- [ ] All 47 prior tests remain green (`pytest tests/unit/ tests/integration/ -v`)

### Phase 5 Handoff

```
src/adversarial/byzantine.py
src/adversarial/partitioner.py
src/adversarial/selfish_miner.py
tests/adversarial/  (9 tests, all green)
```
Total test count: 56.

---

## 11. Phase 6 — Experiment Runner & Analysis

---

> **📋 AGENT CONTEXT — Phase 6**
>
> **Prior phases complete:** Phases 1–5. All 56 tests green.
>
> **Do not modify:** Anything in `src/core/`, `src/consensus/`, `src/adversarial/`.
>
> **You are implementing:** The experiment orchestration and statistical analysis layer. This is the layer that produces the research outputs — the CSVs, plots, and summary tables that constitute the empirical claims in Section 2.
>
> **Reproducibility is non-negotiable.** Every experiment must produce identical output when run with the same seed. This will be verified by a test.

---

### Goal

Build an experiment runner that executes multi-trial simulations, collects `MetricsSnapshot` results per trial, computes 95% confidence intervals, and generates publication-quality comparison plots.

### Deliverables

- `src/experiments/runner.py`
- `src/experiments/scenarios.py`
- `src/analysis/stats.py`
- `src/analysis/visualizer.py`
- `scripts/run_experiment.py`
- `scripts/generate_report.py`
- `tests/unit/test_analysis.py`
- `tests/integration/test_runner.py`

---

### Step 6.1 — Write Tests First

**`tests/unit/test_analysis.py`**

```python
# TEST 1: test_confidence_interval_correct
# Input: measurements = [1.0, 2.0, 3.0, 4.0, 5.0]  (n=5, df=4)
# Expected: mean=3.0, 95% CI uses t-distribution with df=4
#   t_critical(df=4, alpha=0.025) ≈ 2.776
#   SE = std([1,2,3,4,5]) / sqrt(5) = 1.581 / 2.236 ≈ 0.707
#   CI = [3.0 - 2.776*0.707, 3.0 + 2.776*0.707] ≈ [1.04, 4.96]
# Action: result = compute_ci(measurements, confidence=0.95)
# Assert:
#   - abs(result.mean - 3.0) < 0.001
#   - abs(result.ci_lower - 1.04) < 0.10
#   - abs(result.ci_upper - 4.96) < 0.10
#   - result.n == 5

# TEST 2: test_summarize_returns_all_fields
# Input: list of 5 MetricsSnapshot objects with varying throughput_tps values
# Action: summary = summarize(snapshots)
# Assert: summary dict has keys:
#   ['throughput_tps', 'mean_finality_latency_s', 'fork_rate',
#    'safety_violations', 'liveness_failures', 'message_count']
# And each value is a CIResult with fields [mean, ci_lower, ci_upper, n]

# TEST 3: test_ci_single_sample_returns_nan_interval
# Input: measurements = [5.0]  (n=1)
# Assert: result.ci_lower == float('nan') OR result.ci_lower == result.mean
#   (Undefined CI for n=1; do not raise an exception)
```

**`tests/integration/test_runner.py`**

```python
# TEST 4: test_experiment_deterministic
# Setup: ExperimentConfig(
#   protocol="pow", n_nodes=5, n_trials=3,
#   sim_duration=200.0, mean_latency=0.1, seed=42
# )
# Action: Run experiment twice with identical config.
# Assert: result_1.per_trial_metrics == result_2.per_trial_metrics
#   (All throughput_tps, fork_rate, etc. values are identical)

# TEST 5: test_experiment_outputs_csv
# Setup: Run pow experiment with n_trials=5, sim_duration=200.0, seed=42
# Assert:
#   - File "results/pow_n5_seed42.csv" exists
#   - CSV has 5 rows (one per trial)
#   - CSV has columns: trial, throughput_tps, mean_finality_latency_s,
#       fork_rate, safety_violations, liveness_failures, message_count

# TEST 6: test_comparison_plots_generated
# Setup: Run all 3 protocols with n_trials=3, sim_duration=300.0, seed=42
# Action: generate_comparison_plots(results_dir="results/")
# Assert:
#   - File "results/plots/throughput_comparison.png" exists, size > 5KB
#   - File "results/plots/latency_comparison.png" exists, size > 5KB
#   - File "results/plots/fork_rate_vs_latency.png" exists, size > 5KB

# TEST 7: test_latency_sweep_produces_monotone_fork_rates
# Purpose: Full pipeline test producing research claim C1.
# Setup: PoW, n_nodes=10, n_trials=5, target_block_time=10.0, seed=42
# Sweep: mean_latency in [0.01, 0.05, 0.10, 0.50, 1.00]
# For each latency, run experiment, record mean fork_rate.
# Assert:
#   - fork_rates are strictly monotonically increasing across latencies
#   - Pearson correlation(latencies, fork_rates) > 0.95
```

### Step 6.2 — Implement

**`src/analysis/stats.py`**
Implement `CIResult(mean, ci_lower, ci_upper, n)` dataclass and `compute_ci(measurements: List[float], confidence: float = 0.95) -> CIResult` using `scipy.stats.t.interval`. Implement `summarize(snapshots: List[MetricsSnapshot]) -> Dict[str, CIResult]`.

**`src/analysis/visualizer.py`**
Implement `generate_comparison_plots(summary_by_protocol: Dict[str, Dict], output_dir: str)`. Produce at minimum:
- `throughput_comparison.png`: Bar chart with error bars (95% CI) showing mean throughput per protocol.
- `latency_comparison.png`: Bar chart with error bars for mean finality latency per protocol.
- `fork_rate_vs_latency.png`: Line plot of fork rate vs. network latency (PoW only, from latency sweep). Use `seaborn` for styling. Save at 150 DPI minimum.

**`src/experiments/runner.py`**
Implement `ExperimentConfig` dataclass and `ExperimentRunner`. The runner:
1. Creates a fresh SimPy environment for each trial.
2. Seeds the environment with `config.seed + trial_index` (ensures per-trial variation while keeping the overall experiment reproducible).
3. Runs the simulation and collects a `MetricsSnapshot`.
4. After all trials, writes a CSV to `results/<protocol>_n<nodes>_seed<seed>.csv`.
5. Returns an `ExperimentResult(config, per_trial_metrics, summary)`.

**`src/experiments/scenarios.py`**
Implement factory functions:
- `latency_sweep_pow(latencies: List[float], **kwargs) -> List[ExperimentConfig]`
- `byzantine_sweep_bft(f_values: List[int], **kwargs) -> List[ExperimentConfig]`
- `pox_k_sweep(k_values: List[int], **kwargs) -> List[ExperimentConfig]`

**`scripts/run_experiment.py`** (CLI entry point using Click):
```
Usage: python scripts/run_experiment.py --protocol [pow|bft|pox] --config config/pow_default.yaml --trials 10
```

### Acceptance Criteria — Phase 6

- [ ] `pytest tests/unit/test_analysis.py tests/integration/test_runner.py -v` passes with **0 failures**
- [ ] TEST 4 reproducibility test passes (two runs with seed=42 are identical)
- [ ] TEST 7 Pearson correlation > 0.95 is verified and logged at INFO level
- [ ] Running `python scripts/run_experiment.py --protocol pow --trials 5` completes without error and produces a CSV in `results/`
- [ ] All 56 prior tests remain green: `pytest tests/ -v --ignore=tests/adversarial/` (adversarial tests may be slow; run separately)

### Phase 6 Handoff

```
src/experiments/runner.py
src/experiments/scenarios.py
src/analysis/stats.py
src/analysis/visualizer.py
scripts/run_experiment.py
scripts/generate_report.py
tests/unit/test_analysis.py     (3 tests)
tests/integration/test_runner.py (4 tests)
results/                         (CSVs and plots present)
```
**Total test count: 63 tests, all green.**

---

## 12. Running the Full Suite

After all phases are complete, the following commands must all succeed:

```bash
# Install dependencies
pip install -r requirements.txt

# Run all tests with coverage
pytest tests/ -v --cov=src --cov-report=term-missing

# Run the default PoW experiment (10 trials)
python scripts/run_experiment.py --protocol pow --config config/pow_default.yaml --trials 10

# Run the BFT experiment
python scripts/run_experiment.py --protocol bft --config config/bft_default.yaml --trials 10

# Run the PoX experiment
python scripts/run_experiment.py --protocol pox --config config/pox_default.yaml --trials 10

# Generate all comparison plots and summary report
python scripts/generate_report.py --results-dir results/ --output-dir results/plots/

# Run the latency sweep (produces research claim C1 evidence)
python scripts/run_experiment.py --protocol pow --sweep latency --trials 5
```

---

## 13. Expected Research Outputs

When the full suite runs successfully, the `results/` directory must contain:

### 13.1 CSV Files

| File | Content |
|---|---|
| `results/pow_latency_sweep.csv` | fork_rate, throughput_tps at 7 latency values |
| `results/bft_byzantine_sweep.csv` | safety_violations, blocks_committed at f=[0,1,2,3,4] |
| `results/pox_k_sweep.csv` | mean_finality_latency at k=[1,3,6,12] |
| `results/protocol_comparison.csv` | Side-by-side metrics for all 3 protocols |

### 13.2 Plots

| File | Research Claim It Supports |
|---|---|
| `plots/fork_rate_vs_latency.png` | C1: PoW fork rate ∝ latency/block_time |
| `plots/throughput_comparison.png` | C2: BFT throughput is higher but liveness-fragile |
| `plots/latency_comparison.png` | C3: PoX finality latency = k × base_block_time |
| `plots/bft_safety_under_byzantine.png` | C2: BFT safety holds until f ≥ n/3 |
| `plots/partition_behavior.png` | C2 vs C1: BFT stalls; PoW continues |

### 13.3 Summary Table (logged to stdout by `generate_report.py`)

```
Protocol  | Throughput (TPS) | Finality (s)   | Fork Rate | Safety Violations
----------+------------------+----------------+-----------+------------------
PoW       | X.XX ± Y.YY      | XX.X ± YY.Y    | 0.XXX     | 0
BFT       | X.XX ± Y.YY      | X.XX ± Y.YY    | 0.000     | 0
PoX       | X.XX ± Y.YY      | XX.X ± YY.Y    | 0.000*    | 0
* PoX fork rate is 0 at the PoX layer; base chain fork rate measured separately
```

---