package app

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/render"
	"mygo-uke/internal/score"
	"mygo-uke/internal/uke"
)

func (a *app) overlayAccent() ui.Color {
	if a.proj != nil && a.proj.Style.Accent != "" {
		return ui.Hex(a.proj.Style.Accent)
	}
	return pal.accent
}

func (a *app) overlayInk() ui.Color {
	return render.InkOn(a.proj.Style.Accent)
}

func (a *app) stage(c *ui.Context) {
	ui.Column(c).Grow(1).Background(pal.stage).Children(func() {
		a.stageHead(c)
		a.preview(c)
		a.transport(c)
	})
}

func (a *app) stageHead(c *ui.Context) {
	ui.Row(c).Height(58).Padding(0, 18, 0, 18).Gap(14).
		AlignItems(ui.Center).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X, r.Y+r.H-0.5, r.X+r.W, r.Y+r.H-0.5, 1, pal.line)
		}).Children(func() {
		ui.Column(c).Gap(3).Children(func() {
			label(c, a.proj.Title, 17, pal.text, 650)
			mono(c, metaLine(a.proj), 9.5, pal.muted)
		})
		ui.Box(c).Grow(1)
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			badgeRow := ui.Row(c).Padding(4, 8, 4, 8).Radius(999).Background(pal.warnBG).
				AlignItems(ui.Center).Gap(5)
			badgeRow.Children(func() {
				value := "—"
				if confidence := a.avgConfidence(); confidence > 0 {
					value = fmt.Sprintf("%.0f%%", confidence*100)
				}
				label(c, value, 9.5, pal.text, 650)
				text(c, "和弦置信度", 9.5, pal.muted)
			})
			ghostButton(c, "安全区", func() {
				a.proj.Style.ShowSafe = !a.proj.Style.ShowSafe
				a.saveCurrent()
			})
			if a.player != nil {
				muteLabel := "静音"
				if a.muted {
					muteLabel = "有声"
				}
				ghostButton(c, muteLabel, func() { a.setMuted(!a.muted) })
			}
			ghostButton(c, "预览", func() {
				a.toast(c, "预览", "预览与成片共用同一套叠加层")
			})
		})
	})
}

