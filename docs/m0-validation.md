# M0 验证记录

> 2026-11-22，pcw（8 核 x86_64，7.6GB RAM，CPU 推理）。

## V1 · 小节边界与和弦

已跑通真实 `beat_this + BTC large-voca`，HTTP 和离线脚本结果一致。

样本：BTC 官方 `test/example.mp3`，253.6 秒。

| 项 | 结果 |
|---|---|
| tempo | 125.0 BPM |
| time signature | 3/4 |
| key | F# |
| beats | 548 |
| downbeats | 174 |
| chords | 210 |
| 热推理耗时 | 30.6 秒 |
| 常驻 RSS | 约 752MB |

downbeat 间隔稳定在 1.44 秒；按 125 BPM 折算为 3 拍，因此 3/4 判断成立。
波形与 downbeat 叠加图见本地 `analysis/warm.svg`（不提交）。

**结论**：技术链路通过，CPU 延迟满足“3 分钟歌 ≤ 90 秒”。

**尚未完成**：用自己弹唱的 3 首歌做人工和弦准确率核对；当前还没有 ground truth
和弦谱，因此还不能给出 ≥80% 的结论。

## V2 · 火山 ASR

未开始。需要可用的火山 API Key，以及 30 秒歌声样本。

## V3 · 离屏渲染性能

未开始。下一步补 `cmd/bench`，1080×1920 跑 100 帧，确认单帧 ≤ 40ms。

## 复现

```sh
cd chordd
uv sync --extra analysis
uv run --extra analysis python scripts/analyze_file.py song.mp3 \
  --json analysis/song.json --svg analysis/song.svg
```

首次运行下载：

- BTC large-voca：12MB，缓存在 `~/.cache/chordd/models`
- beat_this final0：78MB，缓存在 `~/.cache/torch`
