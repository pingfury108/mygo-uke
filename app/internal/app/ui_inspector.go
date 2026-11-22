package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/score"
	"mygo-uke/internal/uke"
)

func (a *app) inspector(c *ui.Context) {
	ui.Column(c).Width(326).Background(pal.panel).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X+0.5, r.Y, r.X+0.5, r.Y+r.H, 1, pal.line)
		}).Children(func() {
		ui.Row(c).Height(45).Padding(0, 12, 0, 12).Gap(2).
			AlignItems(ui.Stretch).Children(func() {
			for i, name := range []string{"字幕", "和弦", "样式"} {
				index := i
				on := a.tab == index
				b := ui.ButtonBase(c).Grow(1).Center().Label(name).FocusRing(false)
				b.OnClick(func() { a.tab = index })
				b.DrawOver(func(p *ui.Painter, r ui.Rect) {
					if on {
						p.Fill(ui.Rect{X: r.X + 14, Y: r.Y + r.H - 2, W: r.W - 28, H: 2}, pal.accent, 1)
					}
				})
				b.Children(func() {
					color := pal.muted
					weight := 500
					if on {
						color = pal.text
						weight = 650
					}
					label(c, name, 11, color, weight)
				})
			}
		})
		ui.Box(c).Height(1).FillWidth().Background(pal.line)
		scroll := ui.Scroll(c).Grow(1)
		scroll.Children(func() {
			ui.Column(c).FillWidth().Padding(14).Gap(8).Children(func() {
				switch a.tab {
				case 1:
					a.chordTab(c)
				case 2:
					a.styleTab(c)
				default:
					a.subtitleTab(c)
				}
			})
		})
	})
}

func (a *app) subtitleTab(c *ui.Context) {
	ui.Row(c).AlignItems(ui.Center).PaddingY(4).Children(func() {
		label(c, "逐句字幕", 10, pal.muted, 600)
		mono(c, fmt.Sprintf("%d 句", len(a.proj.Lines)), 9, pal.faint).Margin(0, 6, 0, 0)
		ui.Box(c).Grow(1)
		if len(a.proj.Lines) > 0 {
			ghostButton(c, "编辑选中", func() { a.openLineEditor(a.selectedLine) })
		}
		ghostButton(c, "添加", func() { a.openLineEditor(-1) })
	})
	if len(a.proj.Lines) == 0 {
		ui.Row(c).Padding(9, 10, 9, 10).Radius(8).Background(pal.card).
			Children(func() {
				text(c, "还没有字幕：点「添加」手填，或配置火山 Key 后重新分析", 9, pal.faint)
			})
	}
	for i, line := range a.proj.Lines {
		index := i
		l := line
		selected := index == a.selectedLine
		card := ui.ButtonBase(c).FillWidth().Padding(9).Radius(8).
			Label(l.Text).AlignItems(ui.Start).FocusRing(false)
		if selected {
			card.Background(pal.sel).Border(1, pal.lineStrong)
		} else {
			card.Background(pal.card)
		}
		if card.Hovered() && !selected {
			card.Background(pal.panel2)
		}
		card.DrawOver(func(p *ui.Painter, r ui.Rect) {
			if selected {
				p.Fill(ui.Rect{X: r.X, Y: r.Y + 8, W: 2, H: r.H - 16}, pal.accent, 1)
			}
		})
		card.OnClick(func() {
			a.selectedLine = index
			a.playhead = l.Start
			a.playing = false
			if a.player != nil {
				a.player.Pause()
			}
			a.seekPlayer()
			a.syncSelection()
		})
		card.Children(func() {
			ui.Column(c).FillWidth().Gap(3).Children(func() {
				mono(c, fmt.Sprintf("%s — %s", timeText(l.Start), timeText(l.End)), 8.5, pal.faint)
				label(c, l.Text, 12, pal.text, 500)
				ui.Row(c).Gap(10).Children(func() {
					text(c, "点卡片跳到这句", 9, pal.muted)
					if l.NeedsReview {
						ui.Box(c).Grow(1)
						text(c, "待确认", 8.5, pal.warn)
					}
				})
			})
		})
	}

	if n := a.reviewBars(); n > 0 {
		callout := ui.ButtonBase(c).FillWidth().Padding(8, 10, 8, 10).Radius(8).
			Background(pal.warnBG).AlignItems(ui.Center).Label("待确认").FocusRing(false)
		callout.OnClick(func() {
			for _, bar := range a.proj.Bars {
				if bar.NeedsReview {
					a.playhead = bar.Start
					a.seekPlayer()
					a.syncSelection()
					break
				}
			}
		})
		callout.Children(func() {
			ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
				dot(c, pal.warn)
				text(c, fmt.Sprintf("%d 处需要确认 · 点一下跳到对应小节", n), 9.5, pal.muted)
			})
		})
	}

	sectionLabel(c, "节奏型", a.patternName())
	grid := ui.Grid(c).FillWidth().Columns(2).Gap(6)
	grid.Children(func() {
		for i, name := range []string{"民谣基础", "下上", "三指法", "3/4 华尔兹"} {
			index := i
			on := a.pattern == index
			b := ui.ButtonBase(c).Height(30).Radius(7).Center().Label(name).FocusRing(false)
			if on {
				b.Background(pal.warnBG).Border(1, pal.accentLine)
			} else {
				b.Background(pal.card).Border(1, pal.line)
			}
			if b.Hovered() && !on {
				b.Background(pal.panel2)
			}
			b.OnClick(func() {
				a.pattern = index
				a.proj.Pattern = patternID(index)
				a.saveCurrent()
			})
			b.Children(func() {
				color := pal.muted
				weight := 500
				if on {
					color = pal.text
					weight = 600
				}
				label(c, name, 10, color, weight)
			})
		}
	})
	ui.Row(c).Padding(8, 10, 8, 10).Radius(7).Background(pal.card).
		Center().Children(func() {
		mono(c, a.strumText(), 12, pal.muted)
	})
}

