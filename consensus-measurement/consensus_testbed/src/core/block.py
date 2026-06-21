from dataclasses import dataclass, field
import hashlib
import json
from typing import Any


@dataclass(frozen=True)
class Block:
    height: int
    prev_hash: str
    payload_hash: str
    timestamp: float
    protocol_fields: dict[str, Any]
    hash: str = field(init=False)

    def __post_init__(self) -> None:
        canonical_string = (
            f"{self.height}|{self.prev_hash}|{self.payload_hash}|"
            f"{self.timestamp}|{json.dumps(self.protocol_fields, sort_keys=True)}"
        )
        object.__setattr__(
            self,
            "hash",
            hashlib.sha256(canonical_string.encode()).hexdigest(),
        )


def genesis_block() -> Block:
    return Block(
        height=0,
        prev_hash="0" * 64,
        payload_hash="0" * 64,
        timestamp=0.0,
        protocol_fields={},
    )
