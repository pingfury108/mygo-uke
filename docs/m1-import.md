# M1 导入管线

## 流程

```text
选择/拖入视频
  → ffprobe 读时长
  → ffmpeg 抽 22.05kHz mono 128k mp3
  → POST chordd /v1/analyze
  → 轮询 job，拿 chords / beats / downbeats / tempo / key
  → 生成 bars
  → 保存 project.json
```

没有源文件时，界面仍保留三个示例工程用于设计校对。

## 代码位置

- `app/internal/media`：ffmpeg / ffprobe 包装
- `app/internal/chordd`：上传、轮询、取结果
- `app/internal/pipeline`：抽音轨 → 分析 → 生成工程
- `app/internal/project`：`project.json`、原子保存、本地工程库
- `app/internal/app/import.go`：文件选择、拖放、后台任务和 UI 状态

## 环境变量

```text
MYGO_UKE_CHORDD=http://host:8790    chordd 地址
MYGO_UKE_PROJECTS=/path/to/library 工程库目录
MYGO_UKE_FFMPEG=/path/to/ffmpeg     ffmpeg 路径
MYGO_UKE_FFPROBE=/path/to/ffprobe   ffprobe 路径
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

## 已知限制

- ASR 尚未接入，导入后的 `lines` 为空，需要后续补火山 ASR 或手动填词。
- 视频画面尚未接入预览/导出，当前只使用音频分析与波形。
- 重新分析会覆盖当前工程的分析结果，尚未做“保留人工修改”的合并策略。
