from dataclasses import dataclass
import math

import numpy as np
from scipy import stats

from src.core.metrics import MetricsSnapshot


@dataclass(frozen=True)
class CIResult:
    mean: float
    ci_lower: float
    ci_upper: float
    n: int


def compute_ci(measurements: list[float], confidence: float = 0.95) -> CIResult:
    values = np.array(measurements, dtype=float)
    n = int(len(values))
    if n == 0:
        return CIResult(mean=math.nan, ci_lower=math.nan, ci_upper=math.nan, n=0)
    mean = float(values.mean())
    if n == 1:
        return CIResult(mean=mean, ci_lower=math.nan, ci_upper=math.nan, n=1)
    if float(values.std(ddof=1)) == 0.0:
        return CIResult(mean=mean, ci_lower=mean, ci_upper=mean, n=n)
    interval = stats.t.interval(
        confidence,
        df=n - 1,
        loc=mean,
        scale=stats.sem(values),
    )
    return CIResult(mean=mean, ci_lower=float(interval[0]), ci_upper=float(interval[1]), n=n)


def summarize(snapshots: list[MetricsSnapshot]) -> dict[str, CIResult]:
    fields = [
        "throughput_tps",
        "mean_finality_latency_s",
        "fork_rate",
        "safety_violations",
        "liveness_failures",
        "message_count",
    ]
    return {
        field: compute_ci([float(getattr(snapshot, field)) for snapshot in snapshots])
        for field in fields
    }
