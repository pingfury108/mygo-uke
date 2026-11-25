package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
	"mygo-uke/internal/uke"
)

const (
	phoneEditTab = iota
	phoneLyricsTab
	phoneChordsTab
	phoneProjectTab
)

// phone is the single-column shell used by phones and by tablets in portrait.
// It keeps the same project, player and editing actions as the workspace; only
// the composition changes.
func (a *app) phone(c *ui.Context) {
	if a.phoneTab < phoneEditTab || a.phoneTab > phoneProjectTab {
		a.phoneTab = phoneEditTab
	}
	ui.Column(c).Fill().Background(pal.bg).Children(func() {
		a.phoneTopBar(c)
		ui.Column(c).Grow(1).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).FillHeight().Width(phoneContentWidth(c)).Children(func() {
				switch a.phoneTab {
				case phoneLyricsTab:
					a.phoneLyrics(c)
				case phoneChordsTab:
					a.phoneChords(c)
				case phoneProjectTab:
					a.phoneProject(c)
				default:
					a.phoneEdit(c)
				}
			})
		})
		a.phoneTabBar(c)
	})
}

func phoneContentWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	if w > 760 {
		return 720
	}
	if w > 560 {
		return 560
	}
	return w
}

func (a *app) phoneTopBar(c *ui.Context) {
	title := a.proj.Title
	eyebrow := "EDITING"
	switch a.phoneTab {
	case phoneLyricsTab:
		title = "歌词"
		eyebrow = "TIMELINE"
	case phoneChordsTab:
		title = "和弦"
		eyebrow = "CHART"
	case phoneProjectTab:
		title = "工程"
		eyebrow = "LIBRARY"
	}
	ui.Row(c).Height(52).FillWidth().Padding(0, 14, 0, 14).Gap(10).
		AlignItems(ui.Center).Background(pal.bg).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X, r.Y+r.H-0.5, r.X+r.W, r.Y+r.H-0.5, 1, pal.line)
		}).Children(func() {
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			mono(c, eyebrow, 7.5, pal.accent)
			label(c, title, 16, pal.text, 650).SingleLine()
		})
		switch a.phoneTab {
		case phoneLyricsTab:
			a.phoneAction(c, "+", "添加一句", func() { a.openLineEditor(-1) })
		case phoneChordsTab:
			a.phoneAction(c, "谱", "曲谱", a.openSheetModal)
		case phoneProjectTab:
			a.phoneAction(c, "+", "新建工程", a.openNewProject)
		default:
			if a.proj.ChartName != "" || a.proj.ChartText != "" {
				a.phoneAction(c, "谱", "曲谱", a.openSheetModal)
			}
			a.phonePrimary(c, "导出", a.openExportModal)
		}
		a.phoneAction(c, "⋯", "设置", a.openSettings)
	})
}

func (a *app) phoneAction(c *ui.Context, glyph, labelText string, onClick func()) ui.Element {
	b := ui.ButtonBase(c).Size(34, 34).Center().Radius(8).Label(labelText).FocusRing(false)
	b.Background(pal.card).Border(1, pal.line)
	if b.Hovered() {
		b.Background(pal.sel)
	}
	b.OnClick(onClick)
	b.Children(func() {
		text(c, glyph, 14, pal.muted)
	})
	return b
}

func (a *app) phonePrimary(c *ui.Context, value string, onClick func()) ui.Element {
	b := ui.ButtonBase(c).Height(32).PaddingX(12).Center().Radius(8).
		Background(pal.accent).Label(value).FocusRing(false)
	b.OnClick(onClick)
	b.Children(func() {
		label(c, value, 11, pal.accentInk, 650)
	})
	return b
}

