package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (a *app) titlebar(c *ui.Context) {
	bar := c.TitleBar()
	left := float32(16)
	if bar.Left > 0 {
		left = bar.Left + 8
	}
	ui.Row(c).Height(52).Padding(0, 16, 0, left).Gap(14).
		AlignItems(ui.Center).Background(pal.bg2).DragWindow().
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X, r.Y+r.H-0.5, r.X+r.W, r.Y+r.H-0.5, 1, pal.line)
		}).Children(func() {
		ui.Box(c).Size(22, 22).Radius(6).Background(pal.accent).Center().Children(func() {
			label(c, "M", 11, pal.accentInk, 800)
		})
		label(c, "MyGo Uke", 12, pal.text, 650)
		text(c, "/", 11, pal.faint)
		text(c, a.proj.Title, 12, pal.muted)
		ui.Row(c).Padding(2, 7, 2, 7).Radius(999).Background(pal.warnBG).
			AlignItems(ui.Center).Gap(4).Children(func() {
			dot(c, pal.good)
			text(c, "已保存", 9, pal.muted)
		})
		ui.Box(c).Grow(1)
		flatButton(c, "重新分析", func() { a.startReanalyze(c) })
		segment(c, []string{"竖屏", "横屏"}, &a.orientation)
		primaryButton(c, "导出成片", func() {
			a.exportProgress = 0
			a.exportErr = ""
			a.exportPath = ""
			a.exportNote = ""
			a.modal = modalExport
		})
	})
}

func (a *app) rail(c *ui.Context) {
	ui.Column(c).Width(234).Background(pal.bg2).Padding(13, 11, 11, 11).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X+r.W-0.5, r.Y, r.X+r.W-0.5, r.Y+r.H, 1, pal.line)
		}).Children(func() {
		ui.Row(c).Padding(2, 5, 10, 5).AlignItems(ui.Center).Children(func() {
			label(c, "工程", 10, pal.muted, 650)
			ui.Box(c).Grow(1)
			iconButton(c, "+", "导入视频", a.startImport)
		})
		for i := range a.projects {
			index := i
			a.projectRow(c, index)
		}
		ui.Box(c).Grow(1)
		ui.Box(c).Height(1).FillWidth().Background(pal.line)
		ui.Column(c).Padding(10, 5, 0, 5).Gap(8).Children(func() {
			a.chorddState(c)
			apiState(c, pal.warn, fmt.Sprintf("%d 个和弦待确认", a.reviewBars()))
			b := ui.ButtonBase(c).Height(22).Padding(0).Label("设置与 API Key").FocusRing(false)
			if b.Hovered() {
				b.Background(pal.sel)
			}
			b.OnClick(func() { a.toast(c, "设置", "API Key 与输出目录将在设置页配置") })
			b.Children(func() {
				text(c, "设置与 API Key", 10, pal.muted)
			})
		})
	})
}

func (a *app) chorddState(c *ui.Context) {
	color := pal.faint
	value := "chordd 检查中…"
	switch {
	case !a.chorddChecked:
	case a.chorddOnline:
		color = pal.good
		value = "chordd 在线 · " + a.chorddAnalyzer
	default:
		color = pal.warn
		value = "chordd 离线 · 自动重试"
	}
	apiState(c, color, value)
}

func (a *app) projectRow(c *ui.Context, index int) {
	item := a.projects[index]
	selected := index == a.selected
	row := ui.ButtonBase(c).FillWidth().Padding(7).Radius(9).Label(item.title).FocusRing(false)
	if selected {
		row.Background(pal.sel).Border(1, pal.lineStrong)
	} else if row.Hovered() {
		row.Background(pal.sel)
	}
	row.OnClick(func() {
		a.selected = index
		if item.data != nil {
			a.proj = item.data
			a.resetCursor()
		} else if index == 0 {
			a.proj = demoProject()
			a.playhead = 42
			a.selectedLine = 6
		} else {
			a.proj = demoProject()
			a.proj.Title = item.title
			a.playhead = 42
		}
		a.playing = false
	})
	row.DrawOver(func(p *ui.Painter, r ui.Rect) {
		if selected {
			p.Fill(ui.Rect{X: r.X, Y: r.Y + 7, W: 2, H: r.H - 14}, ui.RGBA(243, 181, 68, 0.9), 1)
		}
	})
	row.Children(func() {
		ui.Row(c).FillWidth().Gap(9).AlignItems(ui.Center).Children(func() {
			projectThumb(c, item)
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				label(c, item.title, 11.5, pal.text, 600)
				mono(c, item.meta, 9, pal.muted)
			})
			stateColor := pal.faint
			if selected {
				stateColor = pal.accent
			}
			text(c, item.state, 8.5, stateColor).SingleLine()
		})
	})
}

func projectThumb(c *ui.Context, item projectItem) {
	from := ui.Hex("#4B3C2C")
	to := ui.Hex("#1C1915")
	chord := item.chord
	switch item.tone {
	case 1:
		from, to = ui.Hex("#344238"), ui.Hex("#151A16")
	case 2:
		from, to = ui.Hex("#2C3945"), ui.Hex("#14181C")
	}
	ui.Box(c).Size(40, 40).Radius(7).Clip().LinearGradient(ui.LinearGradient{
		From: from, To: to, Angle: 155,
	}).Draw(func(p *ui.Painter, r ui.Rect) {
		p.FillRadialGradient(r, ui.RadialGradient{
			Stops: []ui.GradientStop{
				{Color: ui.RGBA(255, 196, 112, 0.35), At: 0},
				{Color: ui.RGBA(255, 196, 112, 0), At: 1},
			},
			Center: ui.Point{X: 0.7, Y: 0.18},
		}, 7)
		p.Text(r.X+r.W-19, r.Y+r.H-5, chord, 9, ui.RGBA(255, 248, 232, 0.82))
	})
}

func apiState(c *ui.Context, color ui.Color, value string) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		dot(c, color)
		text(c, value, 9, pal.muted).SingleLine()
	})
}

func (a *app) statusbar(c *ui.Context) {
	ui.Row(c).Height(30).Padding(0, 14, 0, 14).Gap(16).
		AlignItems(ui.Center).Background(pal.bg2).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X, r.Y+0.5, r.X+r.W, r.Y+0.5, 1, pal.line)
		}).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			dot(c, pal.good)
			text(c, "分析完成 · 3 秒前", 9, pal.muted)
		})
		text(c, fmt.Sprintf("字幕 %d 句", len(a.proj.Lines)), 9, pal.muted)
		text(c, fmt.Sprintf("小节 %d · 待确认 %d", len(a.proj.Bars), a.proj.ReviewCount()), 9, pal.muted)
		ui.Box(c).Grow(1)
		text(c, "工程保存在本机", 9, pal.faint)
		mono(c, "v0.1.0", 9, pal.faint)
	})
}

func (a *app) reviewBars() int {
	n := 0
	for _, b := range a.proj.Bars {
		if b.NeedsReview {
			n++
		}
	}
	return n
}
