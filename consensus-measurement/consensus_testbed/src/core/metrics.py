from dataclasses import dataclass
from typing import Hashable


@dataclass(frozen=True)
class MetricsSnapshot:
    throughput_tps: float
    mean_finality_latency_s: float
    fork_rate: float
    safety_violations: int
    liveness_failures: int
    message_count: int


class MetricsCollector:
    def __init__(self, sim_duration: float) -> None:
        self.sim_duration = sim_duration
        self._submitted_txs: dict[Hashable, float] = {}
        self._confirmed_txs: dict[Hashable, float] = {}
        self._mined_blocks: list[Hashable] = []
        self._orphaned_blocks: list[Hashable] = []
        self.safety_violations = 0
        self.liveness_failures = 0
        self.message_count = 0

    def record_tx_submitted(self, tx_id: Hashable, submitted_at: float) -> None:
        self._submitted_txs[tx_id] = submitted_at

    def record_tx_confirmed(self, tx_id: Hashable, confirmed_at: float) -> None:
        self._confirmed_txs[tx_id] = confirmed_at

    def record_block_mined(self, block_id: Hashable) -> None:
        self._mined_blocks.append(block_id)

    def record_block_orphaned(self, block_id: Hashable) -> None:
        self._orphaned_blocks.append(block_id)

    def record_safety_violation(self) -> None:
        self.safety_violations += 1

    def record_liveness_failure(self) -> None:
        self.liveness_failures += 1

    def record_message_sent(self) -> None:
        self.message_count += 1

    def snapshot(self) -> MetricsSnapshot:
        confirmed_count = len(self._confirmed_txs)
        throughput = confirmed_count / self.sim_duration if self.sim_duration > 0 else 0.0

        finality_latencies = [
            confirmed_at - self._submitted_txs[tx_id]
            for tx_id, confirmed_at in self._confirmed_txs.items()
            if tx_id in self._submitted_txs
        ]
        mean_finality = (
            sum(finality_latencies) / len(finality_latencies)
            if finality_latencies
            else 0.0
        )

        mined_count = len(self._mined_blocks)
        fork_rate = len(self._orphaned_blocks) / mined_count if mined_count else 0.0

        return MetricsSnapshot(
            throughput_tps=throughput,
            mean_finality_latency_s=mean_finality,
            fork_rate=fork_rate,
            safety_violations=self.safety_violations,
            liveness_failures=self.liveness_failures,
            message_count=self.message_count,
        )