func (a *app) phoneTabBar(c *ui.Context) {
	_, _, bottom, _ := c.SafeArea()
	items := []struct {
		name string
	}{
		{"编辑"},
		{"歌词"},
		{"和弦"},
		{"工程"},
	}
	ui.Row(c).Height(56+bottom).FillWidth().Padding(0, 0, bottom, 0).
		Margin(0, 0, -bottom, 0).Background(pal.panel).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X, r.Y+0.5, r.X+r.W, r.Y+0.5, 1, pal.line)
		}).Children(func() {
		for i, item := range items {
			index := i
			on := a.phoneTab == index
			b := ui.ButtonBase(c).Grow(1).Height(56).Center().Label(item.name).FocusRing(false)
			b.OnClick(func() { a.phoneTab = index })
			b.Children(func() {
				color := pal.muted
				weight := 500
				if on {
					color = pal.accent
					weight = 650
				}
				ui.Column(c).Gap(2).AlignItems(ui.Center).Children(func() {
					phoneTabIcon(c, index, color)
					label(c, item.name, 9, color, weight)
				})
			})
		}
	})
}

func phoneTabIcon(c *ui.Context, kind int, color ui.Color) ui.Element {
	box := ui.Box(c).Size(18, 18)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		switch kind {
		case phoneEditTab:
			p.Line(r.X+4, r.Y+14, r.X+13, r.Y+5, 1.6, color)
			p.Line(r.X+11, r.Y+3, r.X+15, r.Y+7, 1.6, color)
			p.Line(r.X+3, r.Y+15, r.X+7, r.Y+14, 1.2, color)
		case phoneLyricsTab:
			for i := 0; i < 3; i++ {
				y := r.Y + 5 + float32(i)*4
				p.Line(r.X+2, y, r.X+16, y, 1.4, color)
			}
		case phoneChordsTab:
			for i := 0; i < 4; i++ {
				x := r.X + 4 + float32(i)*3.4
				p.Line(x, r.Y+3, x, r.Y+15, 1, color)
			}
			for i := 0; i < 3; i++ {
				y := r.Y + 6 + float32(i)*4
				p.Line(r.X+4, y, r.X+14, y, 1, color)
			}
		default:
			p.Stroke(ui.Rect{X: r.X + 2, Y: r.Y + 6, W: 14, H: 9}, color, 2, 1.4)
			p.Line(r.X+2, r.Y+6, r.X+8, r.Y+6, 1.4, color)
			p.Line(r.X+8, r.Y+6, r.X+10, r.Y+4, 1.4, color)
			p.Line(r.X+10, r.Y+4, r.X+15, r.Y+4, 1.4, color)
		}
	})
	return box
}

func (a *app) phoneEdit(c *ui.Context) {
	ui.Column(c).Fill().Padding(10, 12, 8, 12).Gap(8).Children(func() {
		ui.Row(c).Height(24).AlignItems(ui.Center).Gap(8).Children(func() {
			mono(c, metaLine(a.proj), 8.5, pal.muted).SingleLine()
			ui.Box(c).Grow(1)
			if a.proj.HasChartChords() {
				mono(c, "曲谱和弦", 8.5, pal.faint)
			} else {
				mono(c, fmt.Sprintf("%.0f%%", a.avgConfidence()*100), 8.5, pal.faint)
			}
		})
		a.phonePreview(c)
		a.phoneTransport(c)
	})
}

func (a *app) phonePreview(c *ui.Context) {
	w, h := c.Size()
	availableH := h - 52 - 56 - 24 - 104 - 26
	frameH := availableH
	if frameH < 120 {
		frameH = 120
	}
	frameW := frameH * 9 / 16
	if maxW := w - 26; frameW > maxW {
		frameW = maxW
		frameH = frameW * 16 / 9
	}
	ui.Box(c).Grow(1).Center().Background(pal.mat).Children(func() {
		frame := a.videoFrame(c, frameW, frameH)
		frame.OnClick(a.togglePlay)
	})
}

