package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/project"
	"mygo-uke/internal/sheet"
)

func (a *app) openSheetModal() {
	a.sheetMode = sheetForCurrent
	a.sheetErr = ""
	a.sheetNote = ""
	a.sheetTab = 0
	a.resetStagedChart()
	if a.proj != nil && a.proj.ChartImage != "" && a.projPath != "" {
		path := filepath.Join(filepath.Dir(a.projPath), a.proj.ChartImage)
		if _, err := os.Stat(path); err == nil {
			name := a.proj.ChartName
			if strings.TrimSpace(name) == "" {
				name = filepath.Base(path)
			}
			a.stagedChart = chartOrigin{Name: name, Path: path}
			a.stagedPreview = decodeChartImage(a.stagedChart)
			a.sheetNote = "当前曲谱已应用；重新识别会按这张图替换字幕与和弦"
		}
	}
	if a.proj != nil && a.proj.ChartText != "" {
		if strings.TrimSpace(a.sheetPaste) == "" {
			a.sheetPaste = a.proj.ChartText
		}
		a.sheetTab = 1
	}
	a.modal = modalSheet
}

func (a *app) sheetModal(c *ui.Context) {
	open := true
	detail := "自动对齐到当前工程的时间轴，并替换字幕与和弦；导入后仍可逐条修改"
	if a.proj != nil && a.proj.Title != "" {
		detail = fmt.Sprintf("自动对齐到「%s」的时间轴，并替换字幕与和弦；导入后仍可逐条修改", a.proj.Title)
	}
	fraction := float32(0.5)
	if a.stagedChart.Name != "" {
		fraction = 0.8
	}
	a.dialog(c, &open, 560, fraction, func() {
		a.sheetPasteShortcut(c)
		modalTitle(c, "导入曲谱", detail)
		if a.proj == nil {
			text(c, "请先导入视频，曲谱需要对齐到音频时间轴。", 10, pal.warn)
		} else {
			if a.stagedChart.Name == "" {
				a.chartSourceCard(c)
			}
			segment(c, []string{"图片曲谱", "文本曲谱"}, &a.sheetTab)
			if a.sheetTab == 0 {
				a.sheetImagePane(c)
			} else {
				a.sheetTextPane(c)
			}
		}

		if a.sheetRunning {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 10, 8, 10).
				Radius(8).Background(pal.card).Children(func() {
				dot(c, pal.accent)
				text(c, "正在识别曲谱图片，通常需要几秒到半分钟…", 10, pal.muted)
			})
		}
		if a.sheetNote != "" {
			text(c, a.sheetNote, 9.5, pal.good)
		}
		if a.sheetErr != "" {
			ui.Row(c).Padding(8, 10, 8, 10).Radius(8).Background(pal.warnBG).
				AlignItems(ui.Center).Children(func() {
				dot(c, pal.warn)
				text(c, a.sheetErr, 9.5, pal.warn)
			})
		}

		ui.Row(c).Justify(ui.End).Children(func() {
			closeLabel := "取消"
			if a.sheetNote != "" {
				closeLabel = "完成"
			}
			if a.sheetRunning {
				closeLabel = "取消识别"
			}
			ghostButton(c, closeLabel, a.closeSheet)
		})
	})
	if !open {
		a.cancelSheet()
		a.modal = modalNone
		a.sheetSeq++
	}
}

func (a *app) sheetImagePane(c *ui.Context) {
	if a.stagedChart.Name != "" {
		a.sheetStagedImage(c)
		return
	}
	if !a.settings.sheetConfig().Ready() {
		ui.Row(c).Padding(10, 12, 10, 12).Radius(8).Background(pal.warnBG).
			AlignItems(ui.Center).Children(func() {
			dot(c, pal.warn)
			text(c, "图片识别还不能用，需要先在设置里配置。", 10, pal.warn)
		})
		ui.Row(c).Gap(8).Children(func() {
			ghostButton(c, "使用文本曲谱", func() { a.sheetTab = 1 })
			primaryButton(c, "去设置", func() {
				a.closeSheet()
				a.openSettings()
			})
		})
		return
	}
	if a.sheetRunning {
		return
	}

	zoneLabel := "拖入曲谱图片，或点击选择"
	zone := ui.ButtonBase(c).FillWidth().Height(104).Radius(10).Center().
		Background(pal.card).Border(1, pal.lineStrong).Label("选择曲谱图片").FocusRing(false)
	if zone.Hovered() {
		zone.Background(pal.sel)
	}
	zone.OnClick(func() { a.pickSheetImage() })
	zone.Children(func() {
		ui.Column(c).Gap(5).Children(func() {
			label(c, zoneLabel, 11.5, pal.text, 600)
			text(c, "PNG / JPG / WebP · 也可以按 ⌘V 粘贴", 9, pal.muted)
		})
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ghostButton(c, "从剪贴板粘贴", a.pasteSheetClipboard)
		text(c, "截图或复制的图片", 8.5, pal.faint).SingleLine()
	})
}

