"""Run the real analyzer on one file and emit JSON plus a downbeat overlay."""

from __future__ import annotations

import argparse
import itertools
import json
import time
from pathlib import Path

from chordd.analyze import RealAnalyzer


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("audio", type=Path)
    parser.add_argument("--duration", type=float, default=None)
    parser.add_argument("--json", type=Path, default=Path("analysis.json"))
    parser.add_argument("--svg", type=Path, default=Path("analysis.svg"))
    args = parser.parse_args()

    started = time.time()
    result = RealAnalyzer().analyze(
        args.audio,
        args.duration,
        lambda value: print(f"progress {value:.0%}", flush=True),
    )
    elapsed = time.time() - started

    args.json.parent.mkdir(parents=True, exist_ok=True)
    args.svg.parent.mkdir(parents=True, exist_ok=True)
    args.json.write_text(
        json.dumps(result.model_dump(), ensure_ascii=False, indent=2), "utf-8"
    )
    render_svg(args.audio, result, args.svg)
    print(
        f"tempo={result.tempo} key={result.key} signature={result.timeSignature} "
        f"beats={len(result.beats)} downbeats={len(result.downbeats)} "
        f"chords={len(result.chords)} elapsed={elapsed:.1f}s"
    )
    print(f"json -> {args.json}")
    print(f"svg  -> {args.svg}")


def render_svg(audio: Path, result, output: Path) -> None:
    import librosa
    import numpy as np

    signal, sample_rate = librosa.load(str(audio), sr=22050, mono=True)
    duration = len(signal) / sample_rate
    width, height = 1600, 460
    left, right, top, bottom = 48, 1548, 52, 330
    plot_width = right - left
    buckets = 1400
    samples = np.abs(signal)
    edges = np.linspace(0, len(samples), buckets + 1, dtype=int)
    peaks = [
        float(samples[a:b].max(initial=0.0)) for a, b in itertools.pairwise(edges)
    ]
    peak = max(peaks) or 1.0
    points = []
    for index, value in enumerate(peaks):
        x = left + plot_width * index / buckets
        y = bottom - (bottom - top) * 0.9 * value / peak
        points.append(f"{x:.1f},{y:.1f}")

    def x_at(seconds: float) -> float:
        return left + plot_width * max(0.0, min(1.0, seconds / duration))

    parts = [
        (
            f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" '
            f'viewBox="0 0 {width} {height}">'
        ),
        '<rect width="100%" height="100%" fill="#111216"/>',
        (
            f'<rect x="{left}" y="{top}" width="{plot_width}" height="{bottom - top}" '
            'fill="#181a20" stroke="#30343d"/>'
        ),
    ]
    parts.append(
        f'<polyline points="{" ".join(points)}" fill="none" stroke="#7d8594" stroke-width="1"/>'
    )
    for value in result.beats:
        x = x_at(value)
        parts.append(
            f'<line x1="{x:.1f}" y1="{bottom - 12}" x2="{x:.1f}" y2="{bottom}" '
            'stroke="#4a505d" stroke-width="1"/>'
        )
    last_label_x = -100.0
    for value in result.downbeats:
        x = x_at(value)
        parts.append(
            f'<line x1="{x:.1f}" y1="{top}" x2="{x:.1f}" y2="{height - 34}" '
            'stroke="#ffb547" stroke-width="1.4"/>'
        )
        if x - last_label_x >= 38:
            parts.append(
                f'<text x="{x + 3:.1f}" y="{top - 8}" fill="#ffb547" '
                f'font-size="10">{value:.1f}</text>'
            )
            last_label_x = x
    last_chord_x = -100.0
    for chord in result.chords:
        x = x_at(chord.start)
        if x - last_chord_x >= 32:
            parts.append(
                f'<text x="{x + 2:.1f}" y="{height - 16}" fill="#e8eaed" '
                f'font-size="11">{chord.label}</text>'
            )
            last_chord_x = x
    parts.append(
        f'<text x="{left}" y="{height - 42}" fill="#9aa1ad" font-size="12">'
        f"{duration:.1f}s · {result.tempo} BPM · {result.key} · "
        f'{result.timeSignature[0]}/{result.timeSignature[1]} · orange = downbeat</text>'
    )
    parts.append("</svg>")
    output.write_text("\n".join(parts), "utf-8")


if __name__ == "__main__":
    main()
