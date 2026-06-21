import logging

import numpy as np

from src.core.message import Message


logger = logging.getLogger(__name__)


class Network:
    def __init__(
        self,
        env,
        mean_latency: float,
        latency_std: float,
        seed: int = 42,
    ) -> None:
        self.env = env
        self.mean_latency = mean_latency
        self.latency_std = latency_std
        self.rng = np.random.default_rng(seed)
        self.nodes = {}
        self._partition: tuple[set[str], set[str]] | None = None

    def register_node(self, node) -> None:
        self.nodes[node.id] = node

    def send(self, sender_id: str, receiver_id: str, message: Message) -> None:
        if self._is_partitioned(sender_id, receiver_id):
            logger.debug("Dropping cross-partition message from %s to %s", sender_id, receiver_id)
            return
        latency = self._sample_latency()
        self.env.process(self._deliver(message, latency))

    def apply_partition(self, group1: list[str], group2: list[str]) -> None:
        self._partition = (set(group1), set(group2))

    def heal_partition(self) -> None:
        self._partition = None

    def _sample_latency(self) -> float:
        sample = self.rng.normal(self.mean_latency, self.latency_std)
        return max(0.0, float(sample))

    def _deliver(self, message: Message, latency: float):
        yield self.env.timeout(latency)
        node = self.nodes.get(message.receiver)
        if node is not None:
            node.inbox.append(message)

    def _is_partitioned(self, sender_id: str, receiver_id: str) -> bool:
        if self._partition is None:
            return False
        group1, group2 = self._partition
        return (
            sender_id in group1
            and receiver_id in group2
            or sender_id in group2
            and receiver_id in group1
        )
