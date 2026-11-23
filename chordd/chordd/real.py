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
DEFAULT_OVERLAP = 0.5

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


def window_starts(frames: int, timestep: int, stride: int) -> list[int]:
    """Return window offsets so every frame is covered at least once."""
    if frames <= 0:
        return []
    if frames <= timestep:
        return [0]
    pad = -(frames - timestep) % stride
    return list(range(0, frames + pad - timestep + 1, stride))


def checkpoint_parts(checkpoint: dict) -> tuple[dict, Any, Any]:
    """Read weights and normalization from either BTC checkpoint layout."""
    state = checkpoint.get("model_state_dict") or checkpoint.get("model")
    if not isinstance(state, dict):
        raise RuntimeError("BTC checkpoint has no model weights")
    normalization = checkpoint.get("normalization")
    if isinstance(normalization, dict) and "mean" in normalization and "std" in normalization:
        return state, normalization["mean"], normalization["std"]
    if "mean" in checkpoint and "std" in checkpoint:
        return state, checkpoint["mean"], checkpoint["std"]
    raise RuntimeError("BTC checkpoint has no normalization stats")


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
        state, mean, std = checkpoint_parts(checkpoint)
        self.btc.load_state_dict(state)
        self.mean = torch.as_tensor(mean, dtype=torch.float32, device=self.device)
        self.std = torch.as_tensor(std, dtype=torch.float32, device=self.device)
        self.batch_size = max(1, int(os.environ.get("CHORDD_BTC_BATCH", "8")))
        overlap = float(os.environ.get("CHORDD_BTC_OVERLAP", str(DEFAULT_OVERLAP)))
        self.overlap = min(0.75, max(0.0, overlap))

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

    def chords(self, signal: Any, sample_rate: int) -> tuple[list[int], list[float], Any]:
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

        timestep = BTC_CONFIG["timestep"]
        stride = max(1, int(round(timestep * (1.0 - self.overlap))))
        starts = window_starts(frames, timestep, stride)
        pad = starts[-1] + timestep - frames
        if pad:
            feature = np.pad(feature, ((0, pad), (0, 0)), mode="constant")
        tensor = self.torch.from_numpy(feature).to(self.device)
        tensor = (tensor - self.mean) / self.std

        weight = np.zeros((frames, BTC_CONFIG["num_chords"]), dtype=np.float32)
        cover = np.zeros(frames, dtype=np.float32)
        with self.torch.inference_mode():
            for offset in range(0, len(starts), self.batch_size):
                chunk = starts[offset : offset + self.batch_size]
                batch = self.torch.stack([tensor[start : start + timestep] for start in chunk])
                hidden, _ = self.btc.self_attn_layers(batch)
                logits = self.btc.output_layer(hidden)
                probs = self.torch.softmax(logits, dim=-1).cpu().numpy()
                for j, start in enumerate(chunk):
                    valid = min(timestep, frames - start)
                    if valid <= 0:
                        continue
                    weight[start : start + valid] += probs[j, :valid]
                    cover[start : start + valid] += 1

        labels = weight.argmax(axis=1)
        confidence = np.divide(
            weight.max(axis=1),
            cover,
            out=np.zeros(frames, dtype=np.float32),
            where=cover > 0,
        )
        probs = np.divide(
            weight,
            cover[:, None],
            out=np.zeros_like(weight),
            where=cover[:, None] > 0,
        )
        return (
            labels.astype(int).tolist(),
            np.clip(confidence, 0.0, 1.0).tolist(),
            probs,
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
        labels, confidence, probs = engine.chords(signal, sample_rate)
        progress(0.82)

    chords = segments_from_frames(
        labels, confidence, FRAME_SECONDS, total, probs=probs, merge_seconds=0.3
    )
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
