from dataclasses import dataclass
from enum import Enum, auto
from typing import Any


class MessageType(Enum):
    GENERIC = auto()
    BLOCK_ANNOUNCE = auto()
    TX_ANNOUNCE = auto()
    VOTE = auto()
    PROPOSAL = auto()
    TIMEOUT = auto()


@dataclass(frozen=True)
class Message:
    type: MessageType
    sender: str
    receiver: str
    payload: Any
    sent_at: float