func (a *app) sheetStagedImage(c *ui.Context) {
	ui.Column(c).Gap(7).Padding(8, 9, 8, 9).Radius(10).
		Background(pal.card).Border(1, pal.line).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			label(c, "待识别的曲谱", 9.5, pal.muted, 600)
			ui.Box(c).Grow(1)
			mono(c, a.stagedChart.display(), 8.5, pal.faint).SingleLine()
		})
		box := ui.Box(c).FillWidth().Height(168).Radius(8).
			Background(pal.bg2).Clip()
		if a.stagedPreview != nil {
			box.Children(func() {
				ui.Image(c, a.stagedPreview).Fill().Fit(ui.Contain)
			})
		} else {
			box.Center().Children(func() {
				text(c, "图片无法预览，仍可直接识别", 9.5, pal.faint)
			})
		}
	})
	if a.sheetRunning {
		return
	}
	if !a.settings.sheetConfig().Ready() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ghostButton(c, "更换图片", a.pickSheetImage)
			primaryButton(c, "去设置", func() {
				a.closeSheet()
				a.openSettings()
			})
		})
		text(c, "识别前需要先在设置里配置支持图片输入的模型。", 8.5, pal.faint)
		return
	}
	action := "开始识别"
	if a.sheetNote != "" {
		action = "重新识别"
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ghostButton(c, "更换图片", a.pickSheetImage)
		primaryButton(c, action, a.parseStagedSheetImage)
	})
	text(c, "确认图片无误后再识别；识别会替换当前字幕与和弦。", 8.5, pal.faint)
}

func (a *app) sheetTextPane(c *ui.Context) {
	action := "生成字幕与和弦"
	if a.sheetMode == sheetForNew {
		action = "解析曲谱"
	}
	ui.Column(c).Gap(6).Children(func() {
		ui.TextArea(c, &a.sheetPaste).Label("曲谱文本").
			Placeholder("BPM 90\n4/4\nF  G  Em7  Am\n好想能看到 你嘴角微笑\n\nF  G  C\n最好在下一秒").
			Height(150).FillWidth()
		text(c, "每两行一组：第一行和弦，第二行歌词；空行会自动忽略。", 9, pal.faint)
		ui.Row(c).Justify(ui.End).Children(func() {
			primaryButton(c, action, a.applySheetText)
		})
	})
}

func (a *app) closeSheet() {
	a.cancelSheet()
	a.modal = modalNone
	a.sheetSeq++
}

func (a *app) cancelSheet() {
	if a.sheetCancel != nil {
		a.sheetCancel()
		a.sheetCancel = nil
	}
}

type chartOrigin struct {
	Name string
	Path string
	Data []byte
	Text string
}

func (o chartOrigin) display() string {
	if o.Name != "" {
		return o.Name
	}
	return "曲谱"
}

// storeChart keeps the source chart with the project so the editor can show
// what produced the current timeline. A failure here is a warning, not a
// reason to throw away a successfully parsed chart.
func (a *app) storeChart(origin chartOrigin) string {
	return a.storeChartFor(a.proj, a.projPath, origin)
}

