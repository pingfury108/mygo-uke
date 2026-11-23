import torch
from pytest import approx

from chordd.real import checkpoint_parts, window_starts


def test_window_starts_covers_every_frame_with_overlap():
    starts = window_starts(390, 108, 54)

    assert starts[0] == 0
    assert all(b - a == 54 for a, b in zip(starts, starts[1:]))
    assert starts[-1] + 108 >= 390
    covered = set()
    for start in starts:
        covered.update(range(start, start + 108))
    assert all(frame in covered for frame in range(390))


def test_window_starts_handles_short_and_empty_audio():
    assert window_starts(0, 108, 54) == []
    assert window_starts(50, 108, 54) == [0]


def test_checkpoint_parts_reads_both_layouts():
    model = {"fc.weight": torch.zeros(2, 2)}

    state, mean, std = checkpoint_parts(
        {"model": model, "mean": torch.tensor(-2.0), "std": torch.tensor(1.5)}
    )
    assert state is model
    assert float(mean) == approx(-2.0)
    assert float(std) == approx(1.5)

    state, mean, std = checkpoint_parts(
        {
            "model_state_dict": model,
            "normalization": {"mean": torch.tensor(-2.3), "std": torch.tensor(1.9)},
        }
    )
    assert state is model
    assert float(mean) == approx(-2.3)
    assert float(std) == approx(1.9)
