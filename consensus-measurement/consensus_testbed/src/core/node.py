from dataclasses import dataclass, field
from enum import Enum, auto

from src.core.message import Message


class NodeRole(Enum):
    HONEST = auto()
    BYZANTINE = auto()


@dataclass
class Node:
    id: str
    role: NodeRole
    inbox: list[Message] = field(default_factory=list)
