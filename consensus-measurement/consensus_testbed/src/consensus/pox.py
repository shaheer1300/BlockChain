from decimal import Decimal
import hashlib
import logging
from types import MethodType

import numpy as np

from src.consensus.base import ConsensusProtocol
from src.core.block import Block, genesis_block
from src.core.message import Message, MessageType


logger = logging.getLogger(__name__)


def compute_stacker_rewards(locked_stx: list[float], winning_bid: float) -> list[float]:
    total_locked = sum(Decimal(str(amount)) for amount in locked_stx)
    bid = Decimal(str(winning_bid))
    if total_locked == 0:
        return [0.0 for _amount in locked_stx]
    rewards = [
        bid * Decimal(str(amount)) / total_locked
        for amount in locked_stx
    ]
    if rewards:
        rewards[-1] = bid - sum(rewards[:-1], Decimal("0"))
    return [float(reward) for reward in rewards]


def create_pox_block(
    height: int,
    prev_hash: str,
    base_block: Block,
    miner_id: str,
    tx_count: int,
    timestamp: float,
) -> Block:
    payload = f"{height}|{prev_hash}|{base_block.hash}|{miner_id}|{tx_count}"
    return Block(
        height=height,
        prev_hash=prev_hash,
        payload_hash=hashlib.sha256(payload.encode()).hexdigest(),
        timestamp=timestamp,
        protocol_fields={
            "anchor_hash": base_block.hash,
            "base_height": base_block.height,
            "miner": miner_id,
            "tx_count": tx_count,
        },
    )


def is_final(pox_block: Block, current_base_height: int, k_confirmations: int) -> bool:
    return current_base_height >= pox_block.protocol_fields["base_height"] + k_confirmations


class BaseChain:
    def __init__(
        self,
        base_block_time: float,
        subscriber_ids: list[str],
        seed: int = 42,
    ) -> None:
        self.base_block_time = base_block_time
        self.subscriber_ids = subscriber_ids
        self.seed = seed
        self.blocks = [genesis_block()]

    def run(self, env, network, metrics):
        while True:
            yield env.timeout(self.base_block_time)
            block = self._next_block(timestamp=float(env.now))
            self.blocks.append(block)
            logger.info("Base block produced height=%s hash=%s", block.height, block.hash)
            for subscriber_id in self.subscriber_ids:
                metrics.record_message_sent()
                network.send(
                    "base-chain",
                    subscriber_id,
                    Message(
                        type=MessageType.BLOCK_ANNOUNCE,
                        sender="base-chain",
                        receiver=subscriber_id,
                        payload=block,
                        sent_at=float(env.now),
                    ),
                )

    def _next_block(self, timestamp: float) -> Block:
        height = len(self.blocks)
        prev_hash = self.blocks[-1].hash
        payload = f"base|{height}|{prev_hash}|{self.seed}"
        return Block(
            height=height,
            prev_hash=prev_hash,
            payload_hash=hashlib.sha256(payload.encode()).hexdigest(),
            timestamp=timestamp,
            protocol_fields={"base_block_time": self.base_block_time},
        )


