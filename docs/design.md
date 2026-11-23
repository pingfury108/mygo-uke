# mygo-uke · 设计文档

> 配套需求见 `mygo-uke-requirements.md`。
> 本文只讲「怎么实现」和「为什么这么选」。

---

## 0. 一句话结论

**一个 Go 写的 MyGo 原生桌面客户端：视频 + 曲谱一次导入，客户端内完成字幕对齐、曲谱解析与成片导出，不做音频和弦识别。**

```
mygo-uke/
├─ app/        Go 1.27 + MyGo v0.10     客户端，约 80MB 分发
└─ docs/
```

曲谱是唯一和弦来源。字幕和曲谱由客户端在**同一套 view 代码**里画到画面上，
再用 ffmpeg 编码成片——不经过 ASS，不经过第二套渲染器。

---

## 1. 仓库结构

```
mygo-uke/
├─ README.md
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
│  │  ├─ pipeline/           编排：抽 PCM → ASR → 曲谱对齐
│  │  ├─ asr/                火山流式 ASR 客户端
│  │  ├─ sheet/              曲谱解析、ASR 对齐、字符级和弦
│  │  ├─ score/              bars / 和弦条布局（纯逻辑，可单测）
│  │  ├─ uke/                尤克里里和弦词典 + 指法图（写死，不做多乐器）
│  │  ├─ render/             view 定义 + 离屏渲染
│  │  ├─ compose/            ffmpeg 解码/编码管道
│  │  ├─ ffbin/              内置 ffmpeg（抄 mygo-video）
│  │  └─ subtitle/           SRT 导出（抄 mygo-video）
│  ├─ resources/fonts/       思源黑体子集
│  └─ designs/
```

**为什么 `app/` 是独立 Go module**：MyGo 工具链从 module 根目录跑
`go tool mygo dev`，独立 module 让客户端依赖保持干净。

---

## 2. 关键决策

### ADR-1：客户端用 MyGo 原生 App，不用 Web

| 理由 | 说明 |
|---|---|
| 已有生态 | `mygo-video`（4 万行）是现成范本，`internal/ffbin`、`internal/subtitle`、`ui/video` 直接复用 |
| 分发 | 单个 `.app`，无 WebView、无 Node 运行时 |
| 播放 | `ui/video` 走系统框架硬解（AVFoundation），零拷贝上 GPU |

**被否决**：React + Vite + 本地 HTTP 服务。多一个运行时、多一套构建、最终还是要打壳。

### ADR-2：客户端单进程，曲谱解析走 OpenAI 兼容接口

图片曲谱交给支持视觉的聊天模型识别，文本曲谱本地解析；客户端不引入
Python、不进模型权重。接口约定固定为 `docs/design.md` §5 的 JSON 结构。

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

### ADR-4：和弦来自曲谱，不做音频识别

| 备选 | 否决理由 |
|---|---|
| 音频和弦识别 | 弹唱混音下准确率不稳定，逐字位置更难对齐 |
| 人工逐字标和弦 | 一首歌几十个和弦，操作成本过高 |
| 解析已有曲谱 | 和弦、调性、BPM、拍号本来就是谱面信息，最可靠 |

**选曲谱作为唯一和弦来源。** 图片曲谱用视觉模型转成结构化 JSON，
文本曲谱本地解析；和弦锚定到歌词字符，持续到下一个和弦。曲谱没有
BPM 时，小节按和弦变化点近似生成。

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

### ADR-6：字幕按句对齐、按字呈现

- 火山 `utterances[]` 天然有句级 `start/end`，够用
- 曲谱和 ASR 做逐字全局对齐，一行曲谱可以跨多句 ASR
- 对上的字使用 ASR `words[]` 词级时间，句首句尾静音裁掉
- ASR 漏字/错字在前后已对上的字之间插值
- 视频没唱到的重复谱面直接丢弃，不按行数硬插值
- 画面只显示当前正在唱的字，随下一个字出现而消失
- 和弦不画在字幕上，顶部和弦条已负责展示

### ADR-7：不内置节奏型

扫弦是演奏者的自由，同一首歌十个人十种扫法，**ground truth 不唯一**。
不识别，也不提供模板手选，工程里不保存 pattern；编辑器只负责曲谱、
和弦、字幕和时间轴。

### ADR-8：只做尤克里里，不做乐器抽象

**反面设计**（否决）：`type Instrument interface { ChordShapes(name string) []Shape }` + 吉他/钢琴实现。

理由：

| | 单乐器写死 | 乐器抽象层 |
|---|---|---|
| 代码量 | 一个 `uke` 包，~300 行 + 一张表 | 接口 + 注册 + 每个乐器一张表 |
| 分发影响 | 无 | 吉他/钢琴表白白占体积 |
| 需求确定性 | **只弹尤克里里，这是事实** | 为不存在的需求付设计成本 |

