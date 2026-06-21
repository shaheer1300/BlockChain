# Consensus Measurement

This project is a **discrete-event simulation testbed** that implements three distinct blockchain consensus mechanisms from first principles and subjects each to a battery of controlled experiments.

**What it produces:**
- A Python simulation engine (built on SimPy) capable of modeling hundreds of nodes exchanging messages over a configurable network.
- Three pluggable consensus protocol implementations: Proof of Work (PoW), Tendermint BFT, and Proof of Transfer (PoX).
- An adversarial scenario harness: network partitions, Byzantine nodes, selfish miners, variable message delay.
- A statistical experiment runner that executes multi-trial experiments, computes confidence intervals, and generates publication-quality plots.

**What it is not:**
- Not a real blockchain implementation. Cryptographic operations are stubbed with deterministic hashes. The goal is behavioral fidelity, not production security.
- Not a distributed system. All nodes run in the same Python process via SimPy coroutines. Network effects are modeled probabilistically.

**The three research claims this testbed must produce empirical evidence for:**

| Claim | Expected Observation |
|---|---|
| C1: Fork rate in PoW scales with `network_latency / block_time` | Fork rate increases monotonically as latency increases; Pearson r > 0.95 |
| C2: BFT (Tendermint) achieves deterministic finality but sacrifices liveness under partition | Zero committed blocks during a 50/50 partition; instant finality otherwise |
| C3: PoX inherits PoW's liveness properties and anchoring latency | PoX finality = k × base_block_time; throughput bounded by base chain's block rate |

Every phase in this project exists to produce the infrastructure needed to test these three claims.

*Core objective.* Build a simulation harness that implements and empirically compares consensus mechanisms — at minimum PoW versus a BFT-style protocol, ideally including a model of Proof of Transfer — measuring throughput, finality latency, fork rate, and behavior under adversarial conditions (network partition, varying message delay, a fraction of Byzantine nodes).

*Target stack & concepts.* A discrete-event simulation framework (Python with SimPy, or Go); statistical analysis and clean plots of the results. Concepts demonstrated: consensus theory, cryptoeconomic incentive analysis, the safety/liveness trade-off, and quantitative systems evaluation.

*System design challenge.* This is the project that speaks directly to Uzmi's research style — he publishes simulation studies and measurement-driven work. The challenge is methodological: defining fair metrics, isolating variables, and producing defensible empirical claims about why one mechanism behaves differently under stress. This is where you convert "I built a thing" into "I can do research."