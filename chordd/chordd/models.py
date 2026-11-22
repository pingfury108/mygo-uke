from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field


class AnalyzeAccepted(BaseModel):
    jobId: str
    etaMs: int


class JobStatus(BaseModel):
    status: Literal["pending", "running", "done", "failed"]
    progress: float = Field(ge=0, le=1)
    error: str | None = None


class ChordSegment(BaseModel):
    start: float
    end: float
    label: str
    conf: float = Field(ge=0, le=1)


class Section(BaseModel):
    start: float
    end: float
    label: str


class AnalyzeResult(BaseModel):
    tempo: float
    timeSignature: list[int]
    key: str
    beats: list[float]
    downbeats: list[float]
    chords: list[ChordSegment]
    sections: list[Section] = Field(default_factory=list)


class Health(BaseModel):
    ok: bool
    analyzer: str
    models: list[str]
