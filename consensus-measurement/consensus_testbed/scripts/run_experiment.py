from pathlib import Path
import sys

import click
import yaml

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from src.experiments.runner import ExperimentConfig, ExperimentRunner


@click.command()
@click.option("--protocol", type=click.Choice(["pow", "bft", "pox"]), required=True)
@click.option("--config", "config_path", type=click.Path(exists=True), default=None)
@click.option("--trials", type=int, default=10)
@click.option("--sweep", type=click.Choice(["latency"]), default=None)
def main(protocol: str, config_path: str | None, trials: int, sweep: str | None) -> None:
    if sweep == "latency" and protocol == "pow":
        for latency in [0.01, 0.05, 0.10, 0.50, 1.00]:
            config = _config_from_yaml(protocol, config_path, trials)
            config = ExperimentConfig(**{**config.__dict__, "mean_latency": latency})
            ExperimentRunner(config).run()
        click.echo("completed latency sweep")
        return

    config = _config_from_yaml(protocol, config_path, trials)
    result = ExperimentRunner(config).run()
    click.echo(f"completed {protocol} experiment with {len(result.per_trial_metrics)} trials")


def _config_from_yaml(protocol: str, config_path: str | None, trials: int) -> ExperimentConfig:
    path = Path(config_path) if config_path else Path("config") / f"{protocol}_default.yaml"
    values = {}
    if path.exists():
        data = yaml.safe_load(path.read_text()) or {}
        values = data.get(protocol, {})
    mapped = {
        "protocol": protocol,
        "n_nodes": values.get("n_nodes", values.get("n_miners", 10)),
        "n_trials": trials,
        "sim_duration": values.get("sim_duration", 1000.0),
        "mean_latency": values.get("mean_latency", 0.1),
        "latency_std": values.get("latency_std", 0.0),
        "seed": values.get("seed", 42),
        "target_block_time": values.get("target_block_time", 10.0),
        "txs_per_block": values.get("txs_per_block", values.get("txs_per_pox_block", 20)),
        "round_timeout": values.get("round_timeout", 0.5),
        "f_byzantine": values.get("f_byzantine", 0),
        "base_block_time": values.get("base_block_time", 10.0),
        "k_confirmations": values.get("k_confirmations", 6),
    }
    return ExperimentConfig(**mapped)


if __name__ == "__main__":
    main()
