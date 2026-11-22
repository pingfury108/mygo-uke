from __future__ import annotations

import os
import tempfile
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Annotated

from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import JSONResponse

from .analyze import Analyzer, analyzer_from_env
from .cache import Cache, sha256_file
from .jobs import Jobs, cleanup
from .models import AnalyzeAccepted, AnalyzeResult, Health, JobStatus

MAX_BYTES = 10 * 1024 * 1024


def create_app(analyzer: Analyzer | None = None, cache: Cache | None = None) -> FastAPI:
    jobs = Jobs()
    app_cache = cache or Cache()
    app_analyzer = analyzer or analyzer_from_env()

    @asynccontextmanager
    async def lifespan(_: FastAPI):
        yield

    app = FastAPI(title="chordd", version="0.1.0", lifespan=lifespan)

    @app.get("/healthz", response_model=Health)
    def health() -> Health:
        return Health(ok=True, analyzer=app_analyzer.name, models=app_analyzer.models)

    @app.post("/v1/analyze", response_model=AnalyzeAccepted, status_code=202)
    def analyze(
        audio: Annotated[UploadFile, File()],
        duration: Annotated[float | None, Form()] = None,
        downmix: Annotated[bool, Form()] = True,
    ) -> JSONResponse:
        del downmix
        suffix = Path(audio.filename or "audio.mp3").suffix or ".mp3"
        data = audio.file.read(MAX_BYTES + 1)
        if len(data) > MAX_BYTES:
            raise HTTPException(status_code=413, detail="audio is larger than 10MB")
        if not data:
            raise HTTPException(status_code=400, detail="audio is empty")

        fd, name = tempfile.mkstemp(prefix="chordd-", suffix=suffix)
        path = Path(name)
        with os.fdopen(fd, "wb") as f:
            f.write(data)

        digest = sha256_file(path)
        job = jobs.create()
        cached = app_cache.get(digest)
        if cached is not None:
            job.result = cached
            job.progress = 1.0
            job.status = "done"
            cleanup(path)
            return JSONResponse(
                status_code=202,
                content=AnalyzeAccepted(jobId=job.id, etaMs=0).model_dump(),
            )

        def work(progress):
            try:
                return app_analyzer.analyze(path, duration, progress)
            finally:
                cleanup(path)

        jobs.run(job, work, on_done=lambda result: app_cache.put(digest, result))
        return JSONResponse(
            status_code=202,
            content=AnalyzeAccepted(jobId=job.id, etaMs=45000).model_dump(),
        )

    @app.get("/v1/jobs/{job_id}", response_model=JobStatus)
    def job_status(job_id: str) -> JobStatus:
        job = jobs.get(job_id)
        if job is None:
            raise HTTPException(status_code=404, detail="job not found")
        return job.snapshot()

    @app.get("/v1/jobs/{job_id}/result", response_model=AnalyzeResult)
    def job_result(job_id: str) -> AnalyzeResult:
        job = jobs.get(job_id)
        if job is None:
            raise HTTPException(status_code=404, detail="job not found")
        status = job.snapshot()
        if status.status == "failed":
            raise HTTPException(status_code=500, detail=status.error or "analysis failed")
        if status.status != "done" or job.result is None:
            raise HTTPException(status_code=409, detail="analysis is not done")
        return job.result

    return app


app = create_app()
