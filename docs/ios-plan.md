# MyGo Uke iOS 实现方案

## 1. 结论

可以做一个完全在 iPad 上运行的版本，不需要自建渲染服务器。

- 编辑、预览、字幕/和弦、本地导出：全部在 iPad 完成。
- 视频解码、编码、音轨合成：改用 AVFoundation，不执行 ffmpeg 进程。
- 现有 ASR 和曲谱图片识别仍然是云 API；如果要求完全离线，需要另外接本地模型，不在本方案的首版范围内。
- pwc 渲染服务只作为可选加速/兜底，不作为 iOS 的必需依赖。

## 2. 当前实现里不能直接上 iOS 的部分

| 位置 | 问题 |
|---|---|
| `app/internal/media/media.go` | 用 ffmpeg 取时长、抽 16k PCM |
| `app/internal/export/feed.go` | 用 ffmpeg 解码、缩放、裁切、补 30fps |
| `app/internal/export/export.go` | 用 ffmpeg 封装音轨 |
| `mygo/plugins/render` | iOS 没有系统编码器；GPU 离屏渲染也被 build tag 排除 |
| `app/internal/app/app.go` | 1440×900、最小 1100 宽的三栏桌面布局 |
| `app/internal/app/app.go` | 依赖文件拖拽、Cmd 快捷键、桌面 title bar |
| `app/internal/project/project.go` | `Source` 是绝对路径，iOS 文件选择器给的是临时副本 |
| `app/internal/app/settings.go` | API Key 明文写 JSON，iOS 上应放 Keychain |

iOS 沙箱不允许 `fork/exec`，所以不能把现有 ffmpeg 可执行文件塞进 app 里运行；App Store 也不接受这种做法。

## 3. 总体架构

```text
Files / 照片选择器
        │
        ▼
复制到工程目录 media/source.<ext>
        │
        ├── AVAssetReader ──► BGRA 帧 ──► render.Overlay ──► AVAssetWriter
        │                                                       │
        └── AVAssetReader ──► 16k PCM ──► 云 ASR                │
                                                                 ▼
                                            AVMutableComposition + AVAssetExportSession
                                                                 │
                                                                 ▼
                                                          MP4 + 可选 SRT
```

桌面端保持不变：

- 桌面继续走 ffmpeg。
- iOS 走 AVFoundation。
- 预览和导出继续共用 `render.Overlay`，保证“预览是什么样，导出就是什么样”。

## 4. 平台抽象

### 4.1 媒体能力

`pipeline.Media` 已经是接口，不需要改调用方：

```go
type Media interface {
    Duration(ctx context.Context, path string) (float64, error)
    ExtractPCM(ctx context.Context, source, destination string) error
}
```

建议拆成：

```text
app/internal/media/
  media.go              // 共享类型和接口
  ffmpeg.go             // //go:build !ios
  avf_ios.go            // //go:build ios
  avf_ios.m             // AVFoundation 的 Objective-C 实现
```

选择实现：

```text
app/internal/app/platform_default.go   // //go:build !ios
app/internal/app/platform_ios.go       // //go:build ios
```

桌面返回 `pipeline.FFmpeg{}`，iOS 返回 `pipeline.NativeMedia{}`。

### 4.2 导出实现

`internal/export` 拆成共享部分和平台部分：

```text
app/internal/export/
  export.go          // 共享 Run 签名、layout、composition、SRT
  run_process.go     // //go:build !ios，现有 ffmpeg 实现
  run_ios.go         // //go:build ios，AVFoundation 实现
  feed_ios.go        // AVAssetReader 帧源
  writer_ios.go      // AVAssetWriter 编码
  mux_ios.go         // 音轨合成
```

`export.Run` 的签名保持不变，app 层不需要区分平台。

## 5. AVFoundation 桥接

### 5.1 技术选择

推荐用 **iOS 专用的 cgo + Objective-C 桥接**，放在 `internal/avf` 或 `internal/media` 下：

