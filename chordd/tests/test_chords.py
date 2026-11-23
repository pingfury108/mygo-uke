from chordd.analyze import RealAnalyzer
from chordd.chords import (
    detect_key,
    infer_time_signature,
    label_for_index,
    merge_short,
    parse_label,
    segments_from_frames,
    simple_label,
)
from chordd.models import AnalyzeResult, ChordSegment


def chord(start: float, end: float, label: str) -> ChordSegment:
    return ChordSegment(start=start, end=end, label=label, conf=0.9)


def test_btc_vocabulary_maps_to_client_labels():
    assert label_for_index(1) == "C"
    assert label_for_index(0) == "Cm"
    assert label_for_index(168) is None
    assert label_for_index(169) is None


def test_simple_label_folds_rare_qualities():
    assert simple_label("Esus2") == "E"
    assert simple_label("Bdim") == "Bm"
    assert simple_label("Caug") == "C"
    assert simple_label("F#m7") == "F#m7"
    assert simple_label("Gmaj7") == "Gmaj7"
    assert simple_label("B7") == "B7"


def test_segments_from_frames_exposes_candidates():
    probs = [[0.0] * 170 for _ in range(2)]
    probs[0][1] = probs[1][1] = 0.8
    probs[0][0] = probs[1][0] = 0.15
    probs[0][2] = probs[1][2] = 0.05

    chords = segments_from_frames([1, 1], [0.8, 0.8], 0.1, 0.2, probs=probs)

    assert chords[0].label == "C"
    assert "Cm" in chords[0].candidates
    assert len(chords[0].candidates) <= 3


def test_merge_short_absorbs_fragments():
    chords = [chord(0, 1, "C"), chord(1, 1.2, "G"), chord(1.2, 2, "C")]

    merged = merge_short(chords, 0.3)

    assert len(merged) == 1
    assert merged[0].label == "C"
    assert merged[0].end == 2


def test_parse_label_accepts_compact_colon_and_flat_labels():
    assert parse_label("Cm7") == (0, "min7")
    assert parse_label("C:min7") == (0, "min7")
    assert parse_label("Bbmaj7") == (10, "maj7")
    assert parse_label("C#dim") == (1, "dim")
    assert parse_label("N") is None


def test_segments_from_frames_run_length_encode_and_skip_no_chord():
    labels = [1, 1, 1, 15, 15, 168]
    confidence = [0.8, 0.8, 1.0, 0.6, 0.6, 0.2]

    chords = segments_from_frames(labels, confidence, 0.1, 0.6, min_seconds=0.0)

    assert [(item.label, item.start, item.end) for item in chords] == [
        ("C", 0.0, 0.3),
        ("C#", 0.3, 0.5),
    ]
    assert chords[0].conf == 0.867


def test_detect_key_from_common_progressions():
    assert detect_key([chord(0, 4, "C"), chord(4, 8, "G"), chord(8, 12, "Am"), chord(12, 16, "F")]) == "C"
    assert detect_key([chord(0, 4, "Am"), chord(4, 8, "Dm"), chord(8, 12, "E7")]) == "Am"


def test_infer_time_signature_from_downbeats():
    beats = [0.0, 0.5, 1.0, 1.5, 2.0, 2.5, 3.0, 3.5]
    assert infer_time_signature(beats, [0.0, 2.0, 4.0]) == [4, 4]
    assert infer_time_signature(beats[:6], [0.0, 1.5, 3.0]) == [3, 4]


def test_real_analyzer_delegates_to_real_module(monkeypatch):
    called = {}

    def fake(audio, duration, progress):
        called["audio"] = audio
        return AnalyzeResult(
            tempo=120,
            timeSignature=[4, 4],
            key="C",
            beats=[],
            downbeats=[],
            chords=[],
        )

    monkeypatch.setattr("chordd.real.analyze", fake)
    result = RealAnalyzer().analyze("song.mp3", 10, lambda value: None)

    assert called["audio"] == "song.mp3"
    assert result.tempo == 120
