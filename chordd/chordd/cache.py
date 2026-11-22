from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path

from .models import AnalyzeResult


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


class Cache:
    def __init__(self, root: Path | None = None) -> None:
        default = Path(os.environ.get("CHORDD_CACHE", "~/.cache/chordd")).expanduser()
        self.root = root or default
        self.root.mkdir(parents=True, exist_ok=True)

    def get(self, digest: str) -> AnalyzeResult | None:
        path = self.root / f"{digest}.json"
        if not path.exists():
            return None
        try:
            return AnalyzeResult.model_validate_json(path.read_text("utf-8"))
        except (OSError, ValueError):
            return None

    def put(self, digest: str, result: AnalyzeResult) -> None:
        path = self.root / f"{digest}.json"
        tmp = path.with_suffix(".tmp")
        tmp.write_text(
            json.dumps(result.model_dump(), ensure_ascii=False, indent=2),
            encoding="utf-8",
        )
        tmp.replace(path)