- 每帧都要把 CVPixelBuffer 交给 Go 渲染，跨语言调用量大，cgo 比 JSON native module 更直接。
- iOS 构建本来就需要 cgo 和 Xcode clang。
- 所有文件加 `//go:build ios`，桌面构建完全不受影响。

如果团队坚持不用 cgo，也可以用 `purego/objc`，但代码量和调试成本更高，建议先按 cgo 方案落地。

### 5.2 需要暴露的 C 接口

```c
double mygo_avf_duration(const char *path, char **err);
int    mygo_avf_extract_pcm(const char *src, const char *dst, char **err);

void  *mygo_avf_reader_open(const char *path, int w, int h, int fps, char **err);
int    mygo_avf_reader_next(void *reader, void **pixels, int *stride, char **err);
void   mygo_avf_reader_close(void *reader);

void  *mygo_avf_writer_open(const char *path, int w, int h, int fps, char **err);
int    mygo_avf_writer_append(void *writer, const void *pixels, int stride,
                              int index, char **err);
int    mygo_avf_writer_close(void *writer, char **err);
void   mygo_avf_writer_abort(void *writer);

int    mygo_avf_mux(const char *video, const char *source,
                    const char *output, char **err);
```

推荐使用 `AVAssetReaderVideoCompositionOutput`：

- 由系统处理源视频的 `preferredTransform`。
- `renderSize` 设为目标 1080×1920 或 1920×1080。
- `frameDuration` 设为 1/30。
- 变换按“等比放大到铺满，再居中裁切”计算，和现在 ffmpeg 的
  `scale=...:force_original_aspect_ratio=increase,crop=...` 一致。

### 5.3 帧数据

- 像素格式用 `kCVPixelFormatType_32BGRA`。
- 每次 `mygo_avf_reader_next` 返回的指针只在下次调用前有效，Go 侧立即复制。
- 复制后转成 `image.RGBA`，再交给 `ui.Bitmap`。
- 每帧放在 `@autoreleasepool` 里，避免长视频内存持续上涨。

## 6. 导入链路

### 6.1 文件入口

- 从“文件”选择视频/曲谱：`mygo.Dialog.Open`。
- 从照片库选择视频/曲谱：`plugins/imagepicker` 的 `Pick`。
- 不再依赖文件拖拽；iPad 上保留也可以，但只能是补充入口。

### 6.2 源视频归档

当前 `Project.Source` 存绝对路径，iOS 上文件选择器返回的是临时副本，必须改：

1. 导入时先创建工程目录。
2. 把视频复制到 `<projectDir>/media/source.<ext>`。
3. `Project.Source` 存相对路径，例如 `media/source.mp4`。
4. 新增 `project.ResolveSource(projectPath, source)`：
   - 绝对路径：原样返回；
   - 相对路径：相对 `project.json` 所在目录解析。
5. 桌面端继续允许绝对路径，不影响老工程。

这样工程目录是自包含的，删除工程时可以一起清理。

### 6.3 时长和 PCM

- `Duration`：`AVURLAsset.duration`，转成秒。
- `ExtractPCM`：`AVAssetReader` 输出 Linear PCM：
  - 16 kHz
  - 单声道
  - 16-bit signed little-endian
  - 与现有火山 ASR 输入格式一致

### 6.4 曲谱图片

- `imagepicker` 返回临时文件。
- 读取后写入 `<projectDir>/chart.png`，沿用现有 `storeChartFor`。
- 上传识别前做两件事：
  - 按 EXIF 方向转正；
  - 最长边限制在 2000px 左右，减少上传体积。
- 原图仍保存在工程目录，识别失败也不丢。

## 7. 本地导出链路

### 7.1 方案 A：CPU 渲染 + 硬件编码（首选 MVP）

复用现有 `renderplugin.Frames` 的 CPU 路径：

1. `AVAssetReader` 按 30fps 输出 BGRA 帧。
2. 把帧交给 `render.Backdrop` / `render.Overlay`。
3. `renderplugin.Frames` 每帧返回 `image.RGBA`。
4. `AVAssetWriter` + `AVAssetWriterInputPixelBufferAdaptor` 用
   VideoToolbox 硬件编码 H.264。