class ProofOfTransfer(ConsensusProtocol):
    def __init__(
        self,
        node_id: str,
        miner_ids: list[str],
        bid_amount: float,
        all_bids: list[float],
        stacker_locked_stx: list[float],
        base_block_time: float,
        k_confirmations: int,
        txs_per_pox_block: int,
        seed: int = 42,
    ) -> None:
        self.node_id = node_id
        self.miner_ids = miner_ids
        self.bid_amount = bid_amount
        self.all_bids = all_bids
        self.stacker_locked_stx = stacker_locked_stx
        self.base_block_time = base_block_time
        self.k_confirmations = k_confirmations
        self.txs_per_pox_block = txs_per_pox_block
        self.seed = seed
        self.pox_chain: list[Block] = []
        self.base_height = 0
        self._node = None
        self._network = None
        self._metrics = None
        self._env = None

    def run(self, env, node, network, metrics):
        self._env = env
        self._node = node
        self._network = network
        self._metrics = metrics
        self._ensure_pox_metrics(metrics)
        while True:
            while node.inbox:
                self.on_message(node.inbox.pop(0))
            yield env.timeout(0.01)

    def on_message(self, msg) -> None:
        if msg.type != MessageType.BLOCK_ANNOUNCE or not isinstance(msg.payload, Block):
            return
        base_block = msg.payload
        if base_block.height <= self.base_height:
            return
        self.base_height = base_block.height
        self._process_base_block(base_block)

    def get_canonical_chain(self) -> list[Block]:
        return list(self.pox_chain)

    @staticmethod
    def _select_winner(bids: list[float], seed: int) -> int:
        positive_indices = [index for index, bid in enumerate(bids) if bid > 0]
        if not positive_indices:
            raise ValueError("at least one positive bid is required")
        positive_bids = np.array([bids[index] for index in positive_indices], dtype=float)
        probabilities = positive_bids / positive_bids.sum()
        rng = np.random.default_rng(seed)
        selected = rng.choice(len(positive_indices), p=probabilities)
        return positive_indices[int(selected)]

    def _process_base_block(self, base_block: Block) -> None:
        winner_index = self._select_winner(self.all_bids, seed=self.seed + base_block.height)
        winner_id = self.miner_ids[winner_index]
        previous_hash = self.pox_chain[-1].hash if self.pox_chain else "0" * 64
        pox_block = create_pox_block(
            height=len(self.pox_chain) + 1,
            prev_hash=previous_hash,
            base_block=base_block,
            miner_id=winner_id,
            tx_count=self.txs_per_pox_block,
            timestamp=self._now(),
        )
        self.pox_chain.append(pox_block)

        if self.node_id == winner_id and self._metrics is not None:
            winning_bid = self.all_bids[winner_index]
            rewards = compute_stacker_rewards(self.stacker_locked_stx, winning_bid)
            self._record_bid_and_rewards(winning_bid, rewards)
            self._record_finality(pox_block)
            logger.info(
                "PoX block mined node=%s height=%s base_height=%s bid=%.3f",
                self.node_id,
                pox_block.height,
                base_block.height,
                winning_bid,
            )

    def _record_bid_and_rewards(self, winning_bid: float, rewards: list[float]) -> None:
        bids_paid = getattr(self._metrics, "_pox_bids_paid", [])
        stacker_rewards = getattr(self._metrics, "_pox_stacker_rewards", [])
        bids_paid.append(float(winning_bid))
        stacker_rewards.extend(float(reward) for reward in rewards)
        setattr(self._metrics, "_pox_bids_paid", bids_paid)
        setattr(self._metrics, "_pox_stacker_rewards", stacker_rewards)

    def _record_finality(self, pox_block: Block) -> None:
        final_at = pox_block.timestamp + (self.k_confirmations * self.base_block_time)
        for tx_index in range(self.txs_per_pox_block):
            tx_id = f"pox-{pox_block.hash}-{tx_index}"
            self._metrics.record_tx_submitted(tx_id, pox_block.timestamp)
            self._metrics.record_tx_confirmed(tx_id, final_at)

    def _now(self) -> float:
        return float(self._env.now) if self._env is not None else 0.0

    @staticmethod
    def _ensure_pox_metrics(metrics) -> None:
        if hasattr(metrics, "pox_total_bids_paid"):
            return

        def pox_total_bids_paid(self):
            return sum(getattr(self, "_pox_bids_paid", []))

        def pox_total_rewards_received(self):
            return sum(getattr(self, "_pox_stacker_rewards", []))

        metrics.pox_total_bids_paid = MethodType(pox_total_bids_paid, metrics)
        metrics.pox_total_rewards_received = MethodType(pox_total_rewards_received, metrics)
