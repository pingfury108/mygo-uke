# M3 视频导出

## 流程

```text
project.json + 源视频
  → ffmpeg 解码为 RGBA 原始帧（scale/crop/fps=30）
  → MyGo Tester 逐帧叠加和弦条 + 指法图 + 当前字幕
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

竖屏叠加层：当前小节高亮、后续 1–2 个和弦、右侧尤克里里指法图；
字幕放在 70% 高度，避开底部 15% 平台文案区和右侧 120px 操作栏。

横屏叠加层：9:16 人像居中 607×1080，左侧当前和弦/调性/指法图，
右侧按当前播放头显示逐句曲谱。

## 实测

macOS Apple Silicon，合成 6 秒 720p 源视频：

| 方向 | 耗时 | 输出 |
|---|---|---|
| 竖屏 | 12.1 秒 | 1080×1920，H.264 + AAC |
| 横屏 | 5.8 秒 | 1920×1080，H.264 + AAC |

完整叠加层单帧：竖屏 23.9ms、横屏 14.2ms，均低于 40ms 预算。
按竖屏约 2× 实时估算，3 分钟视频约 6 分钟，满足 ≤8 分钟。

## 使用

界面：`导出成片 → 开始导出 → 选择保存位置`。
默认同时导出同名 `.srt`，可在导出弹窗里关掉；工程没有逐句字幕时跳过。

命令行：

```sh
cd app
go run ./cmd/export -project /path/project.json -source /path/song.mp4 \
  -out /path/out.mp4 -orientation 0 -srt
```

真实编码集成测试：

```sh
MYGO_UKE_EXPORT_SOURCE=/path/source.mp4 \
MYGO_UKE_EXPORT_OUT=/tmp/out.mp4 \
go test ./internal/export -run TestExportIntegration -v -count=1
```

## 已知限制

- 导入工程尚未接 ASR，`lines` 为空时竖屏只显示和弦条/指法图，
  横屏右侧退化为小节和弦表。
- 使用源视频原音轨，尚未做人声分离或伴奏混音。
- 导出任务没有取消按钮；关闭弹窗后任务仍在后台执行。
