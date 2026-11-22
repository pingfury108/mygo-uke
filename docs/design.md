# mygo-uke · 设计文档

> 配套需求见 `mygo-uke-requirements.md`。
> 本文只讲「怎么实现」和「为什么这么选」。

---

## 0. 一句话结论

**一个 monorepo，两个子项目：Go 写的 MyGo 桌面客户端（零模型）+ Python 写的和弦分析服务（独立部署）。**

```
mygo-uke/
├─ app/        Go 1.27 + MyGo v0.10     客户端，约 80MB 分发
├─ chordd/     Python 3.11 + BTC        和弦/节拍服务，部署在别处
└─ docs/
```

字幕和曲谱由客户端在**同一套 view 代码**里画到画面上，再用 ffmpeg 编码成片——不经过 ASS，不经过第二套渲染器。

---

## 1. 仓库结构

```
mygo-uke/
├─ README.md
├─ Makefile                  顶层：dev / build / chordd-up / verify
├─ docs/
│  ├─ requirements.md
│  └─ design.md
├─ app/                      ── ① 客户端（独立 Go module）
│  ├─ go.mod                 module mygo-uke
│  ├─ main.go
│  ├─ mygo.config.ts         MyGo 打包配置（签名/公证/更新）
│  ├─ internal/
│  │  ├─ app/                视图 ui_*.go
│  │  ├─ project/            project.json 模型 + 读写
│  │  ├─ pipeline/           编排：抽音轨 → ASR ∥ chordd → 对齐
│  │  ├─ asr/                火山流式 ASR 客户端
│  │  ├─ chord/              chordd HTTP 客户端
│  │  ├─ score/              bars / 和弦条布局（纯逻辑，可单测）
│  │  ├─ uke/                尤克里里和弦词典 + 指法图（写死，不做多乐器）
│  │  ├─ render/             view 定义 + 离屏渲染
│  │  ├─ compose/            ffmpeg 解码/编码管道
│  │  ├─ ffbin/              内置 ffmpeg（抄 mygo-video）
│  │  └─ subtitle/           SRT 导出（抄 mygo-video）
│  ├─ resources/fonts/       思源黑体子集
│  └─ designs/
└─ chordd/                   ── ② 和弦服务（独立 Python 工程）
   ├─ pyproject.toml         uv 管理
   ├─ chordd/
   │  ├─ main.py             FastAPI
   │  ├─ analyze.py          BTC + beat_this
   │  ├─ models.py           权重下载/缓存
   │  └─ cache.py            按音频 md5 缓存
   ├─ Dockerfile
   └─ README.md
```

**为什么 `app/` 是独立 Go module**：MyGo 工具链从 module 根目录跑 `go tool mygo dev`，独立 module 让两个子项目互不污染依赖。

---

## 2. 关键决策

### ADR-1：客户端用 MyGo 原生 App，不用 Web

| 理由 | 说明 |
|---|---|
| 已有生态 | `mygo-video`（4 万行）是现成范本，`internal/ffbin`、`internal/subtitle`、`ui/video` 直接复用 |
| 分发 | 单个 `.app`，无 WebView、无 Node 运行时 |
| 播放 | `ui/video` 走系统框架硬解（AVFoundation），零拷贝上 GPU |

**被否决**：React + Vite + 本地 HTTP 服务。多一个运行时、多一套构建、最终还是要打壳。

### ADR-2：两个子项目同仓，但语言和部署分离

同仓解决「接口契约必须一起改」的问题；语言分离解决「客户端不能有 Python」的问题。

**契约放在 `docs/design.md` §5 和 `chordd/` 的 OpenAPI 里，两边都要有测试。**

### ADR-3：ASR 用火山豆包流式，直接复用 `sotto/internal/asr`

**⚠️ 重要事实：火山 ASR 不是 OpenAI 兼容的。**

证据一——自己的代码：

