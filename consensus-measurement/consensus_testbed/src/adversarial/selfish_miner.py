import logging

import numpy as np

from src.consensus.pow import ProofOfWork
from src.core.block import Block
from src.core.message import MessageType


logger = logging.getLogger(__name__)


class SelfishMiner(ProofOfWork):
    def __init__(self, *args, **kwargs) -> None:
        super().__init__(*args, **kwargs)
        self.private_chain: list[Block] = []
        self._released_hashes: set[str] = set()

    def _mining_loop(self, env, network, metrics):
        while True:
            yield env.timeout(self._sample_mining_time())
            block = self.mine_block(timestamp=float(env.now))
            self._record_unique_mined(metrics, block)
            self._accept_block(block)
            self._update_canonical_chain()
            self._confirm_transactions(metrics, block)
            self.private_chain.append(block)
            logger.info("Selfish miner withheld block node=%s height=%s hash=%s", self.node_id, block.height, block.hash)
            if len(self.private_chain) >= 3:
                self._release_private_chain()

    def on_message(self, msg) -> None:
        if msg.type != MessageType.BLOCK_ANNOUNCE:
            return
        block = msg.payload
        if not isinstance(block, Block):
            return
        accepted = self._accept_block(block)
        if accepted:
            self._update_canonical_chain()
        if self.private_chain and block.height >= self.private_chain[0].height:
            self._release_private_chain()

    def _release_private_chain(self) -> None:
        releasable = [block for block in self.private_chain if block.hash not in self._released_hashes]
        if not releasable:
            return
        for block in releasable:
            self._released_hashes.add(block.hash)
            self._broadcast_block(block)
        self.private_chain = []

    @staticmethod
    def simulate_revenue(
        hash_fractions: dict[str, float],
        selfish_miner_id: str | None,
        n_blocks: int,
        seed: int = 42,
    ) -> dict[str, float]:
        miner_ids = list(hash_fractions)
        weights = np.array([hash_fractions[miner_id] for miner_id in miner_ids], dtype=float)
        weights = weights / weights.sum()
        rng = np.random.default_rng(seed)
        revenue = {miner_id: 0 for miner_id in miner_ids}
        private_lead = 0

        for _ in range(n_blocks):
            winner = miner_ids[int(rng.choice(len(miner_ids), p=weights))]
            if selfish_miner_id is None:
                revenue[winner] += 1
                continue
            if winner == selfish_miner_id:
                private_lead += 1
                continue
            if private_lead > 0:
                revenue[selfish_miner_id] += private_lead
                private_lead = 0
            else:
                revenue[winner] += 1

        if selfish_miner_id is not None and private_lead > 0:
            revenue[selfish_miner_id] += private_lead

        total = sum(revenue.values())
        return {miner_id: count / total for miner_id, count in revenue.items()}