func (a *app) avgConfidence() float64 {
	if a.proj == nil {
		return 0
	}
	sum, count := 0.0, 0
	for _, bar := range a.proj.Bars {
		if bar.Conf > 0 {
			sum += bar.Conf
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func (a *app) preview(c *ui.Context) {
	width, height := c.Size()
	if a.orientation == 1 {
		available := width - 234 - 326 - 52
		frameW := available
		if frameW > 760 {
			frameW = 760
		}
		if frameW < 420 {
			frameW = 420
		}
		mat := ui.Box(c).Grow(1).Center().Padding(8, 18, 12, 18).Background(pal.mat)
		mat.Draw(func(p *ui.Painter, r ui.Rect) {
			p.FillRadialGradient(r, ui.RadialGradient{
				Stops: []ui.GradientStop{
					{Color: ui.RGBA(255, 255, 255, 0.035), At: 0},
					{Color: ui.RGBA(255, 255, 255, 0), At: 1},
				},
				Center: ui.Point{X: 0.5, Y: 0.3},
			}, 0)
		})
		mat.Children(func() {
			a.horizontalFrame(c, frameW, frameW*9/16)
		})
		return
	}
	frameH := height - 52 - 30 - 58 - 128
	if frameH > 500 {
		frameH = 500
	}
	if frameH < 280 {
		frameH = 280
	}
	frameW := frameH * 9 / 16

	mat := ui.Box(c).Grow(1).Center().Padding(8, 18, 12, 18).Background(pal.mat)
	mat.Draw(func(p *ui.Painter, r ui.Rect) {
		p.FillRadialGradient(r, ui.RadialGradient{
			Stops: []ui.GradientStop{
				{Color: ui.RGBA(255, 255, 255, 0.035), At: 0},
				{Color: ui.RGBA(255, 255, 255, 0), At: 1},
			},
			Center: ui.Point{X: 0.5, Y: 0.3},
		}, 0)
	})
	mat.Children(func() {
		a.videoFrame(c, frameW, frameH)
	})
}

func (a *app) horizontalFrame(c *ui.Context, w, h float32) {
	bar, _, _ := a.barAt(a.playhead)
	_, lineIndex, _ := a.lineAt(a.playhead)
	centerW := h * 607 / 1080
	sideW := (w - centerW) / 2

	frame := ui.Box(c).Size(w, h).Radius(15).Clip().
		Border(1, pal.frameLine).Background(ui.Hex("#101114")).
		Shadow(0, 26, 54, 0, ui.RGBA(0, 0, 0, 0.32))
	frame.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, ui.Hex("#121316"), 0)
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: sideW, H: r.H}, ui.Hex("#17181C"), 0)
		p.Fill(ui.Rect{X: r.X + r.W - sideW, Y: r.Y, W: sideW, H: r.H}, ui.Hex("#17181C"), 0)
		p.Line(r.X+sideW, r.Y, r.X+sideW, r.Y+r.H, 1, pal.lineStrong)
		p.Line(r.X+r.W-sideW, r.Y, r.X+r.W-sideW, r.Y+r.H, 1, pal.lineStrong)
	})
	frame.Children(func() {
		center := ui.Box(c).Absolute().Left(sideW).Top(0).Width(centerW).Height(h)
		center.Draw(func(p *ui.Painter, r ui.Rect) {
			drawVideoPlaceholder(p, r, "人像 607 × 1080")
		})
		center.Children(func() {
			a.videoLayer(c, ui.Cover)
		})

		ui.Column(c).Absolute().Left(0).Top(0).Width(sideW).Height(h).
			Padding(16, 14, 14, 14).Gap(6).Children(func() {
			label(c, "当前和弦", 8.5, pal.muted, 600)
			name := "—"
			if len(bar.Chords) > 0 {
				name = bar.Chords[0]
			}
			label(c, name, 34, pal.text, 650)
			mono(c, fmt.Sprintf("原调 %s · Capo %d", a.proj.Key, a.proj.Capo), 8.5, pal.muted)
			mono(c, fmt.Sprintf("第 %d 小节 · 4 拍", bar.I+1), 8.5, pal.faint)
			ui.Box(c).Height(10)
			shapeLabel := ""
			shapeBox := ui.Box(c).Size(82, 94)
			if len(bar.Chords) > 0 {
				if chord, ok := uke.Normalize(bar.Chords[0]); ok {
					if shape, ok := uke.Shape(chord, a.proj.Capo); ok {
						shapeLabel = shape.Name
						shapeBox.Draw(func(p *ui.Painter, r ui.Rect) {
							drawDiagram(p, r, shape, a.overlayAccent())
						})
					}
				}
			}
			if shapeLabel != "" {
				text(c, "按 "+shapeLabel+" 形状", 8.5, pal.faint)
			}
		})

		rightW := sideW
		ui.Column(c).Absolute().Right(0).Top(0).Width(rightW).Height(h).
			Padding(16, 14, 14, 14).Gap(7).Children(func() {
			label(c, a.proj.Title, 14, pal.text, 650).SingleLine()
			mono(c, fmt.Sprintf("%.0f BPM · %d/%d · %s 调", a.proj.Tempo,
				a.proj.TimeSignature[0], a.proj.TimeSignature[1], a.proj.Key), 8, pal.faint)
			for i := lineIndex - 1; i <= lineIndex+2; i++ {
				if i < 0 || i >= len(a.proj.Lines) {
					continue
				}
				item := a.proj.Lines[i]
				row := ui.Column(c).FillWidth().Gap(2).Padding(5, 6, 5, 6).Radius(5)
				if i == lineIndex {
					row.Background(pal.warnBG)
				}
				row.Children(func() {
					chords := "—"
					if b, _, ok := a.barAt(item.Start); ok && len(b.Chords) > 0 {
						chords = b.Chords[0]
						if len(b.Chords) > 1 {
							chords += "  " + b.Chords[1]
						}
					}
					mono(c, chords, 8.5, pal.accent)
					text(c, item.Text, 10.5, pal.text).SingleLine()
				})
			}
			ui.Box(c).Grow(1)
			mono(c, a.patternName(), 8.5, pal.faint)
		})
	})
}