```
sotto/internal/asr/protocol.go
  端点   wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async
  鉴权   X-Api-Key / X-Api-Resource-Id / X-Api-Request-Id / X-Api-Sequence
  帧格式 4B header + 4B 大端长度 + payload（gzip）
  音频   16kHz / 16bit / 单声道 / 100ms 一包（3200B）
  结果   result.text 是**全量累计文本**；flags & 0x2 = 定稿
```

证据二——社区专门有个 [`doubao-asr2-openai-proxy`](https://github.com/gydx6/doubao-asr2-openai-proxy)，干的就是「把豆包 ASR 2.0 的 WebSocket 桥接成 `/v1/audio/transcriptions`」。**需要桥接 = 原生不兼容。**

**决定：不引入 proxy，直接用 `sotto` 那套客户端。** 它已经处理了首帧、gzip、末帧定稿、错误帧，还有单测——比 proxy 更完整。

做法：把 `sotto/internal/asr` 抽成公共包（或直接 copy 进 `app/internal/asr`，保持可追溯）。**不重写。**

### ADR-4：和弦做成独立部署的 `chordd`

| 备选 | 否决理由 |
|---|---|
| 买 SaaS（Moises music.ai） | 黑盒、按量计费、**不确定给不给小节边界（downbeats）** |
| 客户端内嵌 ONNX | 模型 150–350MB，且 BTC 导出 ONNX 有坑 |
| 纯 Go 自研 DSP | 准确率 ~60%，不够 |

**选 Python + BTC + beat_this，独立部署。** Python 只活在服务器上，不进分发物——这是整件事的关键分界。

### ADR-5：导出用 Tester 离屏渲染，不用 ASS

MyGo 有公开的离屏渲染能力：

```go
// ui/headless.go
func NewTester(view func(c *Context), width, height int) *Tester
func (t *Tester) Frame()
func (t *Tester) Image() *image.RGBA
// headless renders frames in memory with the software renderer.
```

**流程：**

```
每帧：视频帧 RGBA → ui.NewBitmap → ui.Image 画在底层
      + 字幕 / 和弦条 / 指法图
                    ↓
            Tester.Frame() → Image()
                    ↓
            rawvideo pipe → ffmpeg → libx264
```

| | ASS + libass | Tester 离屏 |
|---|---|---|
| 预览与成片同源 | ❌ 两套渲染器 | ✅ **同一份 view** |
| 字体 fallback 差异 | 常见 | 无 |
| 动效表达力 | ASS 语法限制 | 任意 Go 代码 |
| 依赖 | ffmpeg 必须带 libass | 只要 h264 + aac |

**已知约束**：`internal/scene` 里 `Background` 是 **opaque** 的，`Image()` 拿不到透明层。所以不能「只渲染叠加层交给 ffmpeg overlay」，必须整帧渲染。

**性能对策**（都很好实现）：
1. 当前字幕/和弦条渲染成**缓存位图**，内容不变时只 blit
2. 单帧预算 40ms，1080×1920 整帧 ≈ 5400 帧 ≈ 3.6 分钟
3. 超预算 → 降到 1280×720 渲染，ffmpeg 放大（大字损失可接受）

### ADR-6：字幕只做句级

- 火山 `utterances[]` 天然有句级 `start/end`，够用
- 砍掉字级 → 叠加层变化点从 ~5400 降到 ~150
- 卡拉OK 逐字动画留到 P2（F14）

### ADR-7：节奏型手选，不识别

扫弦 pattern 是演奏者的自由度，同一首歌十个人十种扫法，**ground truth 不唯一**。给 6 个模板让用户点一下，是产品设计不是技术妥协。

模板：`folk-basic`（↓ ↓↑ ↑↓↑）、`down-up`、`shuffle`、`fingerpick-3`、`waltz-3/4`、`custom`。

### ADR-8：只做尤克里里，不做乐器抽象

**反面设计**（否决）：`type Instrument interface { ChordShapes(name string) []Shape }` + 吉他/钢琴实现。

理由：

| | 单乐器写死 | 乐器抽象层 |
|---|---|---|
| 代码量 | 一个 `uke` 包，~300 行 + 一张表 | 接口 + 注册 + 每个乐器一张表 |
| 分发影响 | 无 | 吉他/钢琴表白白占体积 |
| 需求确定性 | **只弹尤克里里，这是事实** | 为不存在的需求付设计成本 |

**关键认知：`chordd` 返回的和弦名是乐器无关的**（`C`、`Am`、`G7`）。「只支持尤克里里」这个约束**只落在客户端渲染层**，不影响服务端，也不需要服务端感知乐器。

所以 `uke` 包只做三件事：

```go
func Normalize(label string) (Chord, bool)   // D# → Eb，C/G → C，Cmaj9 → Cmaj7
func Shape(c Chord, capo int) (Shape, bool)  // 取最容易的按法
func Available(c Chord) bool                 // 尤克里里弹不弹得了
```

---

## 3. 总体架构

```
┌──────────────────────────────────────────────┐
│ app（MyGo 桌面客户端 · 零模型 · ~80MB）        │
│                                              │
│  ┌─────────┐  ┌──────────┐  ┌────────────┐  │
│  │ 校对台   │  │ pipeline │  │ compose    │  │
│  │ ui_*.go │→ │ 编排      │→ │ Tester+ffmpeg│ │
│  └─────────┘  └────┬─────┘  └────────────┘  │
│      ↑             │                          │
│  project.json ←────┘                          │
└──────────┬────────────────────┬──────────────┘
           │ mp3 128k mono      │ mp3 128k mono
           ▼                    ▼
   ┌───────────────┐   ┌──────────────────────┐
   │ 火山豆包 ASR   │   │ chordd（Python）      │
   │ 流式 v3 / WSS  │   │ BTC + beat_this       │
   │ 句级 utterances│   │ chords/beats/downbeats│
   └───────────────┘   └──────────────────────┘
```

**两路并发**：ASR 和 chordd 同时发，别串行。

---

## 4. 客户端设计

### 4.1 `project.json`（唯一真相）

```jsonc
{
  "version": 1,
  "source": "/Users/x/Movies/晴天.mp4",
  "duration": 182.4,
  "audioHash": "sha256:...",          // 缓存键

  "tempo": 68.0,
  "timeSignature": [4, 4],
  "key": "C",
  "capo": 0,
  "tuning": "GCEA",

  "beats":     [0.52, 1.41, 2.29],
  "downbeats": [0.52, 3.30, 6.05],

  "chords": [
    { "start": 0.52, "end": 3.30, "label": "C", "conf": 0.93 }
  ],

  "bars": [
    { "i": 0, "start": 0.52, "end": 3.30, "chord": "C", "conf": 0.93 },
    { "i": 1, "start": 3.30, "end": 6.05, "chord": "G", "conf": 0.88 }
  ],

  "lines": [
    { "start": 1.10, "end": 4.60, "text": "故事的小黄花" }
  ],

  "pattern": "folk-basic",
  "style": { "chordSize": 64, "subSize": 60, "accent": "#FFD166" }
}
```

**`bars` 是整件事的枢纽**：竖屏和弦条、横屏谱面、播放头高亮，全部读它。

### 4.2 `bars` 生成规则（`internal/score`）

1. 按 `downbeats` 切小节区间 `[d[i], d[i+1])`
2. 对每个小节，取落在其中的 `chords` 片段，按**重叠时长加权投票**出一个标签
3. 若和弦变化点落在小节内第 3 拍之后且该和弦延续 ≥ 1 拍 → 允许 `[["C", "G"]]` 双和弦
4. `conf < 0.6` → 标记 `needsReview: true`，UI 标黄
5. 若 `downbeats` 数量与 `chords` 的变化次数严重不匹配 → 整个工程标 `rhythmSuspect`，提示用户

**这一步是纯函数，必须写单测**（喂构造数据，断言输出）。

### 4.3 管线时序

```
1. 导入      ffmpeg -i in.mp4 -vn -ac 1 -ar 16000 -b:a 128k out.mp3
2. 并发
   ├─ ASR   ：PCM 16k 帧 → 火山流式 → utterances[]
   └─ chordd：POST /v1/analyze → 轮询 → chords/beats/downbeats/key
3. 对齐      两条时间轴都基于同一份 audio → 天然同源
4. 合成      bars（规则）+ lines（ASR 句级）+ tempo/key
5. 落盘      project.json
```

**注意**：ASR 要 16k 单声道 PCM，chordd 要 44.1k（或 22.05k）音频——**抽两轨**，别互相将就。

### 4.4 渲染与合成

```
导出竖屏：
  ffmpeg -i source.mp4 -vf scale=1080:1920 -f rawvideo -pix_fmt rgba -
    → 读 5400 帧
    → 每帧：NewBitmap → ui.Image 铺满 → 叠字幕/和弦条/指法图 → Tester.Frame()
    → Image() RGBA → 写 stdout
  → ffmpeg -f rawvideo -s 1080x1920 -pix_fmt rgba -i - -i audio.wav
           -c:v libx264 -crf 18 -preset medium -pix_fmt yuv420p
           -c:a aac -b:a 192k -movflags +faststart out_vertical.mp4
```

**必须 `-pix_fmt yuv420p`**，否则部分平台黑屏。
**必须 `+faststart`**，否则拖动卡。

横屏用同一套 view，只是布局函数换成 `layoutHorizontal()`。

### 4.5 UI 结构

```
┌──────────────────────────────────────────────────────┐
│ 导入  |  分析  |  节奏型 ▾  |  竖屏/横屏  |  导出      │
├──────────┬───────────────────────┬───────────────────┤
│ 工程列表  │      预览画布          │ 检查器             │
│          │   (1080×1920 缩放)     │ ┌───────────────┐ │
│ 晴天      │                       │ │ 字幕（逐句）    │ │
│ 小情歌    │   [和弦条]             │ │ 00:38 故事的小… │ │
│          │     人像               │ └───────────────┘ │
│          │   [歌词字幕]           │ ┌───────────────┐ │
│          │                       │ │ 和弦（按小节）   │ │
│          │                       │ │ C G Am Em …    │ │
│          │                       │ └───────────────┘ │
├──────────┴───────────────────────┴───────────────────┤
│ 时间轴：▶ 00:42 ────●──── 小节刻度 |▮▮▮|▮▮▮|          │
└──────────────────────────────────────────────────────┘
```

**核心交互：一把播放头驱动三处。** 拖进度 → 预览帧、字幕块高亮、谱面小节高亮同时动。

- 播放头用 rAF 读 `video.currentTime`，**不要监听 `timeupdate`**（每秒只 4 次，抖）
- 和弦编辑：点小节 → 弹和弦选择器（尤克里里常用 20 个的矩阵）→ 立刻重排
- 字幕编辑：逐句卡片，点一下跳播放，改字即时反映
- **状态只在主线程写**，goroutine 通过 `post` 回主线程（照 `mygo-video` 的规矩）

### 4.6 包职责边界

| 包 | 能做什么 | 不能做什么 |
|---|---|---|
| `score` | 算 bars、和弦条位置、换行 | 不许 import `ui` |
| `uke` | 和弦归一化、指法图数据、可弹性判断 | 不许 import `ui`、不许 import `render` |
| `render` | 只画，输入 `project.json` + 时间 t | 不许算布局 |
| `pipeline` | 编排、重试、进度回调 | 不许碰 UI |
| `compose` | ffmpeg 管道 | 不许知道「和弦」是什么 |

`score` 不依赖 `ui`，所以能写纯单测——这是可测性的关键。

### 4.7 尤克里里和弦词典（`internal/uke`）

**唯一的乐器相关代码，写死，不抽象。**

```go
// 标准调弦 GCEA（high-G），从 4 弦到 1 弦
var strings = [4]int{67, 60, 64, 69} // G4 C4 E4 A4

type Shape struct {
    Frets  [4]int // 4 弦 → 1 弦，-1 = 不弹，0 = 空弦，n = 第 n 品
    Base   int    // 图示起始品（>1 时显示 "5fr"）
}

// 一张表，覆盖 ~80 个和弦，每个 1–3 个常用按法，按易弹排序
var shapes = map[string][]Shape{
    "C":    {{Frets: [4]int{0, 0, 0, 3}, Base: 1}},
    "Am":   {{Frets: [4]int{2, 0, 0, 0}, Base: 1}},
    "F":    {{Frets: [4]int{2, 0, 1, 0}, Base: 1}},        // uke 的 F 很好按
    "G":    {{Frets: [4]int{0, 2, 3, 2}, Base: 1}},
    "G7":   {{Frets: [4]int{0, 2, 1, 2}, Base: 1}},
    "Em":   {{Frets: [4]int{0, 4, 3, 2}, Base: 1}},
    "Dm":   {{Frets: [4]int{2, 2, 1, 0}, Base: 1}},
    "E7":   {{Frets: [4]int{1, 2, 0, 2}, Base: 1}},
    // ...
}
```

**归一化规则（`Normalize`）：**

| 输入 | 输出 | 说明 |
|---|---|---|
| `D#m` | `Ebm` | enharmonic 归一，优先用降号还是升号看 `key` |
| `C/G` | `C` | slash 和弦取根音，原标记保留在 `bars[].altLabel` |
| `Cmaj9` | `Cmaj7` | 扩展和弦降级到可弹 |
| `G/B` | `G` | 同上 |

降级时 `bars[].approx = true`，谱面在角标一个 `~`。

**变调夹语义（最容易写错）：**

```go
// capo = 2，谱面写 "C"，实际声响是 D
// 显示：原调 D · Capo 2 · 按 C 形状
// 所以：shapeIndex = 按 C 查表；显示调 = transpose(C, +capo)
```

内部统一存**「实际调」**，渲染时再减 capo 反推形状 —— 这样移调操作不用动 capo 逻辑。

**待验证点**：`chordd` 返回的和弦如果落在表外（比如 `Bb`、`Fm`、`Ab`），要确认这些在尤克里里上有合理按法。**标准调弦下 `Bb`、`Eb`、`Ab` 都是 barre 和弦，对新手不友好但存在。** 计划里要给「变调夹建议」：检测到大量 barre 时，提示「夹 1 品弹 A 调更顺」。

---

## 5. `chordd` 接口契约

### 5.1 端点

```http
POST /v1/analyze
Content-Type: multipart/form-data
  audio=<file>        # mp3/wav，≤ 10MB
  downmix=true        # 可选
→ 202 { "jobId": "abc123", "etaMs": 45000 }

GET /v1/jobs/{jobId}
→ 200 {
  "status": "pending|running|done|failed",
  "progress": 0.62,
  "error": null
}

GET /v1/jobs/{jobId}/result
→ 200 {
  "tempo": 68.0,
  "timeSignature": [4, 4],
  "key": "C",
  "beats":     [0.52, 1.41, 2.29],
  "downbeats": [0.52, 3.30, 6.05],        // ★ 命根子，没有它整个交互不成立
  "chords": [
    { "start": 0.52, "end": 3.30, "label": "C", "conf": 0.93 }
  ],
  "sections": [ { "start": 0, "end": 48, "label": "verse" } ]
}

GET /healthz
→ 200 { "ok": true, "models": ["btc-large-voca", "beat-this"] }
```

### 5.2 实现

| 环节 | 选型 |
|---|---|
| Web | FastAPI + uvicorn |
| 节拍/小节 | **beat_this**（SOTA，pip 装，一次出 tempo + beat + downbeat） |
| 和弦 | **BTC large-voca** 现成 checkpoint |
| 调性 | 和弦序列 + Krumhansl 模板（自己写 30 行） |
| 段落 | 和弦自相似矩阵（不引 all-in-one，太重） |
| 缓存 | 按音频 sha256 落盘，重跑秒回 |
| 部署 | Dockerfile + runpulse |

**必须遵守：**

1. **`downbeats` 一定要返回**，哪怕是降级估算
2. 时间单位统一**秒（float）**，别混毫秒
3. 长音频按 30 秒窗口推理后拼接，**时间偏移要加回去**
4. CPU 也要能跑（无 GPU 时 3 分钟歌 40–90 秒）

---

## 6. 风险与验证清单（M0）

**M0 不过，不进 M1。三项，一天内跑完。**

| # | 验什么 | 方法 | 不过怎么办 |
|---|---|---|---|
| **V1** | `chordd` 的 `downbeats` 跟实际小节对不对 | 拿自己 3 首歌，把 downbeats 画成竖线叠在波形上，人眼对 | 差半拍 → 整个「和弦跟小节」交互重设计；或改用「和弦变化点 + 固定 4 拍」推 |
| **V2** | 火山流式 ASR 喂歌声的效果 | 灌 30 秒副歌 PCM，看定稿文本 + `utterances` 时间 | 掉字严重 → 换 `auc` 录音文件识别（另一套协议）；或先 Demucs 分离再喂 |
| **V3** | `ui.NewTester` 渲染 1080×1920 单帧耗时 | 一个 `cmd/bench` 程序，1 张位图 + 2 行中文，跑 100 帧取均值 | > 80ms → 降至 720p 渲染；或改分段渲染 + ffmpeg overlay（需反解 alpha） |

**风险排序：**

| 风险 | 级别 | 缓解 |
|---|---|---|
| V1 小节边界不准 | 🔴 高 | 决定整个产品形态，最先验 |
| V3 渲染太慢 | 🟡 中 | 降分辨率 / 缓存叠加层位图 |
| V2 歌声识别差 | 🟡 中 | 换 auc / 先分离 / 人工改字是兜底（反正要校对） |
| chordd 服务挂了 | 🟡 中 | 明确报错 + 重试；不做离线降级 |
| 火山限流/超时 | 🟢 低 | 单用户，退避重试即可 |

---

## 7. 里程碑

| 阶段 | 内容 | 天 |
|---|---|---|
| M0 | 三项验证 | 1 |
| M1 | 骨架（`mygo dev` 能起）+ 管线 + `project.json` | 2–3 |
| M2 | 校对台：播放头联动、字幕编辑、和弦点选、实时预览 | 3–4 |
| M3 | 合成导出竖屏 | 3 |
| M4 | 横屏 + SRT + 样式 | 2 |

**合计约 2 周。** M2 是最重的一段，也是最容易做歪的一段——先做「播放头联动 + 一个 static overlay」的最小版本，再往上加编辑。

---

## 8. 复用清单（少写一半代码）

| 从哪抄 | 抄什么 |
|---|---|
| `mygo-video/internal/ffbin` | 内置 ffmpeg 二进制 + 路径解析 |
| `mygo-video/internal/executil` | 子进程封装（Windows 无控制台窗口） |
| `mygo-video/internal/subtitle` | SRT/ASS 解析与写出（F10 用） |
| `mygo-video/internal/app/datadir.go` | 数据目录规范 |
| `mygo-video` 的 `ui_*.go` | 列表/工具栏/设置页的写法范本 |
| `sotto/internal/asr` | 火山流式 ASR 客户端（**整包搬**） |
| `sotto/scripts/signing` | 开发签名证书，避免每次重建都要重新授权 |

---

## 9. 明确不引入的东西

| 不引入 | 理由 |
|---|---|
| Python / PyTorch 进客户端 | 分发体积灾难，这是整份设计的起点 |
| ASS + libass 合成 | 两套渲染器必然不一致（ADR-5） |
| OpenAI 兼容 proxy（`doubao-asr2-openai-proxy`） | 多一跳、多一个进程，而 `sotto` 的实现更完整 |
| music21 / LilyPond | 那是五线谱的路 |
| Remotion / Web 渲染 | 客户端不是 Web |
| 本地 ONNX 模型 | P0 不做；未来要离线再说 |
| Celery / Redis / 数据库 | 单用户单机，`project.json` + 目录就是存储 |
| 多乐器抽象（吉他/钢琴表） | 只弹尤克里里，抽象层是为不存在的需求付费（ADR-8） |
| 第三方乐谱库（如 chordpro 生态） | 弹唱卡的排版是自己的一套规则，引进来反而要迁就 |
