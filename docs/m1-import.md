# M1 导入管线

## 流程

```text
选择/拖入视频
  → ffprobe 读时长
  → ffmpeg 抽 22.05kHz mono 128k mp3
  → 配了火山 Key 时再抽 16kHz mono s16le PCM
  → 并行：chordd 分析和弦 ｜ 火山流式 ASR 转写句级字幕
  → 轮询 job，拿 chords / beats / downbeats / tempo / key
  → 生成 bars + lines
  → 保存 project.json
```

没有源文件时，界面仍保留三个示例工程用于设计校对。

## 代码位置

- `app/internal/media`：ffmpeg / ffprobe 包装
- `app/internal/chordd`：上传、轮询、取结果
- `app/internal/asr`：火山双向流式 ASR（复用 `sotto`，补 utterances 解析）
- `app/internal/pipeline`：抽音轨 → 分析 → 生成工程
- `app/internal/project`：`project.json`、原子保存、本地工程库
- `app/internal/app/import.go`：文件选择、拖放、后台任务和 UI 状态

## 环境变量

```text
MYGO_UKE_CHORDD=http://host:8790    chordd 地址
MYGO_UKE_PROJECTS=/path/to/library 工程库目录
MYGO_UKE_FFMPEG=/path/to/ffmpeg     ffmpeg 路径
MYGO_UKE_FFPROBE=/path/to/ffprobe   ffprobe 路径
MYGO_UKE_VOLCANO_API_KEY=...        火山 ASR Key；不配则只分析和弦
MYGO_UKE_VOLCANO_RESOURCE_ID=...    资源 ID，默认 2.0 小时版
MYGO_UKE_VOLCANO_WS_URL=...         WebSocket 端点，默认官方 v3
MYGO_UKE_VOLCANO_MODEL=bigmodel     模型名
```

## 测试

普通单测用 fake analyzer / fake media，不启动 ffmpeg 和网络：

```sh
cd app
go test ./...
```

真实链路集成测试需要本机 ffmpeg 和可访问的 chordd：

```sh
MYGO_UKE_SOURCE=/path/song.mp3 go test ./internal/pipeline \
  -run TestImportIntegration -v -count=1
```

2026-11-22 实测：257 秒音频、pcw 真实分析器（缓存命中），完整导入 17.0 秒，
生成 211 个和弦段、174 个小节。

## ASR 行为

- 未配置 Key：跳过转写，导入照常完成，`lines` 为空。
- 配置 Key：ASR 与 chordd 并行；转写失败不会丢和弦，失败原因写入
  `project.json` 的 `warnings`，界面状态显示「完成 · 字幕未识别」。
- 只接受服务端返回的句级 `result.utterances[]`；没有句子时间不会伪造成整首一条。

## 已知限制

- 尚未做 ASR Key 的图形化设置页，当前用环境变量。
- 视频画面尚未接入预览/导出，当前只使用音频分析与波形。
- 重新分析会覆盖当前工程的分析结果，尚未做“保留人工修改”的合并策略。
