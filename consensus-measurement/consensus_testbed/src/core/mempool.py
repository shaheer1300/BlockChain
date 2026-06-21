from collections import OrderedDict

import numpy as np


class Mempool:
    def __init__(self, max_size: int, seed: int) -> None:
        self.max_size = max_size
        self._txs: OrderedDict[str, None] = OrderedDict()
        self._rng = np.random.default_rng(seed)

    def __contains__(self, tx_id: str) -> bool:
        return tx_id in self._txs

    def __len__(self) -> int:
        return len(self._txs)

    def add(self, tx_id: str) -> None:
        if tx_id in self._txs:
            return
        if len(self._txs) >= self.max_size:
            evict_index = int(self._rng.integers(0, len(self._txs)))
            evict_tx = list(self._txs.keys())[evict_index]
            del self._txs[evict_tx]
        self._txs[tx_id] = None

    def select(self, n: int) -> list[str]:
        tx_ids = list(self._txs.keys())
        if n >= len(tx_ids):
            return tx_ids
        selected = self._rng.choice(tx_ids, size=n, replace=False)
        return [str(tx_id) for tx_id in selected]

    def remove_confirmed(self, tx_ids: list[str]) -> None:
        for tx_id in tx_ids:
            self._txs.pop(tx_id, None)
