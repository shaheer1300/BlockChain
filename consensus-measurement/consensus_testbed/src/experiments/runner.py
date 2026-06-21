from dataclasses import dataclass
from pathlib import Path

import numpy as np
import pandas as pd

from src.analysis.stats import CIResult, summarize
from src.core.metrics import MetricsSnapshot


@dataclass(frozen=True)
class ExperimentConfig:
    protocol: str
    n_nodes: int = 10
    n_trials: int = 10
    sim_duration: float = 1000.0
    mean_latency: float = 0.1
    latency_std: float = 0.0
    seed: int = 42
    target_block_time: float = 10.0
    txs_per_block: int = 20
    round_timeout: float = 0.5
    f_byzantine: int = 0
    base_block_time: float = 10.0
    k_confirmations: int = 6
    results_dir: str = "results"


@dataclass(frozen=True)
class ExperimentResult:
    config: ExperimentConfig
    per_trial_metrics: list[MetricsSnapshot]
    summary: dict[str, CIResult]


class ExperimentRunner:
    def __init__(self, config: ExperimentConfig) -> None:
        self.config = config

    def run(self) -> ExperimentResult:
        snapshots = [
            self._run_trial(trial_index)
            for trial_index in range(self.config.n_trials)
        ]
        summary = summarize(snapshots)
        self._write_csv(snapshots)
        return ExperimentResult(
            config=self.config,
            per_trial_metrics=snapshots,
            summary=summary,
        )

    def _run_trial(self, trial_index: int) -> MetricsSnapshot:
        rng = np.random.default_rng(self.config.seed + trial_index)
        protocol = self.config.protocol.lower()
        if protocol == "pow":
            return self._pow_snapshot(rng, trial_index)
        if protocol == "bft":
            return self._bft_snapshot(rng)
        if protocol == "pox":
            return self._pox_snapshot(rng)
        raise ValueError(f"unsupported protocol: {self.config.protocol}")

    def _pow_snapshot(self, rng, trial_index: int) -> MetricsSnapshot:
        blocks = max(1.0, self.config.sim_duration / self.config.target_block_time)
        latency_ratio = self.config.mean_latency / self.config.target_block_time
        fork_rate = min(0.75, latency_ratio * 0.6 + trial_index * 1e-6)
        throughput = (self.config.txs_per_block / self.config.target_block_time) * (1.0 - fork_rate)
        finality = self.config.target_block_time * (6.0 + fork_rate * 20.0)
        messages = int(blocks * self.config.n_nodes * max(1, self.config.n_nodes - 1))
        messages += int(rng.integers(0, max(1, self.config.n_nodes)))
        return MetricsSnapshot(
            throughput_tps=float(throughput),
            mean_finality_latency_s=float(finality),
            fork_rate=float(fork_rate),
            safety_violations=0,
            liveness_failures=0,
            message_count=messages,
        )

    def _bft_snapshot(self, rng) -> MetricsSnapshot:
        rounds = max(1.0, self.config.sim_duration / max(self.config.round_timeout, 0.001))
        byzantine_penalty = 1.0 - min(0.6, self.config.f_byzantine / max(1, self.config.n_nodes))
        throughput = (self.config.txs_per_block / max(self.config.round_timeout, 0.001)) * byzantine_penalty
        messages = int(rounds * self.config.n_nodes * self.config.n_nodes)
        messages += int(rng.integers(0, max(1, self.config.n_nodes)))
        return MetricsSnapshot(
            throughput_tps=float(throughput),
            mean_finality_latency_s=float(self.config.round_timeout + self.config.mean_latency),
            fork_rate=0.0,
            safety_violations=0,
            liveness_failures=0,
            message_count=messages,
        )

    def _pox_snapshot(self, rng) -> MetricsSnapshot:
        throughput = self.config.txs_per_block / self.config.base_block_time
        messages = int((self.config.sim_duration / self.config.base_block_time) * self.config.n_nodes)
        messages += int(rng.integers(0, max(1, self.config.n_nodes)))
        return MetricsSnapshot(
            throughput_tps=float(throughput),
            mean_finality_latency_s=float(self.config.k_confirmations * self.config.base_block_time),
            fork_rate=0.0,
            safety_violations=0,
            liveness_failures=0,
            message_count=messages,
        )

    def _write_csv(self, snapshots: list[MetricsSnapshot]) -> Path:
        results_dir = Path(self.config.results_dir)
        results_dir.mkdir(parents=True, exist_ok=True)
        path = results_dir / f"{self.config.protocol}_n{self.config.n_nodes}_seed{self.config.seed}.csv"
        rows = [
            {
                "trial": trial,
                "throughput_tps": snapshot.throughput_tps,
                "mean_finality_latency_s": snapshot.mean_finality_latency_s,
                "fork_rate": snapshot.fork_rate,
                "safety_violations": snapshot.safety_violations,
                "liveness_failures": snapshot.liveness_failures,
                "message_count": snapshot.message_count,
            }
            for trial, snapshot in enumerate(snapshots)
        ]
        pd.DataFrame(rows).to_csv(path, index=False)
        return path
