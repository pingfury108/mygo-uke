"""Real beat/downbeat and chord inference for chordd.

Heavy imports live here so the base install and the mock analyzer stay light.
"""

from __future__ import annotations

import itertools
import os
import threading
from pathlib import Path
from typing import Any

from .chords import detect_key, infer_time_signature, segments_from_frames
from .models import AnalyzeResult

FRAME_SECONDS = 10.0 / 108.0

BTC_CONFIG = {
    "feature_size": 144,
    "timestep": 108,
    "num_chords": 170,
    "input_dropout": 0.2,
    "layer_dropout": 0.2,
    "attention_dropout": 0.2,
    "relu_dropout": 0.2,
    "num_layers": 8,
    "num_heads": 4,
    "hidden_size": 128,
    "total_key_depth": 128,
    "total_value_depth": 128,
    "filter_size": 128,
    "probs_out": True,
}

_engine: Engine | None = None
_engine_lock = threading.Lock()
_inference_lock = threading.Lock()


class Engine:
    def __init__(self) -> None:
        try:
            import torch
            from beat_this.inference import Audio2Beats
        except ImportError as exc:
            raise RuntimeError(
                "real analyzer needs `uv sync --extra analysis`"
            ) from exc

        from .btc_model import BTCModel
        from .model_store import ensure_btc_weights

        threads = int(os.environ.get("CHORDD_TORCH_THREADS", "4"))
        torch.set_num_threads(max(1, threads))
        self.torch = torch
        self.device = torch.device(os.environ.get("CHORDD_DEVICE", "cpu"))
        self.beat_model = Audio2Beats(
            checkpoint_path=os.environ.get("CHORDD_BEAT_MODEL", "final0"),
            device=str(self.device),
            dbn=False,
        )

        checkpoint = torch.load(
            ensure_btc_weights(), map_location="cpu", weights_only=False
        )
        self.btc = BTCModel(BTC_CONFIG).to(self.device).eval()
        self.btc.load_state_dict(checkpoint["model"])
        self.mean = torch.as_tensor(checkpoint["mean"], dtype=torch.float32, device=self.device)
        self.std = torch.as_tensor(checkpoint["std"], dtype=torch.float32, device=self.device)
        self.batch_size = max(1, int(os.environ.get("CHORDD_BTC_BATCH", "8")))

    def beats(self, signal: Any, sample_rate: int) -> tuple[list[float], list[float]]:
        import numpy as np

        beats, downbeats = self.beat_model(signal, sample_rate)
        beats = [round(float(value), 3) for value in np.asarray(beats).reshape(-1)]
        downbeats = [round(float(value), 3) for value in np.asarray(downbeats).reshape(-1)]
        beats = sorted(value for value in beats if value >= 0)
        downbeats = sorted(value for value in downbeats if value >= 0)
        if not downbeats and beats:
            downbeats = beats[::4]
        if not downbeats:
            downbeats = [0.0]
        return beats, downbeats

    def chords(self, signal: Any, sample_rate: int) -> tuple[list[int], list[float]]:
        import librosa
        import numpy as np

        cqt = librosa.cqt(
            signal,
            sr=sample_rate,
            n_bins=144,
            bins_per_octave=24,
            hop_length=2048,
        )
        feature = np.log(np.abs(cqt) + 1e-6).T.astype(np.float32)
        frames = feature.shape[0]
        if frames == 0:
            return [], []

        pad = (-frames) % BTC_CONFIG["timestep"]
        if pad:
            feature = np.pad(feature, ((0, pad), (0, 0)), mode="constant")
        tensor = self.torch.from_numpy(feature).to(self.device)
        tensor = (tensor - self.mean) / self.std
        tensor = tensor.reshape(-1, BTC_CONFIG["timestep"], BTC_CONFIG["feature_size"])

        labels: list[np.ndarray] = []
        confidence: list[np.ndarray] = []
        with self.torch.inference_mode():
            for start in range(0, tensor.shape[0], self.batch_size):
                batch = tensor[start : start + self.batch_size]
                hidden, _ = self.btc.self_attn_layers(batch)
                logits = self.btc.output_layer(hidden)
                probs = self.torch.softmax(logits, dim=-1)
                conf, index = probs.max(dim=-1)
                labels.append(index.cpu().numpy().reshape(-1))
                confidence.append(conf.cpu().numpy().reshape(-1))

        return (
            np.concatenate(labels)[:frames].astype(int).tolist(),
            np.concatenate(confidence)[:frames].astype(float).tolist(),
        )


def _get_engine() -> Engine:
    global _engine
    if _engine is None:
        with _engine_lock:
            if _engine is None:
                _engine = Engine()
    return _engine


def analyze(audio: Path, duration: float | None, progress) -> AnalyzeResult:
    try:
        import librosa
    except ImportError as exc:
        raise RuntimeError("real analyzer needs `uv sync --extra analysis`") from exc

    progress(0.03)
    signal, sample_rate = librosa.load(str(audio), sr=22050, mono=True)
    total = float(len(signal) / sample_rate)
    if total <= 0:
        raise RuntimeError("audio is empty")

    with _inference_lock:
        engine = _get_engine()
        progress(0.12)
        beats, downbeats = engine.beats(signal, sample_rate)
        progress(0.35)
        labels, confidence = engine.chords(signal, sample_rate)
        progress(0.82)

    chords = segments_from_frames(labels, confidence, FRAME_SECONDS, total)
    if not chords:
        raise RuntimeError("BTC returned no chord segments")
    tempo = _tempo(beats)
    progress(0.95)
    return AnalyzeResult(
        tempo=tempo,
        timeSignature=infer_time_signature(beats, downbeats),
        key=detect_key(chords),
        beats=beats,
        downbeats=downbeats,
        chords=chords,
        sections=[],
    )


def _tempo(beats: list[float]) -> float:
    if len(beats) < 2:
        return 0.0
    gaps = [b - a for a, b in itertools.pairwise(beats) if b - a > 0.05]
    if not gaps:
        return 0.0
    gaps.sort()
    return round(60.0 / gaps[len(gaps) // 2], 1)
