package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/project"
)

func (a *app) openLineEditor(index int) {
	a.lineIndex = -1
	a.lineText = ""
	a.lineStart = a.playhead
	a.lineEnd = a.playhead + 3
	if index >= 0 && index < len(a.proj.Lines) {
		line := a.proj.Lines[index]
		a.lineIndex = index
		a.lineText = line.Text
		a.lineStart = line.Start
		a.lineEnd = line.End
	}
	if a.lineEnd > a.proj.Duration {
		a.lineEnd = a.proj.Duration
	}
	if a.lineEnd <= a.lineStart {
		a.lineEnd = a.lineStart + 1
	}
	a.modal = modalLine
}

func (a *app) nudgeLine(which int, delta float64) {
	if which == 0 {
		a.lineStart = clampLine(a.lineStart+delta, 0, max(0, a.lineEnd-0.1))
		return
	}
	a.lineEnd = clampLine(a.lineEnd+delta, a.lineStart+0.1, max(0, a.proj.Duration))
}

func (a *app) saveLine(c *ui.Context) {
	text := strings.TrimSpace(a.lineText)
	if text == "" {
		c.AddToast(ui.Toast{Title: "字幕为空", Description: "先写一句歌词", Type: "error"})
		return
	}
	start := clampLine(a.lineStart, 0, max(0, a.proj.Duration))
	end := clampLine(a.lineEnd, 0, max(0, a.proj.Duration))
	if end <= start {
		end = min(a.proj.Duration, start+0.5)
	}
	if end <= start {
		c.AddToast(ui.Toast{Title: "时间不对", Description: "结束时间要晚于开始时间", Type: "error"})
		return
	}
	line := project.Line{Start: start, End: end, Text: text}
	a.selectedLine = a.proj.SetLine(a.lineIndex, line)
	a.proj.LinesEdited = true
	a.saveCurrent()
	a.playhead = start
	a.modal = modalNone
	a.toast(c, "字幕已保存", fmt.Sprintf("%s — %s", timeText(start), timeText(end)))
}

func (a *app) deleteLine(c *ui.Context) {
	if !a.proj.DeleteLine(a.lineIndex) {
		a.modal = modalNone
		return
	}
	a.proj.LinesEdited = true
	if a.selectedLine >= len(a.proj.Lines) {
		a.selectedLine = max(0, len(a.proj.Lines)-1)
	}
	a.saveCurrent()
	a.modal = modalNone
	a.toast(c, "字幕已删除", "")
}

func clampLine(value, low, high float64) float64 {
	if high < low {
		high = low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func (a *app) lineModal(c *ui.Context) {
	open := true
	title := "新增字幕"
	if a.lineIndex >= 0 {
		title = fmt.Sprintf("编辑第 %d 句", a.lineIndex+1)
	}
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(500).Gap(12).Children(func() {
			modalTitle(c, title, "句级时间；保存后曲谱、预览和导出同时更新")
			ui.Column(c).Gap(4).Children(func() {
				text(c, "字幕", 10, pal.muted)
				ui.TextInput(c, &a.lineText).Placeholder("输入这一句歌词").Label("字幕").
					FillWidth().AutoFocus()
			})
			lineTimeRow(c, "开始", 0, a.lineStart, a)
			lineTimeRow(c, "结束", 1, a.lineEnd, a)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if a.lineIndex >= 0 {
					ghostButton(c, "删除", func() { a.deleteLine(c) })
				}
				ghostButton(c, "取消", func() { a.modal = modalNone })
				primaryButton(c, "保存字幕", func() { a.saveLine(c) })
			})
		})
	})
	if !open {
		a.modal = modalNone
	}
}

func lineTimeRow(c *ui.Context, name string, which int, value float64, a *app) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 10, 8, 10).
		Radius(7).Background(pal.card).Children(func() {
		text(c, name, 10, pal.muted).Width(42)
		mono(c, fmt.Sprintf("%s · %.1fs", timeText(value), value), 9.5, pal.text)
		ui.Box(c).Grow(1)
		ghostButton(c, "−0.1", func() { a.nudgeLine(which, -0.1) })
		ghostButton(c, "+0.1", func() { a.nudgeLine(which, 0.1) })
	})
}