5. 渲染视频先写到临时文件（无音轨）。
6. `AVMutableComposition` 插入渲染视频轨和源视频音轨。
7. `AVAssetExportSession` 输出最终 MP4。
8. SRT 继续用现有 `export.WriteSRT`。

优点：

- 不改 mygo。
- 功能先跑通，风险最低。
- 编码是硬件加速，主要耗时在 UI 栅格化。

缺点：

- 1080×1920 的 CPU 栅格化在 iPad 上可能只有十几帧每秒，1 分钟视频可能要几分钟。

### 7.2 方案 B：Metal 离屏渲染 + 硬件编码（性能版）

mygo 的 `internal/gpu/metal` 本身是按 `darwin` 编译的，iOS 也满足这个 tag；
只是 `plugins/render/gpu_darwin.go` 和 `sink_darwin.go` 用了 `darwin && !ios`。

建议做一次 spike：

1. 在 mygo fork 中把这两个文件的 build tag 放开到 iOS。
2. 用 `renderplugin.Frames` + `Renderer: GPU` 在 iPad 上渲染几帧。
3. 如果有问题，再针对 iOS 的 Metal/AVAssetWriter 差异修。

如果 spike 通过：

- 预览和导出都走 Metal；
- 导出速度接近实时；
- 可以给 mygo 提 PR，避免长期维护 fork。

如果 spike 不通过：

- 首版用 CPU 渲染；
- 增加“720×1280 快速导出”选项；
- 或保留 pwc 作为可选加速，但不作为默认路径。

### 7.3 音轨合成

MVP 用两段式：

```text
渲染视频（无音轨） ──┐
                    ├── AVMutableComposition ── AVAssetExportSession ── 最终 MP4
源视频音轨 ─────────┘
```

优先用 `AVAssetExportPresetPassthrough`，源音轨是 AAC 时可以直接复用；
源音轨格式不兼容时回退到 `AVAssetExportPresetHighestQuality` 重新编码。

后续优化可以改成单次 `AVAssetWriter` 同时写视频和 AAC 音轨，省掉二次封装。

### 7.4 导出文件位置

iOS 上没有桌面的“另存为”路径：

- 先导出到 app 的 `Documents/Exports/<name>.mp4`；
- 完成后用 `mygo.Share` 或 `mygo.Dialog.SaveCopy` 让用户存到“文件”、相册或分享出去；
- SRT 作为第二个文件一起分享。

### 7.5 进度与取消

- 解码/渲染阶段按帧上报 0–90%。
- 音轨合成阶段上报 90–100%。
- 用户取消时：
  - 停止 reader；
  - abort writer；
  - 删除临时 MP4；
  - 不留下半个文件。

### 7.6 导出期间保持屏幕常亮

```go
release := mygo.Power.KeepAwake("导出成片", true)
defer release()
```

如果 app 进入后台，iOS 可能挂起进程。首版策略：

- 导出期间提示“请保持 app 在前台”；
- 进入后台时取消导出并清理临时文件；
- 不做后台断点续传。

后续需要后台导出时，再接 `plugins/background` 的 BGProcessingTask。

## 8. iPad / iPhone 界面

### 8.1 断点

不按设备型号判断，按窗口宽度：

- `width >= 1000`：宽屏，复用三栏。
- `width < 1000`：单列 + 底部四栏（编辑 / 歌词 / 和弦 / 工程）。

首版先分这两档；`700 <= width < 1000` 的紧凑横屏后续再单独优化。

iPad 分屏、Stage Manager、旋转都要能实时响应。

### 8.2 iPad 横屏

尽量复用现有布局：

- 左侧工程栏 234pt；
- 中间预览 + 播放控制；
- 右侧字幕/小节/样式检查器；
- 顶部工具栏去掉 macOS 红绿灯和 `DragWindow` 依赖，改为普通工具条；
- 所有 hover 才出现的操作，增加常驻按钮或长按入口。