func (a *app) phoneTransport(c *ui.Context) {
	fraction := 0.0
	if a.proj.Duration > 0 {
		fraction = a.playhead / a.proj.Duration
	}
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		bar := ui.Box(c).Height(18).FillWidth().Label("播放进度")
		bar.DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H/2 - 1, W: r.W, H: 2}, pal.lineStrong, 1)
			p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H/2 - 1, W: r.W * float32(fraction), H: 2}, pal.accent, 1)
			x := r.X + r.W*float32(fraction)
			p.Fill(ui.Rect{X: x - 3, Y: r.Y + r.H/2 - 3, W: 6, H: 6}, pal.accent, 3)
		})
		if bar.Clicked() || bar.Pressed() {
			if x, _, ok := bar.PointerPosition(); ok {
				bounds := bar.Bounds()
				if bounds.W > 0 {
					a.seekTo(float64(x/bounds.W) * a.proj.Duration)
				}
			}
		}
		ui.Row(c).Height(48).AlignItems(ui.Center).Gap(8).Children(func() {
			mono(c, timeText(a.playhead), 11, pal.text)
			mono(c, "/ "+timeText(a.proj.Duration), 8, pal.faint)
			ui.Box(c).Grow(1)
			a.phoneSeek(c, -5)
			a.phonePlay(c)
			a.phoneSeek(c, 5)
			ui.Box(c).Width(8)
			value := "静音"
			if a.muted {
				value = "有声"
			}
			a.phoneAction(c, "♪", value, func() { a.setMuted(!a.muted) })
		})
	})
}

func (a *app) phoneSeek(c *ui.Context, delta int) ui.Element {
	value := fmt.Sprintf("%+d", delta)
	b := ui.ButtonBase(c).Size(38, 38).Radius(999).Center().
		Label(value).Border(1, pal.lineStrong).Background(pal.card).FocusRing(false)
	b.OnClick(func() { a.seek(float64(delta)) })
	b.Children(func() {
		mono(c, value, 9, pal.muted)
	})
	return b
}

func (a *app) phonePlay(c *ui.Context) ui.Element {
	glyph := "▶"
	if a.playing {
		glyph = "❚❚"
	}
	b := ui.ButtonBase(c).Size(48, 48).Radius(999).Center().FocusRing(false).
		Label("播放").Background(pal.accent)
	b.OnClick(a.togglePlay)
	b.Children(func() {
		text(c, glyph, 14, pal.accentInk)
	})
	return b
}

func (a *app) phoneLyrics(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(36).Padding(0, 14, 0, 14).AlignItems(ui.Center).Gap(8).Children(func() {
			mono(c, fmt.Sprintf("%d 句", len(a.proj.Lines)), 9, pal.muted)
			if a.reviewBars() > 0 {
				dot(c, pal.warn)
				text(c, fmt.Sprintf("%d 处待确认", a.reviewBars()), 9, pal.muted)
			}
			ui.Box(c).Grow(1)
			text(c, "点一句跳到那一拍", 8.5, pal.faint)
		})
		ui.Box(c).Height(1).FillWidth().Background(pal.line)
		scroll := ui.Scroll(c).Grow(1)
		scroll.Children(func() {
			ui.Column(c).FillWidth().Padding(6, 12, 8, 12).Gap(4).Children(func() {
				if len(a.proj.Lines) == 0 {
					ui.Row(c).Padding(10, 12, 10, 12).Radius(9).Background(pal.card).Children(func() {
						text(c, "还没有字幕。点「+」手填，或在导入时配置转写。", 10, pal.faint)
					})
				}
				for i := range a.proj.Lines {
					a.phoneLineRow(c, i)
				}
			})
		})
		ui.Row(c).Height(58).Padding(0, 12, 0, 12).Gap(8).AlignItems(ui.Center).
			Background(pal.bg2).DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X, r.Y+0.5, r.X+r.W, r.Y+0.5, 1, pal.line)
		}).Children(func() {
			button := ui.ButtonBase(c).Grow(1).Height(38).Center().Radius(9).
				Background(pal.accent).Label("编辑当前句").FocusRing(false)
			if a.selectedLine < 0 || a.selectedLine >= len(a.proj.Lines) {
				button.Background(pal.card)
			}
			button.OnClick(func() { a.openLineEditor(a.selectedLine) })
			button.Children(func() {
				color := pal.accentInk
				if a.selectedLine < 0 || a.selectedLine >= len(a.proj.Lines) {
					color = pal.faint
				}
				label(c, "编辑当前句", 11, color, 650)
			})
			a.phoneAction(c, "+", "添加一句", func() { a.openLineEditor(-1) })
		})
	})
}

