from chordd.cache import Cache
from chordd.models import AnalyzeResult


def result() -> AnalyzeResult:
    return AnalyzeResult(
        tempo=120,
        timeSignature=[4, 4],
        key="C",
        beats=[],
        downbeats=[],
        chords=[],
        sections=[],
    )


def test_cache_keys_are_isolated_by_version(tmp_path):
    Cache(tmp_path, version="old").put("digest", result())

    assert Cache(tmp_path, version="old").get("digest") is not None
    assert Cache(tmp_path, version="new").get("digest") is None