func (a *app) storeChartFor(p *project.Project, path string, origin chartOrigin) string {
	if p == nil {
		return ""
	}
	a.resetChartThumb()
	p.ChartName = origin.display()
	p.ChartAt = time.Now().Format(time.RFC3339)
	if origin.Text != "" {
		p.ChartText = origin.Text
		p.ChartImage = ""
		if path != "" {
			a.removeChartFiles(filepath.Dir(path), "")
		}
		return ""
	}
	p.ChartText = ""
	if path == "" {
		p.ChartImage = ""
		return "曲谱已应用，但工程还没有保存位置，图片没有归档"
	}
	data := origin.Data
	if len(data) == 0 && origin.Path != "" {
		var err error
		data, err = os.ReadFile(origin.Path)
		if err != nil {
			p.ChartImage = ""
			return "曲谱已应用，但读取图片失败：" + err.Error()
		}
	}
	if len(data) == 0 {
		p.ChartImage = ""
		return "曲谱已应用，但图片为空，没有归档"
	}

	name := "chart.png"
	if origin.Data == nil && origin.Path != "" {
		if ext := strings.ToLower(filepath.Ext(origin.Path)); ext != "" {
			switch ext {
			case ".png", ".jpg", ".jpeg", ".webp":
				name = "chart" + ext
			}
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		p.ChartImage = ""
		return "曲谱已应用，但无法创建工程目录：" + err.Error()
	}
	tmp := filepath.Join(dir, name+".tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		p.ChartImage = ""
		return "曲谱已应用，但保存图片失败：" + err.Error()
	}
	if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
		_ = os.Remove(tmp)
		p.ChartImage = ""
		return "曲谱已应用，但保存图片失败：" + err.Error()
	}
	a.removeChartFiles(dir, name)
	p.ChartImage = name
	return ""
}

func (a *app) removeChartFiles(dir, keep string) {
	if dir == "" {
		return
	}
	for _, name := range []string{"chart.png", "chart.jpg", "chart.jpeg", "chart.webp"} {
		if name == keep {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}

func (a *app) chartPath() string {
	if a.proj == nil || a.projPath == "" || a.proj.ChartImage == "" {
		return ""
	}
	name := filepath.Base(a.proj.ChartImage)
	if name != a.proj.ChartImage {
		return ""
	}
	return filepath.Join(filepath.Dir(a.projPath), name)
}

func (a *app) resetChartThumb() {
	a.chartThumb = nil
	a.chartThumbKey = ""
}

func (a *app) chartThumbFor(path string) ui.ImageSource {
	key := path + "|" + a.proj.ChartAt
	if a.chartThumbKey == key {
		return a.chartThumb
	}
	a.chartThumbKey = key
	a.chartThumb = nil
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	src, err := ui.DecodeImage(data)
	if err != nil {
		return nil
	}
	a.chartThumb = src
	return src
}

func (a *app) chartSourceCard(c *ui.Context) {
	if a.proj == nil || a.proj.ChartName == "" {
		return
	}
	path := a.chartPath()
	ui.Row(c).Padding(8, 10, 8, 10).Radius(8).Background(pal.card).
		AlignItems(ui.Center).Gap(10).Children(func() {
		if path != "" {
			if src := a.chartThumbFor(path); src != nil {
				ui.Box(c).Size(56, 40).Radius(5).Clip().Background(pal.bg).Children(func() {
					ui.Image(c, src).Size(56, 40)
				})
			} else {
				ui.Box(c).Size(56, 40).Radius(5).Background(pal.bg2).Center().Children(func() {
					text(c, "谱", 12, pal.faint)
				})
			}
		} else {
			ui.Box(c).Size(56, 40).Radius(5).Background(pal.bg2).Center().Children(func() {
				text(c, "谱", 12, pal.faint)
			})
		}
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				text(c, "当前曲谱", 9, pal.muted)
				label(c, a.proj.ChartName, 10, pal.text, 600).SingleLine()
			})
			mono(c, chartSourceDetail(a.proj), 8.5, pal.faint).SingleLine()
		})
		if path != "" {
			ghostButton(c, "预览", func() {
				if err := mygo.Shell.PreviewItem(path); err != nil {
					a.sheetErr = "无法预览：" + err.Error()
					a.invalidate()
				}
			})
		}
	})
}

func (a *app) chartChip(c *ui.Context) {
	if a.proj == nil {
		return
	}
	labelText := "+ 添加曲谱"
	color := pal.muted
	if a.proj.ChartName != "" {
		labelText = "曲谱 · " + shortChartName(a.proj.ChartName)
		color = pal.accent
	}
	b := ui.ButtonBase(c).Height(20).PaddingX(7).Radius(5).
		Background(pal.card).Border(1, pal.line).Label(labelText).FocusRing(false)
	if b.Hovered() {
		b.Background(pal.sel)
	}
	b.OnClick(a.openSheetModal)
	b.Children(func() {
		text(c, labelText, 9, color).SingleLine()
	})
}