func (a *app) videoFrame(c *ui.Context, w, h float32) {
	bar, _, ok := a.barAt(a.playhead)
	if !ok && len(a.proj.Bars) > 0 {
		bar = a.proj.Bars[0]
	}
	line, _, hasLine := a.lineAt(a.playhead)

	frame := ui.Box(c).Size(w, h).Radius(17).Clip().
		Border(1, pal.frameLine).Background(ui.Hex("#171512")).
		Shadow(0, 28, 60, 0, ui.RGBA(0, 0, 0, 0.34))
	frame.Draw(func(p *ui.Painter, r ui.Rect) {
		drawVideoPlaceholder(p, r, "原视频 · 9:16")
	})
	frame.DrawOver(func(p *ui.Painter, r ui.Rect) {
		if a.proj.Style.ShowSafe {
			drawSafeLines(p, r)
		}
	})
	frame.Children(func() {
		a.videoLayer(c, ui.Cover)
		a.chordStrip(c, bar, w, h)
		diagramCorner(c, bar, w, h, a.proj.Capo, a.overlayAccent())
		if hasLine {
			subtitleOverlay(c, line.Text, w, h)
		}
		if a.proj.Style.ShowSafe {
			safeLabels(c, w, h)
		}
		ui.Row(c).Absolute().Right(7).Bottom(7).Padding(2, 6, 2, 6).Radius(5).
			Background(ui.RGBA(0, 0, 0, 0.34)).Children(func() {
			mono(c, "1080 × 1920", 7, ui.RGBA(255, 248, 234, 0.62))
		})
	})
}

func drawVideoPlaceholder(p *ui.Painter, r ui.Rect, tag string) {
	p.FillGradient(r, ui.LinearGradient{
		Stops: []ui.GradientStop{
			{Color: ui.Hex("#33261C"), At: 0},
			{Color: ui.Hex("#1C1712"), At: 0.54},
			{Color: ui.Hex("#0E0D0B"), At: 1},
		},
		Angle: 160,
	}, 0)
	p.FillRadialGradient(r, ui.RadialGradient{
		Stops: []ui.GradientStop{
			{Color: ui.RGBA(255, 205, 130, 0.42), At: 0},
			{Color: ui.RGBA(255, 205, 130, 0), At: 1},
		},
		Center: ui.Point{X: 0.72, Y: 0.12},
		Radius: r.W * 0.5,
	}, 0)
	p.FillRadialGradient(r, ui.RadialGradient{
		Stops: []ui.GradientStop{
			{Color: ui.RGBA(94, 126, 95, 0.18), At: 0},
			{Color: ui.RGBA(94, 126, 95, 0), At: 1},
		},
		Center: ui.Point{X: 0.18, Y: 0.84},
		Radius: r.W * 0.48,
	}, 0)
	p.FillGradient(ui.Rect{X: r.X, Y: r.Y + r.H*0.58, W: r.W, H: r.H * 0.42},
		ui.LinearGradient{
			From:  ui.Transparent,
			To:    ui.RGBA(0, 0, 0, 0.58),
			Angle: 180,
		}, 0)

	cx, cy := r.X+r.W/2, r.Y+r.H*0.46
	p.Stroke(ui.Rect{X: cx - 17, Y: cy - 17, W: 34, H: 34}, ui.RGBA(255, 246, 228, 0.28), 17, 1)
	p.Text(cx-4, cy+5, "▶", 11, ui.RGBA(255, 246, 228, 0.52))
	p.Text(cx-float32(len([]rune(tag)))*3.2, cy+36, tag, 8, ui.RGBA(255, 246, 228, 0.42))
}

