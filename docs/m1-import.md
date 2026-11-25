# M1 导入管线

## 流程

```text
新建工程：选择/拖入视频 + 曲谱
  → ffprobe 读时长
  → 配了火山 Key 时抽 16kHz mono s16le PCM
  → 火山流式 ASR 转写句级字幕
  → 识别/解析曲谱（图片走视觉模型，文本本地解析）
  → 把曲谱行对齐到 ASR 时间轴
  → 用曲谱和弦生成字符锚点、和弦区间与小节
  → 保存 project.json
```

音频和弦/节拍分析已从客户端移除。曲谱是唯一和弦来源；曲谱写明了 BPM
和拍号就生成等间隔拍点，没有时按和弦变化点生成小节。没有曲谱也可以
创建工程，但只有字幕。

没有源文件时，界面仍保留示例工程用于设计校对。

## 代码位置

- `app/internal/media`：ffmpeg / ffprobe 包装
- `app/internal/asr`：火山双向流式 ASR（复用 `sotto`，补 utterances 解析）
- `app/internal/sheet`：图片/文本曲谱解析、ASR 对齐、字符级和弦
- `app/internal/pipeline`：抽 PCM → 转写 → 曲谱对齐 → 生成工程
- `app/internal/project`：`project.json`、原子保存、本地工程库
- `app/internal/app/ui_new_project.go`：一次选择视频 + 曲谱
- `app/internal/app/import.go`：后台任务和 UI 状态

## 环境变量

```text
MYGO_UKE_PROJECTS=/path/to/library 工程库目录
MYGO_UKE_FFMPEG=/path/to/ffmpeg     ffmpeg 路径；不配则找内置资源或 PATH
MYGO_UKE_VOLCANO_API_KEY=...        火山 ASR Key；不配则字幕按曲谱行均分
MYGO_UKE_VOLCANO_RESOURCE_ID=...    资源 ID，默认 2.0 小时版
MYGO_UKE_VOLCANO_WS_URL=...         WebSocket 端点，默认官方 v3
MYGO_UKE_VOLCANO_MODEL=bigmodel     模型名
MYGO_UKE_SHEET_BASE_URL=...         曲谱识别 Base URL（或 OPENAI_BASE_URL）
MYGO_UKE_SHEET_API_KEY=...          曲谱识别 Key（或 OPENAI_API_KEY）
MYGO_UKE_SHEET_MODEL=...            支持图片输入的模型
```

也可以在应用左下角「设置」里填写这些值。设置保存在
`~/Library/Application Support/mygo-uke/settings.json`（权限 0600），
可用 `MYGO_UKE_SETTINGS` 覆盖路径；环境变量仍然优先于保存的值。

## 测试

普通单测用 fake media，不启动 ffmpeg 和网络：

```sh
cd app
go test ./...
```

真实链路集成测试需要本机 ffmpeg：

```sh
MYGO_UKE_SOURCE=/path/song.mp3 go test ./internal/pipeline \
  -run TestImportIntegration -v -count=1
```

真实 ASR 联调：

```sh
MYGO_UKE_VOLCANO_API_KEY=... MYGO_UKE_ASR_SOURCE=/path/speech.aiff \
  go test ./internal/asr -run TestTranscribeLive -v -count=1
```

`TestImportFullIntegration` 还会读取 `MYGO_UKE_CHART_TEXT`；提供文本曲谱时
会验证字幕与曲谱一起生成。

## ASR 行为

- 未配置 Key：跳过转写；有曲谱时按总时长均分曲谱行，仍能得到字幕与和弦。
- 配置 Key：转写失败不会丢曲谱内容，失败原因写入 `project.json` 的
  `warnings`，界面状态显示提示。
- 只接受服务端返回的句级 `result.utterances[]`；没有句子时间不会伪造成整首一条。
- `utterances` 会渐进修订同一句（起始时间相同、结束时间和文本增长），
  客户端按起始时间归组，保留最新最长版本。
- 唱歌场景默认把 `end_window_size` 设为 2000ms；火山默认的 800ms
  会把长句切碎，常见表现是曲谱行开头几个字整段丢失、时间从后半句才开始。
  联调时可用 `MYGO_UKE_ASR_END_WINDOW` 对比不同窗口。
- `project.json` 的 `chartLines` 保存曲谱原文（含未匹配上的行）。旧工程没有
  这个字段时，重新转写会先用工程里的 `chartText` 或 `chart.png` 重建完整曲谱，
  避免早先丢失的首句一直找不回来。
  `chartSnapshot` 标记这份快照来自完整曲谱；没有该标记的旧快照会重建一次。

## 已知限制

- 曲谱没有 BPM 时，小节边界按和弦变化近似，不代表真实重拍。
- 重新转写会写回原工程；字幕一旦人工编辑过，整组字幕保持不动；
  手动选过的和弦按小节时间重叠保留。
