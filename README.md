# mygo-uke

一条自录尤克里里弹唱视频进去，出来一条带字幕和曲谱的成片。

- `app/`：MyGo v0.10 原生桌面客户端，零模型。
- `chordd/`：FastAPI 和弦/节拍分析服务，uv 管理，独立部署。
- `designs/`：已经确认的 A · 录音棚界面设计稿。
- `docs/`：需求与设计文档。

## 开发

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
cd chordd && uv sync --frozen --no-dev
rdev --no-sync runpulse restart chordd
```

## 当前阶段

M1/M2 的骨架：录音棚界面、工程模型、小节生成、尤克里里指法表，以及可独立启动的
`chordd` 接口。真实 ASR、BTC/beat_this 推理和 ffmpeg 导出在后续里程碑接入。