func drawSafeLines(p *ui.Painter, r ui.Rect) {
	line := ui.RGBA(243, 181, 68, 0.55)
	p.Line(r.X, r.Y+r.H*0.04, r.X+r.W, r.Y+r.H*0.04, 1, line)
	p.Line(r.X, r.Y+r.H*0.85, r.X+r.W, r.Y+r.H*0.85, 1, line)
	p.Line(r.X+r.W*0.889, r.Y, r.X+r.W*0.889, r.Y+r.H, 1, line)
}

func safeLabels(c *ui.Context, w, h float32) {
	ui.Row(c).Absolute().Right(7).Top(h*0.04-17).Padding(2, 5, 2, 5).Radius(4).
		Background(ui.RGBA(243, 181, 68, 0.16)).Children(func() {
		mono(c, "4%", 6.5, pal.accent)
	})
	ui.Row(c).Absolute().Left(7).Top(h*0.85-17).Padding(2, 5, 2, 5).Radius(4).
		Background(ui.RGBA(243, 181, 68, 0.16)).Children(func() {
		mono(c, "15%", 6.5, pal.accent)
	})
	ui.Row(c).Absolute().Right(w*0.111+7).Top(h*0.5).Padding(2, 5, 2, 5).Radius(4).
		Background(ui.RGBA(243, 181, 68, 0.16)).Children(func() {
		mono(c, "120px", 6.5, pal.accent)
	})
}

func (a *app) chordStrip(c *ui.Context, bar score.Bar, w, h float32) {
	chords := bar.Chords
	if len(chords) == 0 {
		chords = []string{""}
	}
	_, index, _ := a.barAt(a.playhead)
	upcoming := []string{}
	for i := index + 1; i < len(a.proj.Bars) && len(upcoming) < 2; i++ {
		upcoming = append(upcoming, a.proj.Bars[i].Chords...)
	}
	row := ui.Row(c).Absolute().LeftPercent(6.5).TopPercent(5.4).
		Right(w*0.25 + 8).Gap(5).AlignItems(ui.Start)
	row.Children(func() {
		for i, chord := range chords {
			current := i == 0
			a.chordChip(c, chord, current, h)
		}
		shown := 0
		for _, chord := range upcoming {
			if shown >= 2 {
				break
			}
			a.chordChip(c, chord, false, h)
			shown++
		}
	})
}

func (a *app) chordChip(c *ui.Context, chord string, current bool, h float32) {
	if chord == "" {
		chord = "—"
	}
	if current {
		chip := ui.Column(c).Padding(7, 10, 6, 10).Radius(7).Background(a.overlayAccent()).
			Shadow(0, 10, 24, 0, ui.RGBA(0, 0, 0, 0.24))
		chip.Children(func() {
			label(c, chord, 22, a.overlayInk(), 720).SingleLine()
			mono(c, "1 · 2 · 3 · 4", 6.5, ui.RGBA(29, 21, 7, 0.66))
		})
		return
	}
	chip := ui.Column(c).Padding(5, 7, 4, 7).Radius(7).
		Border(1, ui.RGBA(255, 255, 255, 0.14)).
		Background(ui.RGBA(16, 14, 12, 0.42))
	chip.Children(func() {
		label(c, chord, 17, ui.RGBA(255, 248, 234, 0.82), 720).SingleLine()
	})
}

