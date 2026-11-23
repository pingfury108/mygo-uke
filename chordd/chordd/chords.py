"""Pure chord-label helpers shared by BTC inference, key detection and tests."""

from __future__ import annotations

import itertools
from collections.abc import Sequence
from statistics import median

from .models import ChordSegment

ROOTS = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"]
QUALITIES = [
    "min",
    "maj",
    "dim",
    "aug",
    "min6",
    "maj6",
    "min7",
    "minmaj7",
    "maj7",
    "7",
    "dim7",
    "hdim7",
    "sus2",
    "sus4",
]

_QUALITY_SUFFIX = {
    "min": "m",
    "maj": "",
    "dim": "dim",
    "aug": "aug",
    "min6": "m6",
    "maj6": "6",
    "min7": "m7",
    "minmaj7": "mMaj7",
    "maj7": "maj7",
    "7": "7",
    "dim7": "dim7",
    "hdim7": "m7b5",
    "sus2": "sus2",
    "sus4": "sus4",
}

_SUFFIX_QUALITY = {value: key for key, value in _QUALITY_SUFFIX.items()}

_SIMPLE_QUALITY = {
    "min": "min",
    "maj": "maj",
    "dim": "min",
    "aug": "maj",
    "min6": "min",
    "maj6": "maj",
    "min7": "min7",
    "minmaj7": "min7",
    "maj7": "maj7",
    "7": "7",
    "dim7": "min7",
    "hdim7": "min7",
    "sus2": "maj",
    "sus4": "maj",
}

_INTERVALS = {
    "maj": (0, 4, 7),
    "min": (0, 3, 7),
    "dim": (0, 3, 6),
    "aug": (0, 4, 8),
    "min6": (0, 3, 7, 9),
    "maj6": (0, 4, 7, 9),
    "min7": (0, 3, 7, 10),
    "minmaj7": (0, 3, 7, 11),
    "maj7": (0, 4, 7, 11),
    "7": (0, 4, 7, 10),
    "dim7": (0, 3, 6, 9),
    "hdim7": (0, 3, 6, 10),
    "sus2": (0, 2, 7),
    "sus4": (0, 5, 7),
}

_MAJOR_PROFILE = [6.35, 2.23, 3.48, 2.33, 4.38, 4.09, 2.52, 5.19, 2.39, 3.66, 2.29, 2.88]
_MINOR_PROFILE = [6.33, 2.68, 3.52, 5.38, 2.60, 3.53, 2.54, 4.75, 3.98, 2.69, 3.34, 3.17]