func (a *app) chordTab(c *ui.Context) {
	sectionLabel(c, "常用和弦", "点选替换当前小节")
	grid := ui.Grid(c).FillWidth().Columns(4).Gap(6)
	grid.Children(func() {
		for _, name := range []string{"C", "G", "Am", "Em", "F", "Dm", "G7", "Cmaj7", "Am7", "Em7", "D7", "Bb"} {
			chord := name
			b := ui.ButtonBase(c).Height(30).Radius(7).Center().Label(chord).FocusRing(false).
				Background(pal.card).Border(1, pal.line)
			if b.Hovered() {
				b.Background(pal.warnBG)
			}
			b.OnClick(func() {
				bar, index, ok := a.barAt(a.playhead)
				if !ok || len(bar.Chords) == 0 {
					return
				}
				a.setBarChord(index, chord)
				a.toast(c, "和弦已更新", fmt.Sprintf("第 %d 小节 → %s", index+1, chord))
			})
			b.Children(func() {
				label(c, chord, 10.5, pal.text, 600)
			})
		}
	})

	sectionLabel(c, "按小节", fmt.Sprintf("%d 小节", len(a.proj.Bars)))
	currentIndex := score.BarIndexAt(a.proj.Bars, a.playhead)
	for i, bar := range a.proj.Bars {
		index := i
		b := bar
		if i < currentIndex-6 {
			continue
		}
		if i > currentIndex+14 {
			break
		}
		selected := index == currentIndex
		row := ui.ButtonBase(c).FillWidth().Padding(7, 9, 7, 9).Radius(7).
			Label(fmt.Sprintf("第 %d 小节", index+1)).AlignItems(ui.Center).FocusRing(false)
		if selected {
			row.Background(pal.sel).Border(1, pal.lineStrong)
		} else {
			row.Background(pal.card)
		}
		if row.Hovered() && !selected {
			row.Background(pal.panel2)
		}
		row.OnClick(func() {
			a.playhead = b.Start
			a.seekPlayer()
			a.syncSelection()
			a.chordBar = index
			a.chordChoice = firstChord(b)
			a.modal = modalChords
		})
		row.Children(func() {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				mono(c, fmt.Sprintf("%02d", index+1), 9, pal.faint)
				chord := firstChord(b)
				if chord == "" {
					chord = "—"
				}
				label(c, chord, 11, pal.text, 600)
				if len(b.Chords) > 1 {
					label(c, b.Chords[1], 9.5, pal.muted, 500)
				}
				if b.Approx {
					text(c, "~", 9, pal.warn)
				}
				ui.Box(c).Grow(1)
				if b.NeedsReview {
					dot(c, pal.warn)
				}
				mono(c, timeText(b.Start), 8.5, pal.faint)
			})
		})
	}
}

