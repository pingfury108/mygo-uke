# M3 视频导出

## 流程

```text
project.json + 源视频
  → ffmpeg 解码为 RGBA 原始帧（scale/crop/fps=30）
  → MyGo Tester 逐帧叠加当前和弦 + 当前字幕
  → rawvideo 管道喂给 ffmpeg
  → libx264 CRF18 + AAC 192k + faststart
```

预览和成片共用 `app/internal/render`，没有 ASS/libass 第二套渲染器。

## 规格

| 项 | 竖屏 | 横屏 |
|---|---|---|
| 分辨率 | 1080×1920 | 1920×1080 |
| 视频 | H.264 · CRF18 · yuv420p | 同左 |
| 音频 | 源视频音轨 → AAC 192k | 同左 |
| 帧率 | 30 | 30 |

## 实测

macOS Apple Silicon，合成 6 秒 720p 源视频：

| 方向 | 耗时 | 输出 |
|---|---|---|
| 竖屏 | 10.8 秒 | 1080×1920，H.264 + AAC |
| 横屏 | 5.7 秒 | 1920×1080，H.264 + AAC |

按竖屏 1.8× 实时估算，3 分钟视频约 5.4 分钟，满足 ≤8 分钟。

## 使用

界面：`导出成片 → 开始导出 → 选择保存位置`。

命令行：

```sh
cd app
go run ./cmd/export -project /path/project.json -source /path/song.mp4 \
  -out /path/out.mp4 -orientation 0
```

真实编码集成测试：

```sh
MYGO_UKE_EXPORT_SOURCE=/path/source.mp4 \
MYGO_UKE_EXPORT_OUT=/tmp/out.mp4 \
go test ./internal/export -run TestExportIntegration -v -count=1
```

## 已知限制

- 当前叠加的是单个和弦标签 + 单行字幕，尚未画完整逐句曲谱/指法图。
- 使用源视频原音轨，尚未做人声分离或伴奏混音。
- 安全区、样式参数尚未传入导出渲染。
- 导出任务没有取消按钮；关闭弹窗后任务仍在后台执行。
