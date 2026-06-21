from src.experiments.runner import ExperimentConfig


def latency_sweep_pow(latencies: list[float], **kwargs) -> list[ExperimentConfig]:
    return [
        ExperimentConfig(protocol="pow", mean_latency=latency, **kwargs)
        for latency in latencies
    ]


def byzantine_sweep_bft(f_values: list[int], **kwargs) -> list[ExperimentConfig]:
    return [
        ExperimentConfig(protocol="bft", f_byzantine=f_value, **kwargs)
        for f_value in f_values
    ]


def pox_k_sweep(k_values: list[int], **kwargs) -> list[ExperimentConfig]:
    return [
        ExperimentConfig(protocol="pox", k_confirmations=k_value, **kwargs)
        for k_value in k_values
    ]
