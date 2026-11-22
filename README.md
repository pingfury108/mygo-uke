# mygo-uke

一条自录尤克里里弹唱视频进去，出来一条带字幕和曲谱的成片。

- `app/`：MyGo v0.10 原生桌面客户端，零模型。
- `chordd/`：FastAPI 和弦/节拍分析服务，uv 管理，独立部署。
- `designs/`：已经确认的 A · 录音棚界面设计稿。
- `docs/`：需求与设计文档。

## 开发

导入功能需要本机有 `ffmpeg` / `ffprobe`：

```sh
ffmpeg -version
```

```sh
cd app
go mod tidy
go tool mygo dev
```

和弦服务：

```sh
cd chordd
uv sync --dev
uv run uvicorn chordd.main:app --reload --port 8790
```

真实分析器（beat_this + BTC）：

```sh
cd chordd
uv sync --extra analysis
CHORDD_ANALYZER=beat-this uv run uvicorn chordd.main:app --port 8790
```

首次分析会自动下载 12MB BTC 权重和 78MB beat_this 权重，缓存在
`~/.cache/chordd/models` 与 `~/.cache/torch`。

客户端默认连接 pcw 上的 `chordd`：

```text
http://MiWiFi-R1350-srv.local:8790
```

换机器或换端口时用环境变量覆盖：

```sh
MYGO_UKE_CHORDD=http://127.0.0.1:8790 go tool mygo dev
```

pcw 上由 runpulse 常驻，8787 已被同机其他服务占用，因此固定用 8790：

```sh
rdev server use pcw
cd chordd && uv sync --frozen --extra analysis --no-dev
rdev --no-sync runpulse restart chordd
```

## 当前阶段

M1/M2/M3：录音棚界面、工程模型、小节生成、尤克里里指法表、真实导入管线
（视频 → ffmpeg 抽音轨 → chordd → `project.json`），以及竖屏/横屏 ffmpeg
成片导出。竖屏和弦条/指法图与横屏左右谱面已接入导出；真实
`beat_this + BTC` 分析器已部署在 pcw。导入时若设置
`MYGO_UKE_VOLCANO_API_KEY`，会并行跑火山流式 ASR 生成逐句字幕。

- M0 实测记录：`docs/m0-validation.md`
- M1 导入管线：`docs/m1-import.md`
- M3 导出管线：`docs/m3-export.md`

工程库默认在 `~/Library/Application Support/mygo-uke/projects`，可用
`MYGO_UKE_PROJECTS` 覆盖。
