from pathlib import Path
import sys

import click

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from src.analysis.visualizer import generate_comparison_plots


@click.command()
@click.option("--results-dir", default="results/")
@click.option("--output-dir", default="results/plots/")
def main(results_dir: str, output_dir: str) -> None:
    generate_comparison_plots(results_dir=results_dir, output_dir=output_dir)
    click.echo(f"generated plots in {output_dir}")


if __name__ == "__main__":
    main()
