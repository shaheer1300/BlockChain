import simpy


class SimulationEnvironment:
    def __init__(self, duration: float) -> None:
        self.duration = duration
        self.env = simpy.Environment()
        self.network = None
        self.nodes = []

    @property
    def now(self) -> float:
        return self.env.now

    def run(self, duration: float | None = None) -> None:
        self.env.run(until=self.duration if duration is None else duration)