### 8.3 iPad 竖屏 / iPhone

改成单列 + 底部导航：

- 编辑：预览、播放、时间轴、当前句快捷编辑；
- 歌词：逐句列表，点句子跳转视频，点编辑打开 Sheet；
- 和弦：小节列表和样式；
- 工程：工程列表、导入、导出、设置。

细节：

- 使用 `ui/mobile.BottomBar` 或自定义底部栏，底部按 `c.SafeArea()` 预留 Home Indicator。
- 检查器编辑用 `ui/mobile.Sheet`，不要再弹 460/520pt 的桌面 Modal。
- 输入框、按钮最小点击区域 44pt。
- 视频预览点击切换播放/暂停；时间轴支持拖动。
- 顶部只保留：工程名、切换工程、导出、更多。

### 8.4 桌面 Modal 的适配

现有 `ui.Modal` 固定宽度 460/520，手机上会超出屏幕。需要加一个统一 helper：

- 宽度 = `min(设计宽度, 窗口宽度 - 24)`；
- 内容放进 `ui.Scroll`；
- 键盘弹起时保证输入框可见；
- 在手机上优先用 bottom sheet。

涉及：

- 导出
- 设置
- 新建工程
- 曲谱导入
- 逐句编辑
- 和弦编辑
- 工程设置

## 9. 工程存储与设置

### 9.1 工程目录

iOS 建议用：

```text
Documents/
  projects/
    <slug>-<timestamp>/
      project.json
      chart.png
      media/source.mp4
```

图标和 `Info.plist` 打开文件共享后，用户可以在“文件”app 里看到工程目录。

桌面的 `project.Root()` 行为保持不变。

### 9.2 API Key

- 非敏感设置继续存 JSON。
- 火山 API Key、曲谱 API Key 改存 `mygo.Secrets`（iOS 走 Keychain）。
- 设置页不再显示内部文件路径；iOS 上隐藏环境变量相关文案。
- Base URL 在 iOS 上强制 HTTPS；ATS 不为任意 HTTP 开口子。

## 10. 权限、隐私与生命周期

### 10.1 权限

- 照片/视频选择：`imagepicker` 使用系统 PHPicker，不需要相册权限。
- 文件选择：系统文档选择器，不需要额外权限。
- 相机：只有使用拍照识别曲谱时才需要 `NSCameraUsageDescription`。
- 网络：ASR 和曲谱识别需要联网。

### 10.2 隐私

- 视频导出完全本地，不上传。
- ASR 会上传抽取出的 PCM 音频片段。
- 曲谱识别会上传曲谱图片。
- 设置页和首次使用说明要明确这两点。
- 提供 `ios/PrivacyInfo.xcprivacy`，声明网络和文件访问。

### 10.3 生命周期

- `mygo.KeepState(a)` 保存界面状态。
- `Power.OnSuspend` 时保存工程、暂停播放。
- `App.OnMemoryWarning` 时释放缩略图、预览位图和导出缓冲。
- 工程本身已经落盘，app 被系统杀掉不丢编辑。

## 11. 配置与打包

`app/mygo.json` 增加：

```json
{
  "ios": {
    "team": "<Apple Team ID>",
    "infoPlist": {
      "NSPhotoLibraryUsageDescription": "选择视频和曲谱图片",
      "NSCameraUsageDescription": "拍摄曲谱用于识别",
      "UIFileSharingEnabled": true,
      "LSSupportsOpeningDocumentsInPlace": true
    }
  }
}
```

实际字段以 mygo 0.10 的配置文档为准；如果没有 `plugins` 字段，插件通过 import 自动发现。

资源：

- `resources/darwin-amd64/ffmpeg` 不会进入 iOS 包，因为它是桌面平台目录；
- iOS 不新增 ffmpeg；
- 字体、图标沿用现有资源；
- `icon.png` 需要 1024×1024。

签名：

- 模拟器不需要开发者账号；
- 真机需要 Apple Developer Team 和 provisioning profile；
- TestFlight/App Store 用 `mygo build -platform ios` + `mygo publish -platform ios`。

