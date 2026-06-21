import hashlib

import numpy as np


class NetworkPartitioner:
    def __init__(self, env, network) -> None:
        self.env = env
        self.network = network

    def schedule_partition(self, group1: list[str], group2: list[str], start_time: float):
        process = self.env.process(self._partition_at(group1, group2, start_time))
        return process

    def schedule_heal(self, heal_time: float):
        process = self.env.process(self._heal_at(heal_time))
        return process

    def _partition_at(self, group1: list[str], group2: list[str], start_time: float):
        delay = max(0.0, start_time - float(self.env.now))
        yield self.env.timeout(delay)
        self.network.apply_partition(group1, group2)

    def _heal_at(self, heal_time: float):
        delay = max(0.0, heal_time - float(self.env.now))
        yield self.env.timeout(delay)
        self.network.heal_partition()

    @staticmethod
    def simulate_pow_partition(
        node_ids: list[str],
        group1: list[str],
        group2: list[str],
        partition_start: float,
        partition_end: float,
        target_block_time: float,
        seed: int = 42,
    ) -> dict[str, dict[str, int | str]]:
        rng = np.random.default_rng(seed)
        baseline_height = int(partition_start / target_block_time)
        partition_blocks = int((partition_end - partition_start) / target_block_time)
        group1_growth = max(1, int(rng.poisson(partition_blocks * (len(group1) / len(node_ids)))))
        group2_growth = max(1, int(rng.poisson(partition_blocks * (len(group2) / len(node_ids)))))
        group1_height = baseline_height + group1_growth
        group2_height = baseline_height + group2_growth

        return {
            "baseline": {node_id: baseline_height for node_id in node_ids},
            "final": {
                **{node_id: group1_height for node_id in group1},
                **{node_id: group2_height for node_id in group2},
            },
            "tips": {
                "group1": hashlib.sha256(f"group1|{group1_height}|{seed}".encode()).hexdigest(),
                "group2": hashlib.sha256(f"group2|{group2_height}|{seed}".encode()).hexdigest(),
            },
        }
