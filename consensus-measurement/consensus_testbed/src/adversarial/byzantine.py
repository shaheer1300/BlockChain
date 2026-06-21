import hashlib

from src.consensus.bft import BFTVote
from src.core.block import Block
from src.core.message import Message, MessageType
from src.core.node import Node, NodeRole


class EquivocatingNode(Node):
    def __init__(
        self,
        id: str,
        role: NodeRole = NodeRole.BYZANTINE,
        peers: list[str] | None = None,
        byzantine_strategy: str = "split_prevote",
    ) -> None:
        super().__init__(id=id, role=role)
        self.peers = peers or []
        self.byzantine_strategy = byzantine_strategy

    def dispatch_vote(self, network, vote: BFTVote, sent_at: float) -> None:
        if self.role != NodeRole.BYZANTINE or self.byzantine_strategy != "split_prevote":
            self._broadcast_vote(network, vote, sent_at)
            return
        if vote.phase != "PREVOTE":
            self._broadcast_vote(network, vote, sent_at)
            return

        peers = [peer for peer in self.peers if peer != self.id]
        split = len(peers) // 2
        conflicting_vote = BFTVote(
            height=vote.height,
            round=vote.round,
            phase=vote.phase,
            block_hash=self._conflicting_block(vote.block).hash,
            block=self._conflicting_block(vote.block),
            sender=vote.sender,
        )
        for peer in peers[:split]:
            network.send(self.id, peer, Message(MessageType.VOTE, self.id, peer, vote, sent_at))
        for peer in peers[split:]:
            network.send(self.id, peer, Message(MessageType.VOTE, self.id, peer, conflicting_vote, sent_at))

    def _broadcast_vote(self, network, vote: BFTVote, sent_at: float) -> None:
        for peer in self.peers:
            if peer != self.id:
                network.send(self.id, peer, Message(MessageType.VOTE, self.id, peer, vote, sent_at))

    @staticmethod
    def _conflicting_block(block: Block) -> Block:
        return Block(
            height=block.height,
            prev_hash=block.prev_hash,
            payload_hash=hashlib.sha256(f"{block.payload_hash}|equivocate".encode()).hexdigest(),
            timestamp=block.timestamp,
            protocol_fields={**block.protocol_fields, "equivocation": True},
        )