func (a *app) phoneLineRow(c *ui.Context, index int) {
	line := a.proj.Lines[index]
	selected := index == a.selectedLine
	row := ui.ButtonBase(c).FillWidth().MinHeight(62).Padding(7, 9, 7, 9).Radius(10).
		AlignItems(ui.Center).Label(line.Text).FocusRing(false)
	if selected {
		row.Background(pal.sel).Border(1, pal.lineStrong)
	} else {
		row.Background(pal.card)
	}
	row.OnClick(func() {
		a.selectedLine = index
		a.seekTo(line.Start)
	})
	if row.DoubleClicked() {
		a.openLineEditor(index)
	}
	row.Children(func() {
		ui.Column(c).Width(46).Children(func() {
			mono(c, timeText(line.Start), 8, pal.faint)
		})
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			chips := a.lineChordChips(index)
			if labels := phoneChordLabels(chips); labels != "" {
				mono(c, labels, 8.5, pal.accent)
			}
			label(c, line.Text, 11.5, pal.text, 500).SingleLine()
		})
		if selected {
			state := "当前"
			if a.playing {
				state = "播放中"
			}
			text(c, state, 8, pal.accent)
		} else if line.NeedsReview {
			dot(c, pal.warn)
		}
	})
	if a.revealLine == index || (a.playing && a.phoneTab == phoneLyricsTab && selected) {
		row.ScrollIntoView()
		a.revealLine = -1
	}
}

func phoneChordLabels(chips []lineChordChip) string {
	var labels []string
	for _, chip := range chips {
		if len(labels) == 0 || labels[len(labels)-1] != chip.Label {
			labels = append(labels, chip.Label)
		}
	}
	return strings.Join(labels, "  ")
}

func (a *app) phoneChords(c *ui.Context) {
	bar, index, ok := a.barAt(a.playhead)
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(112).Padding(8, 14, 8, 14).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(3).Children(func() {
				mono(c, fmt.Sprintf("第 %d 小节 · 4 拍", index+1), 8.5, pal.muted)
				name := a.currentChordLabel(bar)
				if name == "" {
					name = "—"
				}
				label(c, name, 42, pal.text, 650)
				mono(c, fmt.Sprintf("%s · Capo %d", a.proj.Key, a.proj.Capo), 8.5, pal.faint)
			})
			fret := ui.Box(c).Size(78, 92)
			fret.Draw(func(p *ui.Painter, r ui.Rect) {
				if name := firstChord(bar); name != "" {
					if chord, ok := uke.Normalize(name); ok {
						if shape, ok := uke.Shape(chord, a.proj.Capo); ok {
							drawDiagram(p, r, shape, a.overlayAccent())
							return
						}
					}
				}
				p.Text(r.X+r.W/2-4, r.Y+r.H/2, "?", 16, pal.faint)
			})
		})
		a.phoneNextBars(c, index, ok)
		a.phoneAlignCard(c)
		ui.Row(c).Height(28).Padding(0, 14, 0, 14).AlignItems(ui.Center).Children(func() {
			label(c, "按小节", 9.5, pal.muted, 600)
			ui.Box(c).Grow(1)
			mono(c, fmt.Sprintf("%d 小节", len(a.proj.Bars)), 8, pal.faint)
		})
		scroll := ui.Scroll(c).Grow(1).TrackScroll(&a.phoneScroll)
		scroll.Children(func() {
			ui.Column(c).FillWidth().Padding(0, 12, 8, 12).Gap(4).Children(func() {
				a.phoneBarRows(c)
			})
		})
	})
}

