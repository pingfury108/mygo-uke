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

真实分析器在 `chordd/real.py`：

- `beat_this` 出 `tempo / beats / downbeats`
- BTC（ChordMini 微调权重，2026）出 `chords`，50% 重叠滑窗投票
- 调性用和弦序列 + Krumhansl 模板

可调参数：

```text
CHORDD_ANALYZER=beat-this      启用真实分析器
CHORDD_TORCH_THREADS=4         torch 线程数
CHORDD_BTC_BATCH=8             BTC 分块批大小
CHORDD_BTC_OVERLAP=0.5         相邻 108 帧窗口的重叠比例
CHORDD_DEVICE=cpu              torch 设备
CHORDD_MODEL_DIR=...           模型缓存目录，默认 ~/.cache/chordd/models
CHORDD_BTC_WEIGHTS=...         直接指定 BTC 权重
CHORDD_BEAT_MODEL=final0       beat_this checkpoint 名称或路径
```

单独分析一个文件并生成波形/downbeat 叠加图：

```sh
uv run --extra analysis python scripts/analyze_file.py song.mp3 \
  --json analysis/song.json --svg analysis/song.svg
```

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
cd chordd && uv sync --frozen --extra analysis --no-dev
rdev --no-sync .venv/bin/python -m chordd.model_store
rdev --no-sync runpulse start /data/rdev/mygo-uke/chordd/.venv/bin/uvicorn \
  --name chordd --cwd /data/rdev/mygo-uke/chordd \
  --env CHORDD_ANALYZER=beat-this --env CHORDD_TORCH_THREADS=4 \
  --env CHORDD_BTC_BATCH=8 --env PYTHONUNBUFFERED=1 \
  -- chordd.main:app --host 0.0.0.0 --port 8790
rdev --no-sync curl -s http://127.0.0.1:8790/healthz
```