func diagramCorner(c *ui.Context, bar score.Bar, w, h float32, capo int, accent ui.Color) {
	labelValue := ""
	if len(bar.Chords) > 0 {
		labelValue = bar.Chords[0]
	}
	chord, ok := uke.Normalize(labelValue)
	var shape uke.Fingering
	if ok {
		shape, ok = uke.Shape(chord, capo)
	}
	box := ui.Column(c).Absolute().RightPercent(5.5).TopPercent(5.2).Width(62).
		Padding(5, 5, 4, 5).Radius(9).
		Border(1, ui.RGBA(255, 255, 255, 0.14)).
		Background(ui.RGBA(16, 14, 12, 0.46)).Gap(2)
	box.Children(func() {
		diagram := ui.Box(c).Height(62).FillWidth()
		diagram.Draw(func(p *ui.Painter, r ui.Rect) {
			if ok {
				drawDiagram(p, r, shape, accent)
			} else {
				p.Text(r.X+r.W/2-8, r.Y+r.H/2, "?", 16, pal.faint)
			}
		})
		text(c, labelValue, 7, ui.RGBA(255, 248, 234, 0.78)).TextAlign(ui.Center).SingleLine()
	})
}

func drawDiagram(p *ui.Painter, r ui.Rect, shape uke.Fingering, accent ui.Color) {
	left := r.X + r.W*0.1
	right := r.X + r.W*0.9
	top := r.Y + r.H*0.18
	bottom := r.Y + r.H*0.9
	spanX := (right - left) / 3
	spanY := (bottom - top) / 4
	lineColor := ui.RGBA(255, 255, 255, 0.30)
	for i := 0; i < 4; i++ {
		x := left + spanX*float32(i)
		p.Line(x, top, x, bottom, 1, lineColor)
	}
	for k := 0; k <= 4; k++ {
		y := top + spanY*float32(k)
		if k == 0 {
			p.Line(left, y, right, y, 2, ui.RGBA(255, 255, 255, 0.52))
		} else {
			p.Line(left, y, right, y, 1, lineColor)
		}
	}
	base := shape.Base
	if base < 1 {
		base = 1
	}
	for i, fret := range shape.Frets {
		x := left + spanX*float32(i)
		switch {
		case fret < 0:
			p.Text(x-2.5, top-5, "×", 7, ui.RGBA(255, 248, 234, 0.62))
		case fret == 0:
			p.Stroke(ui.Rect{X: x - 2.5, Y: top - 8, W: 5, H: 5}, ui.RGBA(255, 248, 234, 0.72), 2.5, 1)
		case fret >= base && fret <= base+3:
			cy := top + spanY*(float32(fret-base)+0.5)
			p.Fill(ui.Rect{X: x - 3, Y: cy - 3, W: 6, H: 6}, accent, 3)
		}
	}
}

func subtitleOverlay(c *ui.Context, value string, w, h float32) {
	ui.Box(c).Absolute().LeftPercent(5).RightPercent(5).Bottom(h * 0.19).
		Center().Children(func() {
		ui.Row(c).Padding(7, 10, 7, 10).Radius(7).
			Background(ui.RGBA(10, 9, 8, 0.58)).Children(func() {
			label(c, value, 13.5, ui.Hex("#FFF8EA"), 600).SingleLine()
		})
	})
}

func (a *app) transport(c *ui.Context) {
	ui.Row(c).Height(104).Padding(0, 18, 0, 18).Gap(12).
		AlignItems(ui.Center).Background(pal.stage).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X, r.Y+0.5, r.X+r.W, r.Y+0.5, 1, pal.line)
		}).Children(func() {
		a.playButton(c)
		ui.Column(c).Gap(0).Children(func() {
			mono(c, timeText(a.playhead), 12, pal.text)
			mono(c, "/ "+timeText(a.proj.Duration), 8, pal.faint)
		})
		a.timeline(c)
		ui.Column(c).Width(92).Gap(2).AlignItems(ui.End).Children(func() {
			mono(c, fmt.Sprintf("%.0f BPM", a.proj.Tempo), 9.5, pal.muted)
			mono(c, fmt.Sprintf("%s 调 · Capo %d", a.proj.Key, a.proj.Capo), 9.5, pal.faint)
			text(c, "民谣基础", 9.5, pal.faint)
		})
	})
}

