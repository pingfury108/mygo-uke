package app

import (
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *app) titlebar(c *ui.Context) {
	bar := c.TitleBar()
	left := float32(16)
	if bar.Left > 0 {
		left = bar.Left + 8
	}
	row := ui.Row(c).Height(52).Padding(0, 16, 0, left).Gap(14).
		AlignItems(ui.Center).Background(pal.bg2)
	if runtime.GOOS != "ios" {
		row = row.DragWindow()
	}
	row.DrawOver(func(p *ui.Painter, r ui.Rect) {
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
		flatButton(c, "重新转写", func() { a.startReanalyze(c) })
		segment(c, []string{"竖屏", "横屏"}, &a.orientation)
		primaryButton(c, "导出成片", a.openExportModal)
	})
}

func railWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	if w < 1280 {
		return 208
	}
	return 234
}

func inspectorWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	if w < 1280 {
		return 296
	}
	return 326
}

func (a *app) rail(c *ui.Context) {
	ui.Column(c).Width(railWidth(c)).Background(pal.bg2).Padding(13, 11, 11, 11).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X+r.W-0.5, r.Y, r.X+r.W-0.5, r.Y+r.H, 1, pal.line)
		}).Children(func() {
		ui.Row(c).Padding(2, 5, 10, 5).AlignItems(ui.Center).Children(func() {
			label(c, "工程", 10, pal.muted, 650)
			ui.Box(c).Grow(1)
		})
		ui.Row(c).Gap(6).Padding(0, 5, 10, 5).Children(func() {
			railAction(c, "+ 新建工程", a.startImport)
		})
		for i := range a.projects {
			index := i
			a.projectRow(c, index)
		}
		ui.Box(c).Grow(1)
		ui.Box(c).Height(1).FillWidth().Background(pal.line)
		ui.Column(c).Padding(10, 0, 0, 0).Children(func() {
			b := ui.ButtonBase(c).FillWidth().Height(22).Padding(0, 2, 0, 2).
				Justify(ui.Start).Label("设置").FocusRing(false)
			if b.Hovered() {
				b.Background(pal.sel)
			}
			b.OnClick(func() { a.openSettings() })
			b.Children(func() {
				text(c, "设置", 10, pal.muted)
			})
		})
	})
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
		a.selectProject(index)
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
			if item.data != nil || item.path != "" {
				iconButton(c, "⋯", "工程设置", func() { a.openProjectModal(index) })
			}
			stateColor := pal.faint
			if selected {
				stateColor = pal.accent
			}
			text(c, item.state, 8.5, stateColor).SingleLine()
		})
	})
}

func (a *app) selectProject(index int) {
	if index < 0 || index >= len(a.projects) {
		return
	}
	a.selected = index
	item := a.projects[index]
	if item.data != nil {
		a.proj = item.data
		a.projPath = item.path
		a.resetCursor()
		a.phoneBar = -1
		a.phoneScroll.Y = 0
		a.openPlayerFor(a.proj.Source)
	} else {
		a.closePlayer()
		a.proj = demoProject()
		a.projPath = ""
		if index > 0 {
			a.proj.Title = item.title
		}
		a.playhead = 42.35
		a.selectedLine = 7
		a.revealLine = 7
		a.revealBar = -1
		a.chordCursor = -1
		a.phoneBar = -1
		a.phoneScroll.Y = 0
		a.resetInspectorScroll()
	}
	a.playing = false
	if a.player != nil {
		a.player.Pause()
	}
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
		if item.data != nil && item.data.ChartName != "" {
			p.Fill(ui.Rect{X: r.X + r.W - 10, Y: r.Y + 6, W: 6, H: 6}, pal.accent, 3)
		}
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
			text(c, a.analysisText(), 9, pal.muted)
		})
		text(c, fmt.Sprintf("字幕 %d 句", len(a.proj.Lines)), 9, pal.muted)
		text(c, fmt.Sprintf("小节 %d · 待确认 %d", len(a.proj.Bars), a.proj.ReviewCount()), 9, pal.muted)
		ui.Box(c).Grow(1)
		text(c, "工程保存在本机", 9, pal.faint)
		mono(c, "v0.1.0", 9, pal.faint)
	})
}

func (a *app) analysisText() string {
	if a.analysisAt.IsZero() {
		return "工程已载入"
	}
	age := time.Since(a.analysisAt)
	switch {
	case age < time.Minute:
		return "转写完成 · 刚刚"
	case age < time.Hour:
		return fmt.Sprintf("转写完成 · %d 分钟前", int(age.Minutes()))
	default:
		return "转写完成 · " + a.analysisAt.Format("01-02 15:04")
	}
}

func (a *app) reviewBars() int {
	if a.proj != nil && a.proj.HasChartChords() {
		return 0
	}
	n := 0
	for _, b := range a.proj.Bars {
		if b.NeedsReview {
			n++
		}
	}
	return n
}