func (a *app) styleTab(c *ui.Context) {
	sectionLabel(c, "叠加层", "竖屏实时预览")
	sliderRow(c, "和弦字号", &a.proj.Style.ChordSize, a.saveCurrent)
	sliderRow(c, "字幕字号", &a.proj.Style.SubSize, a.saveCurrent)

	sectionLabel(c, "样式预设", "一键套用")
	ui.Row(c).Gap(6).Children(func() {
		for i, preset := range stylePresets {
			index := i
			item := preset
			on := a.proj.Style.Accent == item.Accent &&
				a.proj.Style.ChordSize == item.ChordSize &&
				a.proj.Style.SubSize == item.SubSize
			b := ui.ButtonBase(c).Grow(1).Height(30).Radius(7).Center().
				Label(item.Name).FocusRing(false)
			if on {
				b.Background(pal.sel).Border(1, pal.accentLine)
			} else {
				b.Background(pal.card).Border(1, pal.line)
			}
			if b.Hovered() && !on {
				b.Background(pal.panel2)
			}
			b.OnClick(func() { a.applyStylePreset(c, index) })
			b.Children(func() {
				text(c, item.Name, 9.5, pal.text)
			})
		}
	})

	sectionLabel(c, "强调色", a.proj.Style.Accent)
	ui.Row(c).Gap(7).Children(func() {
		for _, item := range []struct {
			name string
			hex  string
		}{
			{"琥珀", "#F3B544"},
			{"朱砂", "#B5482E"},
			{"青绿", "#1F7A74"},
			{"米白", "#F3EEE4"},
		} {
			it := item
			on := a.proj.Style.Accent == it.hex
			b := ui.ButtonBase(c).Size(38, 26).Radius(6).Center().Label(it.name).FocusRing(false).
				Background(ui.Hex(it.hex))
			if on {
				b.Border(2, pal.text)
			}
			b.OnClick(func() {
				a.proj.Style.Accent = it.hex
				a.saveCurrent()
				a.toast(c, "强调色已更新", it.name)
			})
			b.Children(func() {
				color := ui.Hex("#1D1507")
				if it.hex == "#B5482E" || it.hex == "#1F7A74" {
					color = ui.Hex("#FFF8F0")
				}
				text(c, it.name, 9, color)
			})
		}
	})

	sectionLabel(c, "输出画幅", "预览与成片同源")
	segment(c, []string{"竖屏 9:16", "横屏 16:9"}, &a.orientation).FillWidth()

	sectionLabel(c, "变调夹", fmt.Sprintf("Capo %d", a.proj.Capo))
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ghostButton(c, "−", func() {
			if a.proj.Capo > 0 {
				a.proj.Capo--
				a.saveCurrent()
			}
		})
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			text(c, fmt.Sprintf("实际 %s 调", a.proj.Key), 9.5, pal.muted)
			mono(c, "按 "+shapeForKey(a.proj.Key, a.proj.Capo)+" 形状", 9, pal.faint)
		})
		ghostButton(c, "+", func() {
			if a.proj.Capo < 7 {
				a.proj.Capo++
				a.saveCurrent()
			}
		})
	})

	ui.Row(c).Padding(8, 10, 8, 10).Radius(7).Background(pal.card).
		AlignItems(ui.Center).Children(func() {
		text(c, "安全区", 10, pal.muted)
		ui.Box(c).Grow(1)
		if ui.Switch(c, &a.proj.Style.ShowSafe).Changed() {
			a.saveCurrent()
		}
	})
	ui.Row(c).Padding(8, 10, 8, 10).Radius(7).Background(pal.card).
		AlignItems(ui.Center).Children(func() {
		text(c, "和弦指法图", 10, pal.muted)
		ui.Box(c).Grow(1)
		text(c, "随当前小节", 9, pal.faint)
	})

	sectionLabel(c, "工程", "project.json")
	ui.Column(c).Padding(9, 10, 9, 10).Radius(7).Gap(3).Background(pal.card).Children(func() {
		mono(c, fmt.Sprintf("%.2fs · %.0f BPM", a.proj.Duration, a.proj.Tempo), 9, pal.muted)
		mono(c, fmt.Sprintf("%d bars · %d lines", len(a.proj.Bars), len(a.proj.Lines)), 9, pal.faint)
		asrState := "未配置"
		if a.asrConfig.APIKey != "" {
			asrState = "已配置"
		}
		mono(c, "ASR "+asrState, 8.5, pal.faint)
		if len(a.proj.Warnings) > 0 {
			mono(c, a.proj.Warnings[0], 8.5, pal.warn).SingleLine()
		}
		mono(c, a.proj.SourceHash, 8.5, pal.faint).SingleLine()
	})
}

func sliderRow(c *ui.Context, name string, value *float64, onChange func()) {
	ui.Column(c).Gap(3).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			text(c, name, 10, pal.muted)
			ui.Box(c).Grow(1)
			mono(c, fmt.Sprintf("%.0f", *value), 9, pal.faint)
		})
		if ui.Slider(c, value, 32, 96).FillWidth().Changed() && onChange != nil {
			onChange()
		}
	})
}

func firstChord(b score.Bar) string {
	if len(b.Chords) == 0 {
		return ""
	}
	return b.Chords[0]
}

func shapeForKey(key string, capo int) string {
	chord, ok := uke.Normalize(key)
	if !ok {
		return key
	}
	shape, ok := uke.Shape(chord, capo)
	if !ok {
		return key
	}
	return shape.Name
}

func (a *app) patternName() string {
	switch a.pattern {
	case 1:
		return "下上"
	case 2:
		return "三指法"
	case 3:
		return "3/4 华尔兹"
	default:
		return "民谣基础"
	}
}

var patternIDs = []string{"folk-basic", "down-up", "fingerpick-3", "waltz-3"}

func patternIndex(id string) int {
	for i, value := range patternIDs {
		if value == id {
			return i
		}
	}
	return 0
}

func patternID(index int) string {
	if index < 0 || index >= len(patternIDs) {
		return patternIDs[0]
	}
	return patternIDs[index]
}

func (a *app) strumText() string {
	switch a.pattern {
	case 1:
		return "↓  ↑  ↓  ↑"
	case 2:
		return "p  i  m  a"
	case 3:
		return "↓   ↓↑   ↓↑"
	default:
		return "↓   ↓↑   ↑↓↑"
	}
}
