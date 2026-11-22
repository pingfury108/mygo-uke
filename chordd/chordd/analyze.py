from __future__ import annotations

import os
import time
from collections.abc import Callable
from dataclasses import dataclass, field
from pathlib import Path
from typing import Protocol

from .models import AnalyzeResult, ChordSegment

Progress = Callable[[float], None]


class Analyzer(Protocol):
    name: str
    models: list[str]

    def analyze(self, audio: Path, duration: float | None, progress: Progress) -> AnalyzeResult:
        ...


@dataclass
class MockAnalyzer:
    """Deterministic result for wiring the client before models are installed."""

    name: str = "mock"
    models: list[str] = field(default_factory=list)

    def analyze(self, audio: Path, duration: float | None, progress: Progress) -> AnalyzeResult:
        total = float(duration or 180.0)
        total = max(8.0, min(total, 6 * 60.0))
        tempo = 68.0
        beat = 60.0 / tempo
        bar = beat * 4
        downbeats = [round(0.52 + i * bar, 3) for i in range(int((total - 0.52) / bar) + 1)]
        beats = [round(0.52 + i * beat, 3) for i in range(int((total - 0.52) / beat) + 1)]
        progression = ["C", "G", "Am", "Em", "F", "C", "Dm", "G"]
        chords: list[ChordSegment] = []
        for i, start in enumerate(downbeats):
            end = downbeats[i + 1] if i + 1 < len(downbeats) else total
            if end - start < 0.2:
                continue
            chords.append(
                ChordSegment(
                    start=round(start, 3),
                    end=round(end, 3),
                    label=progression[i % len(progression)],
                    conf=0.52 if i % 11 == 4 else 0.9,
                )
            )
        for i, value in enumerate((0.12, 0.32, 0.58, 0.82)):
            time.sleep(0.05)
            progress(value)
        return AnalyzeResult(
            tempo=tempo,
            timeSignature=[4, 4],
            key="C",
            beats=beats,
            downbeats=downbeats,
            chords=chords,
            sections=[],
        )


@dataclass
class RealAnalyzer:
    """Adapter seam for beat_this + BTC.

    The model code stays out of the base install: `uv sync --extra analysis`
    installs it. Keeping the seam explicit means the HTTP contract, cache,
    and job lifecycle are already testable without a 300MB download.
    """

    name: str = "beat-this"
    models: list[str] = field(
        default_factory=lambda: ["btc-large-voca", "beat-this"]
    )

    def analyze(self, audio: Path, duration: float | None, progress: Progress) -> AnalyzeResult:
        from .real import analyze as run_real

        return run_real(audio, duration, progress)


def analyzer_from_env() -> Analyzer:
    choice = os.environ.get("CHORDD_ANALYZER", "mock").strip().lower()
    if choice in {"beat-this", "beat_this", "real", "auto"}:
        return RealAnalyzer()
    return MockAnalyzer()
