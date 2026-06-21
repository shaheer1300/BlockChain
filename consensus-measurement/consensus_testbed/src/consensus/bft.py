from collections import defaultdict
from dataclasses import dataclass
import hashlib
import logging
import math
from types import MethodType

import numpy as np

from src.consensus.base import ConsensusProtocol
from src.core.block import Block, genesis_block
from src.core.message import Message, MessageType


logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class BFTProposal:
    height: int
    round: int
    block: Block
    proposer: str


@dataclass(frozen=True)
class BFTVote:
    height: int
    round: int
    phase: str
    block_hash: str
    block: Block
    sender: str


class TendermintBFT(ConsensusProtocol):
    def __init__(
        self,
        node_id: str,
        node_ids: list[str],
        round_timeout: float,
        txs_per_block: int,
        seed: int = 42,
        byzantine: bool = False,
    ) -> None:
        self.node_id = node_id
        self.node_ids = node_ids
        self.round_timeout = round_timeout
        self.txs_per_block = txs_per_block
        self.byzantine = byzantine
        self.rng = np.random.default_rng(seed)
        self.chain = [genesis_block()]
        self.height = 1
        self.round = 0
        self.phase = "PROPOSE"
        self.locked_block: Block | None = None
        self.votes: defaultdict[tuple[int, int, str, str], set[str]] = defaultdict(set)
        self._sender_votes: dict[tuple[int, int, str, str], str] = {}
        self._sent_votes: set[tuple[int, int, str]] = set()
        self._proposed_rounds: set[tuple[int, int]] = set()
        self._committed_heights: set[int] = set()
        self._timeout_rounds: set[tuple[int, int]] = set()
        self._node = None
        self._network = None
        self._metrics = None
        self._env = None

    def proposer_for_round(self, round: int) -> str:
        return self.node_ids[round % len(self.node_ids)]

    def quorum_size(self) -> int:
        return math.ceil(2 * len(self.node_ids) / 3)

    def run(self, env, node, network, metrics):
        self._env = env
        self._node = node
        self._network = network
        self._metrics = metrics
        self._ensure_bft_metrics(metrics)
        env.process(self._message_loop(env))
        env.process(self._round_loop(env))
        while True:
            yield env.timeout(self.round_timeout)

    def on_message(self, msg) -> None:
        if msg.type == MessageType.PROPOSAL and isinstance(msg.payload, BFTProposal):
            self._handle_proposal(msg.payload)
        elif msg.type == MessageType.VOTE and isinstance(msg.payload, BFTVote):
            self._handle_vote(msg.payload)

    def get_canonical_chain(self) -> list[Block]:
        return list(self.chain)

    def vote_message(self, sender: str, vote: BFTVote) -> Message:
        return Message(
            type=MessageType.VOTE,
            sender=sender,
            receiver=self.node_id,
            payload=vote,
            sent_at=self._now(),
        )

    def proposal_message(self, sender: str, proposal: BFTProposal) -> Message:
        return Message(
            type=MessageType.PROPOSAL,
            sender=sender,
            receiver=self.node_id,
            payload=proposal,
            sent_at=self._now(),
        )

    def _message_loop(self, env):
        while True:
            while self._node is not None and self._node.inbox:
                self.on_message(self._node.inbox.pop(0))
            yield env.timeout(0.01)

    def _round_loop(self, env):
        while True:
            round_key = (self.height, self.round)
            if self.proposer_for_round(self.round) == self.node_id:
                self._broadcast_proposal()
            yield env.timeout(self.round_timeout)
            if round_key == (self.height, self.round):
                self._timeout_round()

    def _broadcast_proposal(self) -> None:
        round_key = (self.height, self.round)
        if round_key in self._proposed_rounds:
            return
        self._proposed_rounds.add(round_key)
        block = self._make_block()
        proposal = BFTProposal(
            height=self.height,
            round=self.round,
            block=block,
            proposer=self.node_id,
        )
        self._handle_proposal(proposal)
        for peer_id in self._peers():
            self._send_message(peer_id, MessageType.PROPOSAL, proposal)

    def _handle_proposal(self, proposal: BFTProposal) -> None:
        if proposal.height != self.height or proposal.round != self.round:
            return
        if proposal.proposer != self.proposer_for_round(proposal.round):
            return
        if proposal.block.prev_hash != self.chain[-1].hash:
            return
        self.phase = "PREVOTE"
        self._send_vote("PREVOTE", proposal.block)

    def _handle_vote(self, vote: BFTVote) -> None:
        if vote.height != self.height or vote.round != self.round:
            return
        sender_key = (vote.height, vote.round, vote.phase, vote.sender)
        prior_hash = self._sender_votes.get(sender_key)
        if prior_hash is not None:
            if prior_hash != vote.block_hash:
                logger.warning(
                    "equivocation detected from %s at height=%s round=%s phase=%s",
                    vote.sender,
                    vote.height,
                    vote.round,
                    vote.phase,
                )
            return
        self._sender_votes[sender_key] = vote.block_hash
        vote_key = (vote.height, vote.round, vote.phase, vote.block_hash)
        self.votes[vote_key].add(vote.sender)

        if vote.phase == "PREVOTE" and len(self.votes[vote_key]) >= self.quorum_size():
            if self.phase != "PRECOMMIT":
                self.phase = "PRECOMMIT"
                self.locked_block = vote.block
                self._send_vote("PRECOMMIT", vote.block)

        if vote.phase == "PRECOMMIT" and len(self.votes[vote_key]) >= self.quorum_size():
            self._commit(vote.block)

    def _send_vote(self, phase: str, block: Block) -> None:
        vote_key = (self.height, self.round, phase)
        if vote_key in self._sent_votes:
            return
        self._sent_votes.add(vote_key)

        if self.byzantine and phase == "PREVOTE":
            self._send_byzantine_prevotes(block)
            return

        vote = BFTVote(
            height=self.height,
            round=self.round,
            phase=phase,
            block_hash=block.hash,
            block=block,
            sender=self.node_id,
        )
        self.on_message(self.vote_message(self.node_id, vote))
        for peer_id in self._peers():
            self._send_message(peer_id, MessageType.VOTE, vote)

    def _send_byzantine_prevotes(self, block: Block) -> None:
        conflicting_block = self._conflicting_block(block)
        peers = self._peers()
        split = len(peers) // 2
        original_vote = BFTVote(
            height=self.height,
            round=self.round,
            phase="PREVOTE",
            block_hash=block.hash,
            block=block,
            sender=self.node_id,
        )
        conflicting_vote = BFTVote(
            height=self.height,
            round=self.round,
            phase="PREVOTE",
            block_hash=conflicting_block.hash,
            block=conflicting_block,
            sender=self.node_id,
        )
        self.on_message(self.vote_message(self.node_id, original_vote))
        for peer_id in peers[:split]:
            self._send_message(peer_id, MessageType.VOTE, original_vote)
        for peer_id in peers[split:]:
            self._send_message(peer_id, MessageType.VOTE, conflicting_vote)

    def _commit(self, block: Block) -> None:
        if block.height in self._committed_heights:
            return
        if block.height != self.height or block.prev_hash != self.chain[-1].hash:
            return
        if not self._partition_has_quorum():
            self._record_liveness_failure()
            return
        object.__setattr__(block, "committed_at", self._now())
        self.chain.append(block)
        self._committed_heights.add(block.height)
        self._record_commit(block)
        self._record_finality(block)
        self.height += 1
        self.round = 0
        self.phase = "PROPOSE"
        self.locked_block = None
        logger.info("BFT block committed node=%s height=%s hash=%s", self.node_id, block.height, block.hash)

    def _timeout_round(self) -> None:
        timeout_key = (self.height, self.round)
        if timeout_key in self._timeout_rounds:
            return
        self._timeout_rounds.add(timeout_key)
        self._send_timeout()
        self._record_liveness_failure()
        self.round += 1
        self.phase = "PROPOSE"
        self.locked_block = None

    def _send_timeout(self) -> None:
        payload = {"height": self.height, "round": self.round, "sender": self.node_id}
        for peer_id in self._peers():
            self._send_message(peer_id, MessageType.TIMEOUT, payload)

    def _send_message(self, receiver_id: str, message_type: MessageType, payload) -> None:
        if self._network is None:
            return
        if self._metrics is not None:
            self._metrics.record_message_sent()
        self._network.send(
            self.node_id,
            receiver_id,
            Message(
                type=message_type,
                sender=self.node_id,
                receiver=receiver_id,
                payload=payload,
                sent_at=self._now(),
            ),
        )

    def _make_block(self) -> Block:
        payload = f"{self.height}|{self.round}|{self.node_id}|{self.txs_per_block}"
        payload_hash = hashlib.sha256(payload.encode()).hexdigest()
        return Block(
            height=self.height,
            prev_hash=self.chain[-1].hash,
            payload_hash=payload_hash,
            timestamp=self._now(),
            protocol_fields={
                "round": self.round,
                "proposer": self.node_id,
                "tx_count": self.txs_per_block,
            },
        )

    def _conflicting_block(self, block: Block) -> Block:
        return Block(
            height=block.height,
            prev_hash=block.prev_hash,
            payload_hash=hashlib.sha256(f"{block.payload_hash}|conflict".encode()).hexdigest(),
            timestamp=block.timestamp,
            protocol_fields={**block.protocol_fields, "equivocation": self.node_id},
        )

    def _partition_has_quorum(self) -> bool:
        if self._network is None or getattr(self._network, "_partition", None) is None:
            return True
        group1, group2 = self._network._partition
        if self.node_id in group1:
            return len(group1) >= self.quorum_size()
        if self.node_id in group2:
            return len(group2) >= self.quorum_size()
        return True

    def _record_commit(self, block: Block) -> None:
        if self._metrics is None:
            return
        commits = getattr(self._metrics, "_bft_commit_events", [])
        committed_keys = getattr(self._metrics, "_bft_commit_keys", set())
        safety_by_height = getattr(self._metrics, "_bft_hash_by_height", {})
        existing_hash = safety_by_height.get(block.height)
        if existing_hash is not None and existing_hash != block.hash:
            self._metrics.record_safety_violation()
        safety_by_height[block.height] = block.hash
        key = (block.height, block.hash)
        if key not in committed_keys:
            commits.append((block.height, block.hash, self._now()))
            committed_keys.add(key)
        setattr(self._metrics, "_bft_commit_events", commits)
        setattr(self._metrics, "_bft_commit_keys", committed_keys)
        setattr(self._metrics, "_bft_hash_by_height", safety_by_height)

    def _record_finality(self, block: Block) -> None:
        if self._metrics is None:
            return
        for index in range(self.txs_per_block):
            tx_id = f"bft-{block.height}-{index}"
            self._metrics.record_tx_submitted(tx_id, block.timestamp)
            self._metrics.record_tx_confirmed(tx_id, self._now())

    def _record_liveness_failure(self) -> None:
        if self._metrics is not None:
            self._metrics.record_liveness_failure()

    def _peers(self) -> list[str]:
        return [node_id for node_id in self.node_ids if node_id != self.node_id]

    def _now(self) -> float:
        return float(self._env.now) if self._env is not None else 0.0

    @staticmethod
    def _ensure_bft_metrics(metrics) -> None:
        if hasattr(metrics, "bft_total_commits"):
            return

        def bft_total_commits(self):
            return len(getattr(self, "_bft_commit_events", []))

        def bft_commits_before(self, end_time):
            return sum(1 for _height, _hash, committed_at in getattr(self, "_bft_commit_events", []) if committed_at < end_time)

        def bft_commits_between(self, start_time, end_time):
            return sum(
                1
                for _height, _hash, committed_at in getattr(self, "_bft_commit_events", [])
                if start_time <= committed_at < end_time
            )

        metrics.bft_total_commits = MethodType(bft_total_commits, metrics)
        metrics.bft_commits_before = MethodType(bft_commits_before, metrics)
        metrics.bft_commits_between = MethodType(bft_commits_between, metrics)
