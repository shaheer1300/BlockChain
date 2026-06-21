import importlib
import math

from src.core.metrics import MetricsSnapshot


def require_analysis_symbol(name):
    return getattr(importlib.import_module("src.analysis.stats"), name)


def test_confidence_interval_correct():
    compute_ci = require_analysis_symbol("compute_ci")

    result = compute_ci([1.0, 2.0, 3.0, 4.0, 5.0], confidence=0.95)

    assert abs(result.mean - 3.0) < 0.001
    assert abs(result.ci_lower - 1.04) < 0.10
    assert abs(result.ci_upper - 4.96) < 0.10
    assert result.n == 5


def test_summarize_returns_all_fields():
    summarize = require_analysis_symbol("summarize")
    CIResult = require_analysis_symbol("CIResult")
    snapshots = [
        MetricsSnapshot(
            throughput_tps=float(i),
            mean_finality_latency_s=float(i + 10),
            fork_rate=float(i) / 100,
            safety_violations=i % 2,
            liveness_failures=i,
            message_count=i * 10,
        )
        for i in range(5)
    ]

    summary = summarize(snapshots)

    assert set(summary) == {
        "throughput_tps",
        "mean_finality_latency_s",
        "fork_rate",
        "safety_violations",
        "liveness_failures",
        "message_count",
    }
    assert all(isinstance(value, CIResult) for value in summary.values())
    assert all(hasattr(value, field) for value in summary.values() for field in ["mean", "ci_lower", "ci_upper", "n"])


def test_ci_single_sample_returns_nan_interval():
    compute_ci = require_analysis_symbol("compute_ci")

    result = compute_ci([5.0], confidence=0.95)

    assert math.isnan(result.ci_lower) or result.ci_lower == result.mean