**关键认知：曲谱里的和弦名是乐器无关的**（`C`、`Am`、`G7`）。「只支持尤克里里」这个约束**只落在客户端渲染层**。

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
│ app（MyGo 桌面客户端 · 零本地模型 · ~80MB）    │
│                                              │
│  ┌─────────┐  ┌──────────┐  ┌────────────┐  │
│  │ 校对台   │  │ pipeline │  │ compose    │  │
│  │ ui_*.go │→ │ 编排      │→ │ Tester+ffmpeg│ │
│  └─────────┘  └────┬─────┘  └────────────┘  │
│      ↑             │                          │
│  project.json ←────┘                          │
└──────────┬────────────────────┬──────────────┘
           │ 16k mono PCM       │ 图片曲谱
           ▼                    ▼
   ┌───────────────┐   ┌──────────────────────┐
   │ 火山豆包 ASR   │   │ OpenAI 兼容视觉模型   │
   │ 流式 v3 / WSS  │   │ 只输出结构化曲谱 JSON  │
   │ 句级 utterances│   │ 文本曲谱则本地解析     │
   └───────────────┘   └──────────────────────┘
```

ASR 与曲谱解析互不依赖；曲谱行最终用 ASR 的句级时间对齐到视频时间轴。

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

  "style": {
    "chordSize": 64, "subSize": 60, "subtitleY": 78,
    "accent": "#FFD166"
  }
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
1. 新建工程  选择视频 + 曲谱（图片/文本）
2. 读时长    ffprobe
3. 抽音轨    ffmpeg -i in.mp4 -vn -ac 1 -ar 16000 -f s16le out.pcm
4. ASR       PCM 16k 帧 → 火山流式 → utterances[]
5. 解析曲谱  图片走视觉模型，文本本地解析 → key/capo/BPM/拍号/逐行和弦
6. 对齐      曲谱行匹配 ASR 句，和弦锚定到具体字符
7. 生成      chords + bars + lines；有 BPM 用等间隔拍点，否则按和弦变化点
8. 落盘      project.json
```

没有 ASR Key 时跳过第 4 步，曲谱行按总时长平均分布；没有曲谱时只生成字幕。

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
│ 导入  |  分析  |  竖屏/横屏  |  导出                  │
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
- 和弦编辑：点小节 → 弹和弦选择器 → 立刻重排；手动修改在重新分析后保留
- 字幕编辑：点卡片跳播放，双击编辑文字和起止时间；手动修改在重新分析后保留
- 播放快捷键：空格/K 播放暂停，J/L 退/进 10 秒，←/→ 退/进 5 秒，M 静音
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

**待验证点**：曲谱里的和弦如果落在表外（比如 `Bb`、`Fm`、`Ab`），要确认这些在尤克里里上有合理按法。**标准调弦下 `Bb`、`Eb`、`Ab` 都是 barre 和弦，对新手不友好但存在。** 计划里要给「变调夹建议」：检测到大量 barre 时，提示「夹 1 品弹 A 调更顺」。

---

## 5. 曲谱 JSON 契约

视觉模型与本地文本解析都输出同一结构；客户端只认这个契约。

```json
{
  "key": "C",
  "capo": 0,
  "bpm": 90,
  "timeSignature": [4, 4],
  "lines": [
    {
      "lyrics": "好想能看到 你嘴角微笑",
      "chords": [
        { "at": 0, "label": "F" },
        { "at": 3, "label": "G" }
      ]
    }
  ]
}
```

规则：

1. `at` 是歌词的 0 基字符下标，中文按一个字一个下标；和弦覆盖多个字时只标起始字。
2. 和弦从 `at` 开始，持续到下一个和弦、句尾或曲尾。
3. `bpm` 缺失时为 0；`timeSignature` 缺失时按 4/4。
4. 没有歌词的纯和弦段落也要输出，`lyrics` 为空字符串。
5. 无法判断位置时省略 `at`，客户端按行内均分并标记为近似。

---

## 6. 风险与验证清单（M0）

**M0 不过，不进 M1。三项，一天内跑完。**

| # | 验什么 | 方法 | 不过怎么办 |
|---|---|---|---|
| **V1** | 曲谱 bpm/拍号能否生成可用小节 | 拿 3 首带 BPM 的曲谱，把拍点叠在波形上人眼对 | 无 BPM 的谱按和弦变化点生成小节，并标为近似 |
| **V2** | 火山流式 ASR 喂歌声的效果 | 灌 30 秒副歌 PCM，看定稿文本 + `utterances` 时间 | 掉字严重 → 换 `auc` 录音文件识别（另一套协议）；或先 Demucs 分离再喂 |
| **V3** | `ui.NewTester` 渲染 1080×1920 单帧耗时 | 一个 `cmd/bench` 程序，1 张位图 + 2 行中文，跑 100 帧取均值 | > 80ms → 降至 720p 渲染；或改分段渲染 + ffmpeg overlay（需反解 alpha） |

**风险排序：**

| 风险 | 级别 | 缓解 |
|---|---|---|
| V1 小节边界不准 | 🟡 中 | 有 BPM 用等间隔拍点；没有时按和弦变化点近似，可在 UI 手动改 |
| V3 渲染太慢 | 🟡 中 | 降分辨率 / 缓存叠加层位图 |
| V2 歌声识别差 | 🟡 中 | 换 auc / 先分离 / 人工改字是兜底（反正要校对） |
| 曲谱图片识别失败 | 🟡 中 | 允许粘贴文本曲谱，或导入后逐字/逐小节手动改和弦 |
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
