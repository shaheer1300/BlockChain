import hashlib
import logging

import numpy as np

from src.consensus.base import ConsensusProtocol
from src.core.block import Block, genesis_block
from src.core.mempool import Mempool
from src.core.message import Message, MessageType


logger = logging.getLogger(__name__)

PoWBlock = Block


def fork_choice(chains: list[list[Block]]) -> list[Block]:
    return max(chains, key=lambda chain: chain[-1].height)


class ProofOfWork(ConsensusProtocol):
    def __init__(
        self,
        node_id: str,
        peer_ids: list[str],
        hash_fraction: float,
        target_block_time: float,
        txs_per_block: int,
        seed: int = 42,
        mempool_size: int = 10_000,
        difficulty: int = 4,
    ) -> None:
        self.node_id = node_id
        self.peer_ids = peer_ids
        self.hash_fraction = hash_fraction
        self.target_block_time = target_block_time
        self.txs_per_block = txs_per_block
        self.difficulty = difficulty
        self.rng = np.random.default_rng(seed)
        self.mempool = Mempool(max_size=mempool_size, seed=seed)
        self.genesis = genesis_block()
        self.blocks_by_hash: dict[str, Block] = {self.genesis.hash: self.genesis}
        self.first_seen: dict[str, int] = {self.genesis.hash: 0}
        self.canonical_chain: list[Block] = [self.genesis]
        self._seen_counter = 1
        self._relayed_hashes: set[str] = set()
        self._metrics = None
        self._network = None
        self._node = None

    def run(self, env, node, network, metrics):
        self._node = node
        self._network = network
        self._metrics = metrics
        env.process(self._message_loop(env))
        env.process(self._mining_loop(env, network, metrics))
        while True:
            yield env.timeout(self.target_block_time)

    def on_message(self, msg) -> None:
        if msg.type != MessageType.BLOCK_ANNOUNCE:
            return
        block = msg.payload
        if not isinstance(block, Block):
            return
        if not self._accept_block(block):
            return
        self._update_canonical_chain()
        self._relay_block(block, excluded_peer=msg.sender)

    def get_canonical_chain(self) -> list[Block]:
        return list(self.canonical_chain)

    def mine_block(self, timestamp: float) -> Block:
        selected_txs = self.mempool.select(self.txs_per_block)
        tip = self.canonical_chain[-1]
        payload_hash = self._payload_hash(selected_txs)
        block = PoWBlock(
            height=tip.height + 1,
            prev_hash=tip.hash,
            payload_hash=payload_hash,
            timestamp=timestamp,
            protocol_fields={
                "difficulty": self.difficulty,
                "miner": self.node_id,
                "tx_count": len(selected_txs),
                "tx_ids": selected_txs,
            },
        )
        self.mempool.remove_confirmed(selected_txs)
        return block

    def _sample_mining_time(self) -> float:
        mean = self.target_block_time / self.hash_fraction
        return float(self.rng.exponential(mean))

    def _mining_loop(self, env, network, metrics):
        while True:
            yield env.timeout(self._sample_mining_time())
            block = self.mine_block(timestamp=float(env.now))
            self._record_unique_mined(metrics, block)
            self._accept_block(block)
            self._update_canonical_chain()
            self._confirm_transactions(metrics, block)
            logger.info("PoW block mined node=%s height=%s hash=%s", self.node_id, block.height, block.hash)
            self._broadcast_block(block)

    def _message_loop(self, env):
        while True:
            while self._node is not None and self._node.inbox:
                msg = self._node.inbox.pop(0)
                self.on_message(msg)
            yield env.timeout(0.01)

    def _accept_block(self, block: Block) -> bool:
        if block.hash in self.blocks_by_hash:
            return False
        if block.prev_hash not in self.blocks_by_hash:
            return False
        parent = self.blocks_by_hash[block.prev_hash]
        if block.height != parent.height + 1:
            return False
        if not self._is_hex_hash(block.hash):
            return False
        self.blocks_by_hash[block.hash] = block
        self.first_seen[block.hash] = self._seen_counter
        self._seen_counter += 1
        return True

    def _update_canonical_chain(self) -> None:
        old_hashes = {block.hash for block in self.canonical_chain}
        best_tip = max(
            self.blocks_by_hash.values(),
            key=lambda block: (block.height, -self.first_seen[block.hash]),
        )
        new_chain = self._chain_to_tip(best_tip)
        new_hashes = {block.hash for block in new_chain}
        for orphan_hash in old_hashes - new_hashes:
            if orphan_hash != self.genesis.hash:
                self._record_unique_orphan(orphan_hash)
        self.canonical_chain = new_chain

    def _chain_to_tip(self, tip: Block) -> list[Block]:
        chain = [tip]
        while chain[-1].prev_hash in self.blocks_by_hash:
            parent = self.blocks_by_hash[chain[-1].prev_hash]
            chain.append(parent)
            if parent.height == 0:
                break
        return list(reversed(chain))

    def _broadcast_block(self, block: Block) -> None:
        self._relayed_hashes.add(block.hash)
        for peer_id in self.peer_ids:
            self._send_block(peer_id, block)

    def _relay_block(self, block: Block, excluded_peer: str) -> None:
        if block.hash in self._relayed_hashes:
            return
        self._relayed_hashes.add(block.hash)
        for peer_id in self.peer_ids:
            if peer_id != excluded_peer:
                self._send_block(peer_id, block)

    def _send_block(self, peer_id: str, block: Block) -> None:
        if self._network is None or self._metrics is None:
            return
        self._metrics.record_message_sent()
        self._network.send(
            self.node_id,
            peer_id,
            Message(
                type=MessageType.BLOCK_ANNOUNCE,
                sender=self.node_id,
                receiver=peer_id,
                payload=block,
                sent_at=float(self._network.env.now),
            ),
        )

    def _record_unique_mined(self, metrics, block: Block) -> None:
        seen = getattr(metrics, "_pow_mined_hashes", set())
        if block.hash not in seen:
            metrics.record_block_mined(block.hash)
            seen.add(block.hash)
            setattr(metrics, "_pow_mined_hashes", seen)

    def _record_unique_orphan(self, block_hash: str) -> None:
        if self._metrics is None:
            return
        seen = getattr(self._metrics, "_pow_orphan_hashes", set())
        if block_hash not in seen:
            self._metrics.record_block_orphaned(block_hash)
            seen.add(block_hash)
            setattr(self._metrics, "_pow_orphan_hashes", seen)

    def _confirm_transactions(self, metrics, block: Block) -> None:
        for tx_id in block.protocol_fields.get("tx_ids", []):
            if block.height <= 0:
                continue
            metrics.record_tx_submitted(tx_id, 0.0)
            metrics.record_tx_confirmed(tx_id, block.timestamp)

    @staticmethod
    def _payload_hash(tx_ids: list[str]) -> str:
        payload = "|".join(tx_ids)
        return hashlib.sha256(payload.encode()).hexdigest()

    @staticmethod
    def _is_hex_hash(value: str) -> bool:
        if len(value) != 64:
            return False
        try:
            int(value, 16)
        except ValueError:
            return False
        return True