## 12. 测试与验收

### 12.1 自动化

- `project.ResolveSource` 的相对/绝对路径测试。
- 导入复制源视频、工程目录自包含测试。
- AVFoundation：
  - 时长；
  - PCM 长度和采样率；
  - 旋转视频的帧方向；
  - 横屏、竖屏、无音轨、HEVC 源。
- 导出：
  - 2 秒短片端到端；
  - 输出时长、分辨率、音轨存在；
  - 取消后无残留文件。
- UI golden：
  - iPad 横屏 1366×1024；
  - iPad 竖屏 820×1180；
  - iPhone 390×844；
  - 安全区 inset。

### 12.2 手工验收

- 从“文件”和“照片”各导入一次视频。
- 导入曲谱图片并识别和弦。
- 字幕逐字显示与音频对齐。
- 预览和导出画面对比，和弦、字幕位置一致。
- 导出 MP4 在系统相册播放，音画同步。
- 5 分钟视频导出，观察内存、发热、耗时。
- 导出中切后台、锁屏、旋转屏幕。
- 取消导出、删除工程、重新打开工程。

### 12.3 性能目标

- 预览稳定 30fps。
- 1 分钟 1080×1920：
  - CPU 导出不超过 3 倍实时；
  - Metal 导出不超过 1.2 倍实时。
- 峰值内存不超过 500MB。

用 `MYGO_FRAME_STATS=1` 和导出日志定位瓶颈。

## 13. 里程碑

| 阶段 | 内容 | 预计 |
|---|---|---|
| M0 | Spike：mygo iOS 跑起来、播放视频、AVF 取时长/PCM、解一帧、编 2 秒 | 2–3 天 |
| M1 | 导入 + 工程目录自包含 + ASR + 曲谱 | 4–5 天 |
| M2 | iPad 横屏 UI + CPU 本地导出 + 分享/SRT | 6–8 天 |
| M3 | Metal 加速 spike，决定是否 fork mygo | 3–5 天 |
| M4 | iPad 竖屏/iPhone 布局、Sheet、触控 | 5–7 天 |
| M5 | 真机测试、隐私、签名、TestFlight | 3–5 天 |

按 iPad-first 算，约 2–3 周可以拿到可用的本地导出版本；
加 iPhone 适配和 Metal 优化，约 4–6 周。

## 14. 风险

| 风险 | 影响 | 应对 |
|---|---|---|
| 旋转/裁切变换算错 | 导出画面方向或比例错 | 用 AVAssetReaderVideoCompositionOutput 让系统处理 transform，固定测试视频 |
| 音画不同步 | 成片不可用 | 先做两段式合成，必要时改单次 writer |
| CPU 栅格化太慢 | 导出时间长 | Metal spike；720p 快速导出；pwc 仅作可选 |
| mygo iOS render 缺口 | GPU 路径不可用 | 先 CPU；fork 小改；给上游提 PR |
| 大视频复制占空间 | 存储压力 | 导入时提示大小；删除工程时清理 |
| 后台挂起 | 导出中断 | 首版前台导出 + 常亮 + 取消清理 |
| ATS/HTTP Base URL | 请求失败 | iOS 强制 HTTPS |

## 15. 明确不做

- 不在 iOS 内执行 ffmpeg。
- 不把 ffmpeg 静态链接进 app 作为首版方案。
- 不要求 pwc 才能导出。
- 首版不追求完全离线 ASR/曲谱识别。
- 首版不做后台断点续传导出。

## 16. 建议的下一步

先做 M0 Spike，不写业务代码：

1. 在 iPad 模拟器跑通当前 app 的 UI 和视频播放。
2. 用 AVFoundation 读一个竖屏视频的时长和第一帧。
3. 用 AVAssetWriter 编码 2 秒纯色视频。
4. 测一下 CPU 渲染 1080×1920 一帧的耗时。
5. 再决定：CPU 首版，还是直接投入 Metal 路径。

Spike 结果出来后再进入 M1，避免在错误的导出方案上投入。
