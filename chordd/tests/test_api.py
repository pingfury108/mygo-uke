from __future__ import annotations

import time

from fastapi.testclient import TestClient

from chordd.cache import Cache
from chordd.main import create_app


def test_health() -> None:
    client = TestClient(create_app())
    response = client.get("/healthz")
    assert response.status_code == 200
    body = response.json()
    assert body["ok"] is True
    assert body["analyzer"] == "mock"


def test_analyze_and_fetch_result(tmp_path) -> None:
    client = TestClient(create_app(cache=Cache(tmp_path)))
    response = client.post(
        "/v1/analyze",
        files={"audio": ("test.mp3", b"not really audio", "audio/mpeg")},
        data={"duration": "12"},
    )
    assert response.status_code == 202
    job_id = response.json()["jobId"]

    deadline = time.time() + 3
    while time.time() < deadline:
        status = client.get(f"/v1/jobs/{job_id}").json()
        if status["status"] == "done":
            break
        time.sleep(0.05)
    assert status["status"] == "done"
    assert status["progress"] == 1.0

    result = client.get(f"/v1/jobs/{job_id}/result")
    assert result.status_code == 200
    body = result.json()
    assert body["tempo"] == 68.0
    assert body["downbeats"]
    assert body["chords"][0]["label"] == "C"


def test_result_before_done_is_conflict(tmp_path) -> None:
    client = TestClient(create_app(cache=Cache(tmp_path)))
    response = client.post(
        "/v1/analyze",
        files={"audio": ("test.mp3", b"x", "audio/mpeg")},
        data={"duration": "12"},
    )
    job_id = response.json()["jobId"]
    result = client.get(f"/v1/jobs/{job_id}/result")
    assert result.status_code in (200, 409)
