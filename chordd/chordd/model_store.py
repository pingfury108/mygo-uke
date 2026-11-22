"""Download and cache model assets outside the repository."""

from __future__ import annotations

import hashlib
import os
import shutil
import urllib.request
from pathlib import Path

BTC_WEIGHTS_URL = (
    "https://raw.githubusercontent.com/jayg996/BTC-ISMIR19/master/test/"
    "btc_model_large_voca.pt"
)
BTC_WEIGHTS_SHA256 = "1673d23f8f9a55ae7f9e8b80a51da616debb22675b8d8b67ea6ce0ef37b0ab51"


def model_root() -> Path:
    path = Path(os.environ.get("CHORDD_MODEL_DIR", "~/.cache/chordd/models")).expanduser()
    path.mkdir(parents=True, exist_ok=True)
    return path


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def ensure_btc_weights() -> Path:
    override = os.environ.get("CHORDD_BTC_WEIGHTS")
    if override:
        path = Path(override).expanduser()
        if not path.is_file():
            raise RuntimeError(f"CHORDD_BTC_WEIGHTS does not exist: {path}")
        return path

    path = model_root() / "btc_model_large_voca.pt"
    if path.is_file() and sha256_file(path) == BTC_WEIGHTS_SHA256:
        return path

    partial = path.with_suffix(".part")
    partial.unlink(missing_ok=True)
    request = urllib.request.Request(BTC_WEIGHTS_URL, headers={"User-Agent": "chordd/0.1"})
    try:
        with urllib.request.urlopen(request, timeout=120) as response, partial.open("wb") as out:
            shutil.copyfileobj(response, out)
        digest = sha256_file(partial)
        if digest != BTC_WEIGHTS_SHA256:
            raise RuntimeError(
                f"BTC weights checksum mismatch: expected {BTC_WEIGHTS_SHA256}, got {digest}"
            )
        partial.replace(path)
    except Exception:
        partial.unlink(missing_ok=True)
        raise
    return path


if __name__ == "__main__":
    print(ensure_btc_weights())