func (a *app) phoneNextBars(c *ui.Context, current int, ok bool) {
	if !ok || len(a.proj.Bars) == 0 {
		return
	}
	ui.Row(c).Height(38).Padding(0, 14, 0, 14).Gap(6).AlignItems(ui.Center).Children(func() {
		text(c, "接下来", 8.5, pal.faint)
		for step := 1; step <= 3; step++ {
			index := (current + step) % len(a.proj.Bars)
			item := a.proj.Bars[index]
			name := firstChord(item)
			if name == "" {
				name = "—"
			}
			b := ui.ButtonBase(c).Height(28).PaddingX(11).Radius(7).Center().
				Label(name).Background(pal.card).Border(1, pal.line).FocusRing(false)
			b.OnClick(func() { a.seekTo(item.Start) })
			b.Children(func() {
				label(c, name, 10, pal.text, 600)
			})
		}
	})
}

func (a *app) phoneAlignCard(c *ui.Context) {
	if a.selectedLine < 0 || a.selectedLine >= len(a.proj.Lines) {
		return
	}
	line := a.proj.Lines[a.selectedLine]
	ui.Column(c).Margin(0, 12, 0, 12).Padding(9, 10, 9, 10).Radius(10).
		Background(pal.panel).Border(1, pal.line).Gap(5).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			label(c, "按字对齐", 9.5, pal.muted, 600)
			ui.Box(c).Grow(1)
			text(c, "点字跳转 · 点上方和弦改", 8, pal.faint)
		})
		if chips := a.lineChordChips(a.selectedLine); len(chips) > 0 {
			a.lineChart(c, a.selectedLine, line, chips)
		} else {
			label(c, line.Text, 12, pal.text, 500)
		}
		a.phoneCharRow(c, a.selectedLine, line)
	})
}

func (a *app) phoneCharRow(c *ui.Context, index int, line project.Line) {
	runes := []rune(line.Text)
	times := line.RuneTimes()
	current := line.CurrentRune(a.playhead)
	spans := a.proj.LineChordSpans(index)
	ui.Row(c).Wrap().Gap(3).Children(func() {
		for i, r := range runes {
			if r == ' ' {
				ui.Box(c).Width(8).Height(30)
				continue
			}
			at := i
			item := string(r)
			chord, hasChord := project.ChordAtRune(spans, at)
			cell := ui.ButtonBase(c).MinWidth(26).Height(30).PaddingX(4).Radius(6).
				Center().Label(item).FocusRing(false)
			cell.Background(pal.card)
			if hasChord && chordsDifferAt(spans, at) {
				cell.Border(1, pal.accentLine)
			}
			if at == current {
				cell.Background(pal.accent)
			}
			cell.OnClick(func() {
				a.selectedLine = index
				if at < len(times) {
					a.seekTo(times[at])
				}
				if hasChord {
					a.chordBar = score.BarIndexAt(a.proj.Bars, times[at])
					a.chordChoice = chord.Label
				}
			})
			if cell.DoubleClicked() {
				if hasChord {
					a.chordBar = score.BarIndexAt(a.proj.Bars, times[at])
					a.chordChoice = chord.Label
					a.modal = modalChords
				}
			}
			cell.Children(func() {
				color := pal.text
				if at == current {
					color = pal.accentInk
				}
				label(c, item, 11, color, 500)
			})
		}
	})
}

func chordsDifferAt(spans []project.ChordSpan, at int) bool {
	for _, span := range spans {
		if at >= span.From && at < span.To {
			return !span.Inherited
		}
	}
	return false
}

