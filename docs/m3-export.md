# M3 视频导出

## 流程

```text
project.json + 源视频
  → ffmpeg 顺序解码为 YUV420P 原始帧（scale/crop/fps=30）
  → ui.Bitmap.UpdateYUV 上传 I420 平面（GPU shader 转 RGB）
  → MyGo plugins/render Composition 在 Metal 上合成视频 + 和弦/指法/字幕
  → Metal 直接写入 CVPixelBuffer 的 IOSurface，AVFoundation 硬件编码 H.264
  → ffmpeg 复用源视频音轨：AAC 192k + faststart
```

预览和成片共用 `app/internal/render` 的 `Backdrop` / `Overlay`，
没有 ASS/libass 第二套渲染器。

## GPU 合成

- macOS 默认 `render.Auto`：有 Metal 就用 Metal，没有则回退 MyGo CPU
  渲染器；编码走系统 AVFoundation，H.264 由 VideoToolbox 完成。
- 源视频帧不再逐帧转 RGBA。ffmpeg 输出 `yuv420p`，`UpdateYUV` 直接把
  Y/Cb/Cr 平面交给 GPU，带宽约为 RGBA 的 3/8。
- CPU 回退时反过来让 ffmpeg 输出 RGBA，避免 CPU 渲染器每帧再做一次
  YUV→RGB 转换。
- 解码帧用 4 个缓冲轮转。MyGo 绘制第 i 帧时第 i+1 帧正在构建，
  不能在 GPU 读完前覆盖同一块像素内存。
- Metal 合成结果直接写入编码器的 IOSurface，不经过 CPU 读回；
  只有音轨由 ffmpeg 在封装阶段拷贝，视频流是 `-c:v copy`。

## 规格

| 项 | 竖屏 | 横屏 |
|---|---|---|
| 分辨率 | 1080×1920 | 1920×1080 |
| 视频 | H.264 · Metal + VideoToolbox | 同左 |
| 画质 | 小体积 4 / 标准 9 / 原画质 12.4 Mbps | 同左 |
| 音频 | 源视频音轨 → AAC 96/160/192k | 同左 |
| 帧率 | 30 | 30 |

竖屏叠加层：当前小节高亮、后续 1–2 个和弦、右侧尤克里里指法图；
字幕默认放在 78% 高度，可在样式页按 60–80% 调整，避开底部 15%
平台文案区和右侧 120px 操作栏。

横屏叠加层：9:16 人像居中 606×1080（4:2:0 取偶），左侧当前和弦/调性/指法图，
右侧按当前播放头显示逐句曲谱。

## 实测

macOS Intel，Intel Iris Plus Graphics 645，源视频 1080×1920：

| 源视频 | 耗时 | 分段 | 输出 |
|---|---|---|---|
| 6 秒 | 4.2 秒 | 渲染 3.9s + 封装 0.3s | 9.7MB |
| 36.2 秒 | 19.1 秒 | 渲染 17.9s + 封装 1.2s | 57.6MB（原画质 12.4 Mbps） |
| 6 秒 · CPU 回退 | 16.4 秒 | 渲染 15.9s + 封装 0.5s | 9.3MB |

36.2 秒成片约 1.9× 实时，3 分钟视频预计约 1.5–2 分钟，远低于
8 分钟预算。进度回调里的渲染器名称为
`Metal (Intel(R) Iris(TM) Plus Graphics 645)`。

导出默认使用“原画质”档 12.4 Mbps / AAC 192k，与引入画质档之前一致
（渲染器默认 `0.2 bit/pixel/frame`，1080×1920@30 约 12.4 Mbps）。同样的
70 秒竖屏视频，原画质约 110MB，标准档约 80MB，小体积档约 36MB。

旧路径对照（同一台机器、同一段 36.2 秒视频）：

| 路径 | 耗时 |
|---|---|
| CPU `ui.NewTester` + libx264 | 约 111 秒 |
| CPU 叠加 + VideoToolbox 编码 | 45.1 秒 |
| Metal 合成 + VideoToolbox 编码 | 19.1 秒 |

## 使用

界面：`导出成片 → 开始导出 → 选择保存位置`。
字幕和曲谱默认逐帧烧进 MP4；同名 `.srt` 是可选附加文件，默认关闭，
可在导出弹窗里打开。工程没有逐句字幕时不会生成 SRT。
导出中显示按进度估算的实时剩余时间，可以点「取消导出」终止并清理
未完成的 mp4。

强制路径做对照：

```sh
MYGO_UKE_ENCODER=cpu    # 软件编码，合成仍优先 Metal
MYGO_UKE_RENDERER=cpu   # 强制 CPU 合成（很慢，只用于排查）
MYGO_UKE_RENDERER=gpu   # 强制 Metal，失败不自动回退
MYGO_UKE_EXPORT_DEBUG=1 # 打印渲染器、分段耗时
```

命令行：

```sh
cd app
go run ./cmd/export -project /path/project.json -source /path/song.mp4 \
  -out /path/out.mp4 -orientation 0 -quality original -srt
```

真实编码集成测试：

```sh
MYGO_UKE_EXPORT_SOURCE=/path/source.mp4 \
MYGO_UKE_EXPORT_OUT=/tmp/out.mp4 \
go test ./internal/export -run TestExportIntegration -v -count=1
```

## 已知限制

- 未配火山 Key 或转写失败时 `lines` 为空，竖屏只显示和弦条/指法图，
  横屏右侧退化为小节和弦表。
- 使用源视频原音轨，尚未做人声分离或伴奏混音。
- 关闭弹窗后任务仍在后台执行；关闭整个应用会取消导出。
