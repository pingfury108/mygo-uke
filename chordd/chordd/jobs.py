from __future__ import annotations

import threading
import uuid
from collections.abc import Callable
from dataclasses import dataclass, field
from pathlib import Path

from .models import AnalyzeResult, JobStatus


@dataclass
class Job:
    id: str
    status: str = "pending"
    progress: float = 0.0
    error: str | None = None
    result: AnalyzeResult | None = None
    lock: threading.Lock = field(default_factory=threading.Lock)

    def snapshot(self) -> JobStatus:
        with self.lock:
            return JobStatus(status=self.status, progress=self.progress, error=self.error)


class Jobs:
    def __init__(self) -> None:
        self._jobs: dict[str, Job] = {}
        self._lock = threading.Lock()

    def create(self) -> Job:
        job = Job(id=uuid.uuid4().hex[:12])
        with self._lock:
            self._jobs[job.id] = job
        return job

    def get(self, job_id: str) -> Job | None:
        with self._lock:
            return self._jobs.get(job_id)

    def run(
        self,
        job: Job,
        fn: Callable[[Callable[[float], None]], AnalyzeResult],
        on_done: Callable[[AnalyzeResult], None] | None = None,
    ) -> None:
        def progress(value: float) -> None:
            with job.lock:
                job.status = "running"
                job.progress = max(0.0, min(1.0, value))

        def work() -> None:
            with job.lock:
                job.status = "running"
            try:
                result = fn(progress)
                if on_done is not None:
                    on_done(result)
                with job.lock:
                    job.result = result
                    job.progress = 1.0
                    job.status = "done"
            except Exception as exc:  # noqa: BLE001 - reported to the client
                with job.lock:
                    job.error = str(exc)
                    job.status = "failed"

        threading.Thread(target=work, daemon=True, name=f"chordd-{job.id}").start()


def cleanup(path: Path) -> None:
    try:
        path.unlink(missing_ok=True)
    except OSError:
        pass