func (a *app) phoneBarRows(c *ui.Context) {
	if len(a.proj.Bars) == 0 {
		ui.Row(c).Padding(10, 12, 10, 12).Radius(9).Background(pal.card).Children(func() {
			text(c, "还没有小节，导入曲谱后会生成。", 10, pal.faint)
		})
		return
	}
	current := score.BarIndexAt(a.proj.Bars, a.playhead)
	if a.phoneBar != current {
		if current >= 0 {
			a.phoneScroll.Y = float32(max(0, current-2)) * 46
		}
		a.phoneBar = current
	}
	for i, bar := range a.proj.Bars {
		index := i
		item := bar
		selected := index == current
		row := ui.ButtonBase(c).FillWidth().MinHeight(42).Padding(6, 10, 6, 10).
			Radius(8).AlignItems(ui.Center).Label(fmt.Sprintf("第 %d 小节", index+1)).FocusRing(false)
		if selected {
			row.Background(pal.sel).Border(1, pal.lineStrong)
		} else {
			row.Background(pal.card)
		}
		row.OnClick(func() { a.seekTo(item.Start) })
		if row.DoubleClicked() {
			a.chordBar = index
			a.chordChoice = firstChord(item)
			a.modal = modalChords
		}
		row.Children(func() {
			mono(c, fmt.Sprintf("%02d", index+1), 8, pal.faint).Width(22)
			name := firstChord(item)
			if name == "" {
				name = "—"
			}
			label(c, name, 11, pal.text, 600).Width(44)
			ui.Box(c).Grow(1)
			if item.NeedsReview {
				dot(c, pal.warn)
			}
			mono(c, timeText(item.Start), 8, pal.faint)
			ui.Box(c).Width(6)
			edit := ui.ButtonBase(c).Height(26).PaddingX(8).Radius(6).Center().
				Label("改").FocusRing(false).Background(pal.panel2).Border(1, pal.line)
			edit.OnClick(func() {
				a.chordBar = index
				a.chordChoice = firstChord(item)
				a.modal = modalChords
			})
			edit.Children(func() {
				label(c, "改", 8.5, pal.muted, 600)
			})
		})
	}
}

func (a *app) phoneProject(c *ui.Context) {
	ui.Column(c).Fill().Padding(10, 12, 8, 12).Gap(8).Children(func() {
		newProject := ui.ButtonBase(c).FillWidth().Height(66).Padding(8, 12, 8, 12).
			Radius(11).Background(pal.card).Border(1, pal.line).Label("新建工程").FocusRing(false)
		newProject.OnClick(a.openNewProject)
		newProject.Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				box := ui.Box(c).Size(38, 38).Radius(9).Background(pal.warnBG).Center()
				box.Children(func() { text(c, "+", 18, pal.accent) })
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					label(c, "新建工程", 12, pal.text, 650)
					text(c, "视频 + 曲谱 → 自动对齐", 8.5, pal.muted)
				})
				text(c, "视频 / 曲谱", 8.5, pal.faint)
			})
		})
		scroll := ui.Scroll(c).Grow(1)
		scroll.Children(func() {
			ui.Column(c).FillWidth().Gap(4).Children(func() {
				for i, item := range a.projects {
					a.phoneProjectRow(c, i, item)
				}
			})
		})
		ui.Row(c).Height(52).AlignItems(ui.Center).Gap(8).Children(func() {
			button := ui.ButtonBase(c).Grow(1).Height(38).Center().Radius(9).
				Background(pal.accent).Label("导出成片").FocusRing(false)
			button.OnClick(a.openExportModal)
			button.Children(func() {
				label(c, "导出成片", 11, pal.accentInk, 650)
			})
			a.phoneAction(c, "⚙", "设置", a.openSettings)
		})
	})
}

func (a *app) phoneProjectRow(c *ui.Context, index int, item projectItem) {
	selected := index == a.selected
	row := ui.ButtonBase(c).FillWidth().MinHeight(58).Padding(7, 9, 7, 9).
		Radius(10).AlignItems(ui.Center).Label(item.title).FocusRing(false)
	if selected {
		row.Background(pal.sel).Border(1, pal.lineStrong)
	} else {
		row.Background(pal.card)
	}
	row.OnClick(func() {
		a.selectProject(index)
		a.phoneTab = phoneEditTab
	})
	row.Children(func() {
		box := ui.Box(c).Size(40, 40).Radius(8).Background(pal.bg2).Center()
		box.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Text(r.X+10, r.Y+r.H-10, item.chord, 9, pal.muted)
		})
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			label(c, item.title, 11.5, pal.text, 600).SingleLine()
			mono(c, item.meta, 8.5, pal.muted).SingleLine()
		})
		text(c, item.state, 8, pal.faint)
		a.phoneAction(c, "⋯", "工程设置", func() { a.openProjectModal(index) })
	})
}

func (a *app) openExportModal() {
	a.exportProgress = 0
	a.exportErr = ""
	a.exportPath = ""
	a.exportNote = ""
	a.modal = modalExport
}