func shortChartName(name string) string {
	runes := []rune(name)
	if len(runes) <= 14 {
		return name
	}
	return string(runes[:13]) + "…"
}

func chartSourceDetail(p *project.Project) string {
	if p.ChartText != "" {
		lines := len(strings.Split(strings.TrimSpace(p.ChartText), "\n"))
		return fmt.Sprintf("粘贴文本 · %d 行 · %s", lines, chartTime(p.ChartAt))
	}
	return "图片 · " + chartTime(p.ChartAt)
}

func chartTime(at string) string {
	if at == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, at); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return at
}

func (a *app) pickSheetImage() {
	if a.sheetRunning {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win,
			Title:  "导入曲谱图片",
			Filters: []mygo.FileFilter{{
				Name:       "曲谱图片",
				Extensions: []string{"png", "jpg", "jpeg", "webp"},
			}},
		})
		if err != nil {
			mygo.RunOnMain(func() {
				a.sheetErr = err.Error()
				a.invalidate()
			})
			return
		}
		if len(paths) == 0 {
			return
		}
		mygo.RunOnMain(func() { a.stageSheetImage(paths[0]) })
	}()
}

func (a *app) stageSheetImage(path string) {
	a.stageChartImage(chartOrigin{Name: filepath.Base(path), Path: path})
}

func (a *app) stageSheetImageData(data []byte) {
	a.stageChartImage(chartOrigin{Name: "剪贴板图片", Data: data})
}

func (a *app) stageChartImage(origin chartOrigin) {
	if a.sheetRunning {
		a.sheetErr = "图片还在识别，请稍候"
		a.invalidate()
		return
	}
	a.sheetTab = 0
	a.stagedChart = origin
	a.stagedPreview = decodeChartImage(origin)
	a.sheetErr = ""
	a.sheetNote = ""
	if a.sheetMode == sheetForNew {
		a.newChart = nil
		a.newChartOrigin = chartOrigin{}
	}
	a.setSheetModal()
	a.invalidate()
}

func (a *app) resetStagedChart() {
	a.stagedChart = chartOrigin{}
	a.stagedPreview = nil
}

func decodeChartImage(origin chartOrigin) ui.ImageSource {
	data := origin.Data
	if len(data) == 0 && origin.Path != "" {
		data, _ = os.ReadFile(origin.Path)
	}
	if len(data) == 0 {
		return nil
	}
	src, err := ui.DecodeImage(data)
	if err != nil {
		return nil
	}
	return src
}

func (a *app) parseStagedSheetImage() {
	origin := a.stagedChart
	if origin.Name == "" {
		a.sheetErr = "先粘贴或选择一张曲谱图片"
		a.invalidate()
		return
	}
	if len(origin.Data) > 0 {
		data := origin.Data
		a.startChartParse(origin, func(ctx context.Context, client *sheet.Client) (*sheet.Sheet, error) {
			return client.ParseImageData(ctx, data, "image/png")
		})
		return
	}
	path := origin.Path
	a.startChartParse(origin, func(ctx context.Context, client *sheet.Client) (*sheet.Sheet, error) {
		return client.ParseImage(ctx, path)
	})
}

func (a *app) startChartParse(origin chartOrigin, parse func(context.Context, *sheet.Client) (*sheet.Sheet, error)) {
	if a.sheetRunning {
		return
	}
	if a.sheetMode == sheetForCurrent && (a.proj == nil || a.proj.Duration <= 0) {
		a.sheetErr = "请先导入视频，曲谱需要对齐到音频时间轴"
		a.setSheetModal()
		a.invalidate()
		return
	}
	cfg := a.settings.sheetConfig()
	if !cfg.Ready() {
		a.sheetErr = "图片识别还没有配置，请到「设置」完成配置"
		a.setSheetModal()
		a.invalidate()
		return
	}

	a.sheetRunning = true
	a.sheetErr = ""
	a.sheetNote = ""
	a.setSheetModal()
	a.sheetSeq++
	seq := a.sheetSeq
	a.invalidate()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	a.sheetCancel = cancel
	go func() {
		defer cancel()
		client, err := sheet.New(ctx, cfg)
		var parsed *sheet.Sheet
		if err == nil {
			parsed, err = parse(ctx, client)
		}
		a.sheetDispatch(func() {
			if seq != a.sheetSeq {
				return
			}
			a.sheetCancel = nil
			a.sheetRunning = false
			if err != nil {
				a.sheetErr = err.Error()
				a.invalidate()
				return
			}
			if a.sheetMode == sheetForNew {
				a.newChart = parsed
				a.newChartOrigin = origin
				a.sheetNote = fmt.Sprintf("已识别 %d 行、%d 个和弦；创建工程时自动对齐",
					len(parsed.Lines), chartChordCount(parsed))
				a.invalidate()
				return
			}
			a.applySheet(parsed, origin)
			a.invalidate()
		})
	}()
}

