import importlib
import logging
from pathlib import Path

import numpy as np
import pandas as pd


logger = logging.getLogger(__name__)


def require_runner_symbol(name):
    return getattr(importlib.import_module("src.experiments.runner"), name)


def require_visualizer():
    return getattr(importlib.import_module("src.analysis.visualizer"), "generate_comparison_plots")


def make_config(**overrides):
    ExperimentConfig = require_runner_symbol("ExperimentConfig")
    defaults = {
        "protocol": "pow",
        "n_nodes": 5,
        "n_trials": 3,
        "sim_duration": 200.0,
        "mean_latency": 0.1,
        "seed": 42,
    }
    defaults.update(overrides)
    return ExperimentConfig(**defaults)


def run_config(config):
    ExperimentRunner = require_runner_symbol("ExperimentRunner")
    return ExperimentRunner(config).run()


def test_experiment_deterministic():
    config = make_config(protocol="pow", n_nodes=5, n_trials=3, sim_duration=200.0, mean_latency=0.1, seed=42)

    result_1 = run_config(config)
    result_2 = run_config(config)

    assert result_1.per_trial_metrics == result_2.per_trial_metrics


def test_experiment_outputs_csv():
    config = make_config(protocol="pow", n_nodes=5, n_trials=5, sim_duration=200.0, seed=42)

    run_config(config)
    csv_path = Path("results/pow_n5_seed42.csv")
    df = pd.read_csv(csv_path)

    assert csv_path.exists()
    assert len(df) == 5
    assert set(df.columns) == {
        "trial",
        "throughput_tps",
        "mean_finality_latency_s",
        "fork_rate",
        "safety_violations",
        "liveness_failures",
        "message_count",
    }


def test_comparison_plots_generated():
    for protocol in ["pow", "bft", "pox"]:
        config = make_config(protocol=protocol, n_nodes=5, n_trials=3, sim_duration=300.0, seed=42)
        run_config(config)
    generate_comparison_plots = require_visualizer()

    generate_comparison_plots(results_dir="results/")

    for filename in [
        "throughput_comparison.png",
        "latency_comparison.png",
        "fork_rate_vs_latency.png",
    ]:
        path = Path("results/plots") / filename
        assert path.exists()
        assert path.stat().st_size > 5 * 1024


def test_latency_sweep_produces_monotone_fork_rates(caplog):
    caplog.set_level(logging.INFO)
    latencies = [0.01, 0.05, 0.10, 0.50, 1.00]
    fork_rates = []

    for latency in latencies:
        config = make_config(
            protocol="pow",
            n_nodes=10,
            n_trials=5,
            target_block_time=10.0,
            mean_latency=latency,
            seed=42,
        )
        result = run_config(config)
        fork_rates.append(result.summary["fork_rate"].mean)

    correlation = float(np.corrcoef(latencies, fork_rates)[0, 1])
    logger.info("PoW latency sweep Pearson r=%.4f fork_rates=%s", correlation, fork_rates)

    assert all(a < b for a, b in zip(fork_rates, fork_rates[1:]))
    assert correlation > 0.95
