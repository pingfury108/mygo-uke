# M0 验证记录

> 2026-11-22，pcw（8 核 x86_64，7.6GB RAM，CPU 推理）。

## V2 · 火山 ASR

未开始。需要可用的火山 API Key，以及 30 秒歌声样本。

## V3 · 离屏渲染性能

已补 `app/cmd/bench`，macOS Apple Silicon：

| 场景 | 1080×1920 平均单帧 | 200 帧总计 |
|---|---|---|
| 导出帧（视频底图 + 和弦 + 字幕） | 14.1ms | 2.8s |
| 整套三栏编辑器 UI（保守上界） | 43.1ms | 8.6s |

**结论**：真实导出路径 14ms，满足 ≤40ms；3 分钟视频理论渲染约 76 秒。
整套编辑器 UI 超过 40ms 但不用于逐帧导出，且仍低于 80ms 降级线。

复现：

```sh
cd app
go run ./cmd/bench -mode export -n 200 -width 1080 -height 1920
go run ./cmd/bench -mode ui -n 200 -width 1080 -height 1920
```