func (a *app) pasteSheetClipboard() {
	if a.sheetRunning {
		a.sheetErr = "图片还在识别，请稍候"
		a.invalidate()
		return
	}
	if data := mygo.Clipboard.ReadImage(); len(data) > 0 {
		a.stageSheetImageData(data)
		return
	}
	if files, err := mygo.Clipboard.ReadFiles(); err == nil {
		for _, path := range files {
			if isSheetImage(path) {
				a.stageSheetImage(path)
				return
			}
		}
	}
	if text := strings.TrimSpace(mygo.Clipboard.ReadText()); text != "" {
		a.sheetTab = 1
		a.resetStagedChart()
		a.sheetPaste = text
		a.sheetErr = ""
		a.sheetNote = "已粘贴文本曲谱；确认后点「生成字幕与和弦」"
		a.invalidate()
		return
	}
	a.sheetErr = "剪贴板里没有图片或曲谱文本"
	a.invalidate()
}

// sheetPasteShortcut is declared from inside the chart dialogs: a modal
// holds the focus and the window's shortcuts built behind it wait.
func (a *app) sheetPasteShortcut(c *ui.Context) {
	if c.FocusTakesText() {
		return
	}
	c.OnWindowShortcut(ui.Cmd, ui.KeyV, a.pasteSheetClipboard)
}

func (a *app) applySheetText() {
	if a.sheetRunning {
		a.sheetErr = "图片还在识别，请稍候再生成"
		return
	}
	text := strings.TrimSpace(a.sheetPaste)
	if text == "" {
		a.sheetErr = "先粘贴一段文本曲谱：和弦行在上、歌词行在下"
		return
	}
	parsed, err := sheet.ParseText(text)
	if err != nil {
		a.sheetErr = err.Error()
		return
	}
	if a.sheetMode == sheetForNew {
		a.resetStagedChart()
		a.newChart = parsed
		a.newChartOrigin = chartOrigin{Name: "粘贴文本", Text: text}
		a.sheetNote = fmt.Sprintf("已解析 %d 行、%d 个和弦；创建工程时自动对齐",
			len(parsed.Lines), chartChordCount(parsed))
		a.sheetErr = ""
		a.invalidate()
		return
	}
	a.resetStagedChart()
	a.applySheet(parsed, chartOrigin{Name: "粘贴文本", Text: text})
}

func (a *app) setSheetModal() {
	if a.sheetMode == sheetForNew {
		a.modal = modalNewProject
		return
	}
	a.modal = modalSheet
}

func chartChordCount(s *sheet.Sheet) int {
	if s == nil {
		return 0
	}
	count := 0
	for _, line := range s.Lines {
		count += len(line.Chords)
	}
	return count
}

func (a *app) applySheet(parsed *sheet.Sheet, origin chartOrigin) {
	if a.proj == nil || a.proj.Duration <= 0 {
		a.sheetErr = "请先导入视频，曲谱需要对齐到音频时间轴"
		return
	}
	sheet.Apply(a.proj, parsed)
	warning := a.storeChart(origin)
	if a.projPath != "" {
		if err := project.Save(a.projPath, a.proj); err != nil {
			a.sheetErr = "保存曲谱失败：" + err.Error()
			return
		}
	}
	if a.selected >= 0 && a.selected < len(a.projects) {
		tone := a.projects[a.selected].tone
		a.projects[a.selected] = a.projectItem(a.proj, a.projPath, tone)
	}
	a.resetCursor()
	a.sheetErr = warning
	a.sheetPaste = ""
	suffix := ""
	if a.projPath != "" {
		suffix = "，并保存到工程"
	}
	a.sheetNote = fmt.Sprintf("已从%s生成 %d 行字幕、%d 个和弦%s",
		origin.display(), len(a.proj.Lines), len(a.proj.Chords), suffix)
}

func isSheetImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	}
	return false
}
