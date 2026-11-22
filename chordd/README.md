# chordd

`mygo-uke` 的和弦/节拍分析服务。Python 只活在这里，不进入桌面分发物。

## 启动

```sh
uv sync
uv run uvicorn chordd.main:app --reload --port 8790
```

默认使用确定性的 mock 分析器，方便先把客户端链路跑通。接真实模型：

```sh
uv sync --extra analysis
CHORDD_ANALYZER=beat-this uv run uvicorn chordd.main:app --port 8790
```

真实分析器入口在 `chordd/analyze.py`：

- `beat_this` 出 `tempo / beats / downbeats`
- BTC large-voca 出 `chords`
- 调性用和弦序列 + Krumhansl 模板

## 接口

```text
POST /v1/analyze            multipart: audio=<file>, duration=<seconds?>
GET  /v1/jobs/{jobId}
GET  /v1/jobs/{jobId}/result
GET  /healthz
```

时间单位统一为秒（float）。`downbeats` 是产品的命根子，即使降级也必须返回。

## 部署

服务部署在 pcw，用 runpulse 常驻，端口 `8790`（同机 `8787` 已被占用）。
客户端默认访问 `http://MiWiFi-R1350-srv.local:8790`，也可用
`MYGO_UKE_CHORDD` 覆盖。

```sh
rdev server use pcw
cd chordd && uv sync --frozen --no-dev
rdev --no-sync runpulse restart chordd
rdev --no-sync curl -s http://127.0.0.1:8790/healthz
```
