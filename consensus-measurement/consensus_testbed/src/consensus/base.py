from abc import ABC, abstractmethod


class ConsensusProtocol(ABC):
    @abstractmethod
    def run(self, env, node, network, metrics):
        pass

    @abstractmethod
    def on_message(self, msg) -> None:
        pass

    @abstractmethod
    def get_canonical_chain(self):
        pass
