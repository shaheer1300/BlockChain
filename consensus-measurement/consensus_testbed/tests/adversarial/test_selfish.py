import importlib
import logging


logger = logging.getLogger(__name__)


def require_selfish_miner():
    return getattr(importlib.import_module("src.adversarial.selfish_miner"), "SelfishMiner")


def run_pow_revenue_sim(selfish=False):
    SelfishMiner = require_selfish_miner()
    hash_fractions = {"N0": 0.4}
    hash_fractions.update({f"N{i}": 0.6 / 9 for i in range(1, 10)})
    return SelfishMiner.simulate_revenue(
        hash_fractions=hash_fractions,
        selfish_miner_id="N0" if selfish else None,
        n_blocks=5000,
        seed=42,
    )


def test_selfish_miner_earns_disproportionate_share(caplog):
    caplog.set_level(logging.INFO)
    shares = run_pow_revenue_sim(selfish=True)
    selfish_share = shares.get("N0", 0.0)
    logger.info("selfish miner canonical share: %.4f", selfish_share)

    assert selfish_share > 0.40


def test_honest_miner_earns_fair_share():
    shares = run_pow_revenue_sim(selfish=False)

    for miner_id in [f"N{i}" for i in range(1, 10)]:
        assert 0.04 <= shares.get(miner_id, 0.0) <= 0.10