func (a *app) playButton(c *ui.Context) {
	glyph := "▶"
	if a.playing {
		glyph = "❚❚"
	}
	b := ui.ButtonBase(c).Size(38, 38).Radius(999).Center().FocusRing(false).
		Label("播放").Border(1, pal.lineStrong).Background(pal.card)
	if b.Hovered() {
		b.Background(pal.sel)
	}
	b.OnClick(a.togglePlay)
	b.Children(func() {
		text(c, glyph, 12, pal.text)
	})
}

func (a *app) timeline(c *ui.Context) {
	fraction := 0.0
	if a.proj.Duration > 0 {
		fraction = a.playhead / a.proj.Duration
	}
	tl := ui.Box(c).Grow(1).Height(64).Clip().Label("时间轴")
	tl.DrawOver(func(p *ui.Painter, r ui.Rect) {
		x := r.X + float32(fraction)*r.W
		p.Fill(ui.Rect{X: x - 1, Y: r.Y + 3, W: 2, H: r.H - 6}, pal.accent, 1)
		p.Fill(ui.Rect{X: x - 3.5, Y: r.Y + 1, W: 7, H: 4}, pal.accent, 2)
	})
	tl.Children(func() {
		const buckets = 12
		ui.Row(c).Height(19).FillWidth().Children(func() {
			for i := 0; i < buckets; i++ {
				at := (float64(i) + 0.5) / buckets * a.proj.Duration
				b, _, ok := a.barAt(at)
				cell := ui.Box(c).Grow(1).Center()
				cell.Children(func() {
					color := pal.faint
					weight := 500
					if fraction >= float64(i)/buckets && fraction < float64(i+1)/buckets {
						color = pal.accent
						weight = 700
					} else if b.NeedsReview {
						color = pal.warn
					}
					name := "—"
					if ok && len(b.Chords) > 0 {
						name = b.Chords[0]
					}
					mono(c, name, 8.5, color).FontWeight(weight)
				})
			}
		})
		wave := ui.Box(c).FillWidth().Height(28).Margin(0, 0, 0, 0)
		wave.Draw(func(p *ui.Painter, r ui.Rect) {
			drawWave(p, r, fraction)
		})
		ticks := ui.Row(c).FillWidth().Height(10).Gap(0)
		ticks.Children(func() {
			for i := 0; i < 24; i++ {
				ui.Box(c).Width(1).Height(4 + float32(i%4)*2).Background(pal.lineStrong)
				ui.Box(c).Grow(1)
			}
		})
	})
	if tl.Clicked() {
		if x, _, ok := tl.PointerPosition(); ok {
			bounds := tl.Bounds()
			if bounds.W > 0 {
				a.playhead = math.Max(0, math.Min(1, float64(x/bounds.W))) * a.proj.Duration
				a.seekPlayer()
				a.syncSelection()
			}
		}
	}
}

func drawWave(p *ui.Painter, r ui.Rect, fraction float64) {
	const count = 96
	gap := r.W / count
	barW := float32(math.Max(1, float64(gap*0.55)))
	for i := 0; i < count; i++ {
		v := 0.28 + 0.44*math.Abs(math.Sin(float64(i)*0.61)+0.62*math.Sin(float64(i)*0.17+1.1)) +
			0.14*math.Abs(math.Sin(float64(i)*1.7+0.4))
		if v > 1 {
			v = 1
		}
		h := float32(v) * r.H
		x := r.X + float32(i)*gap
		y := r.Y + (r.H-h)/2
		color := pal.wave
		if float64(i)/count < fraction {
			color = pal.accent
		}
		p.Fill(ui.Rect{X: x, Y: y, W: barW, H: h}, color, barW/2)
	}
}