def label_for_index(index: int) -> str | None:
    """Map BTC's 170-class vocabulary to compact client labels."""
    if index < 0 or index >= 168:
        return None
    root = ROOTS[index // 14]
    quality = QUALITIES[index % 14]
    return root + _QUALITY_SUFFIX[quality]


def simple_label(label: str) -> str:
    """Fold extensions into the five qualities the editor exposes."""
    parsed = parse_label(label)
    if parsed is None:
        return label
    root, quality = parsed
    quality = _SIMPLE_QUALITY.get(quality, quality)
    return ROOTS[root] + _QUALITY_SUFFIX[quality]


def parse_label(label: str) -> tuple[int, str] | None:
    """Parse compact (``Cm7``) or colon (``C:min7``) labels."""
    label = label.strip()
    if not label or label in {"N", "X", "N.C.", "NC"}:
        return None
    flats = {"Db": "C#", "Eb": "D#", "Gb": "F#", "Ab": "G#", "Bb": "A#"}
    roots = [(name, name) for name in ROOTS] + list(flats.items())
    root_text = ""
    quality_text = ""
    for name, normalized in sorted(roots, key=lambda item: len(item[0]), reverse=True):
        if label.startswith(name):
            root_text = normalized
            quality_text = label[len(name) :].removeprefix(":")
            break
    if root_text not in ROOTS:
        return None
    quality = quality_text if quality_text in _INTERVALS else _SUFFIX_QUALITY.get(quality_text)
    if quality not in _INTERVALS:
        return None
    return ROOTS.index(root_text), quality


def chord_intervals(label: str) -> tuple[int, tuple[int, ...]] | None:
    parsed = parse_label(label)
    if parsed is None:
        return None
    root, quality = parsed
    return root, _INTERVALS[quality]


def detect_key(chords: Sequence[ChordSegment]) -> str:
    """Krumhansl-style key estimate from duration-weighted chord tones."""
    chroma = [0.0] * 12
    for chord in chords:
        parsed = chord_intervals(chord.label)
        if parsed is None:
            continue
        root, intervals = parsed
        weight = max(0.05, chord.end - chord.start)
        for interval in intervals:
            chroma[(root + interval) % 12] += weight
    if not any(chroma):
        return "C"

    best: tuple[float, int, bool] | None = None
    for tonic in range(12):
        for minor, profile in ((False, _MAJOR_PROFILE), (True, _MINOR_PROFILE)):
            rotated = [profile[(pitch - tonic) % 12] for pitch in range(12)]
            score = _correlation(chroma, rotated)
            if best is None or score > best[0]:
                best = (score, tonic, minor)
    assert best is not None
    return ROOTS[best[1]] + ("m" if best[2] else "")


def infer_time_signature(beats: Sequence[float], downbeats: Sequence[float]) -> list[int]:
    if len(downbeats) < 2:
        return [4, 4]
    counts: list[int] = []
    for start, end in itertools.pairwise(downbeats):
        if end - start <= 0.2:
            continue
        counts.append(sum(1 for beat in beats if start - 0.03 <= beat < end - 0.03))
    if not counts:
        return [4, 4]
    value = round(median(counts))
    if value in (2, 3):
        return [value, 4]
    return [4, 4]


def segments_from_frames(
    labels: Sequence[int],
    confidence: Sequence[float],
    frame_seconds: float,
    duration: float,
    min_seconds: float = 0.12,
    probs: Sequence[Sequence[float]] | None = None,
    candidates: bool = True,
    merge_seconds: float = 0.0,
) -> list[ChordSegment]:
    """Run-length encode per-frame chord ids into API chord segments."""
    if not labels:
        return []
    if duration <= 0:
        duration = len(labels) * frame_seconds

    out: list[ChordSegment] = []
    start = 0
    for end in range(1, len(labels) + 1):
        if end < len(labels) and labels[end] == labels[start]:
            continue
        raw = label_for_index(int(labels[start]))
        label = simple_label(raw) if raw is not None else None
        begin_seconds = start * frame_seconds
        end_seconds = min(end * frame_seconds, duration)
        if label is not None and end_seconds - begin_seconds >= min_seconds:
            window = confidence[start:end]
            conf = sum(window) / len(window) if window else 0.5
            alts: list[str] = []
            if candidates and raw is not None and raw != label:
                alts.append(raw)
            if candidates and probs is not None:
                window = probs[start:end]
                if hasattr(window, "tolist"):
                    window = window.tolist()
                scores = [sum(row[idx] for row in window) for idx in range(170)]
                for idx in sorted(range(170), key=lambda i: scores[i], reverse=True):
                    alt = label_for_index(idx)
                    if alt is None:
                        continue
                    alt = simple_label(alt)
                    if alt == label or alt in alts:
                        continue
                    alts.append(alt)
                    if len(alts) == 3:
                        break
            out.append(
                ChordSegment(
                    start=round(begin_seconds, 3),
                    end=round(end_seconds, 3),
                    label=label,
                    conf=round(max(0.0, min(1.0, conf)), 3),
                    candidates=alts[:3],
                )
            )
        start = end
    return merge_short(out, merge_seconds) if merge_seconds > 0 else out


def merge_short(chords: list[ChordSegment], min_seconds: float = 0.3) -> list[ChordSegment]:
    """Absorb blips shorter than min_seconds into the neighbouring chord."""
    if len(chords) < 2:
        return chords
    out: list[ChordSegment] = []
    for chord in chords:
        if out and chord.end - chord.start < min_seconds:
            prev = out[-1]
            prev.end = chord.end
            prev.conf = max(prev.conf, chord.conf)
            for alt in chord.candidates:
                if alt != prev.label and alt not in prev.candidates:
                    prev.candidates.append(alt)
            prev.candidates = prev.candidates[:3]
            continue
        out.append(chord)
    if len(out) > 1 and out[0].end - out[0].start < min_seconds:
        out[1].start = out[0].start
        del out[0]
    merged: list[ChordSegment] = []
    for chord in out:
        if merged and merged[-1].label == chord.label:
            merged[-1].end = chord.end
            merged[-1].conf = max(merged[-1].conf, chord.conf)
            continue
        merged.append(chord)
    return merged


def _correlation(a: Sequence[float], b: Sequence[float]) -> float:
    mean_a = sum(a) / len(a)
    mean_b = sum(b) / len(b)
    numerator = sum((x - mean_a) * (y - mean_b) for x, y in zip(a, b))
    norm_a = sum((x - mean_a) ** 2 for x in a) ** 0.5
    norm_b = sum((y - mean_b) ** 2 for y in b) ** 0.5
    if norm_a == 0 or norm_b == 0:
        return 0.0
    return numerator / (norm_a * norm_b)
