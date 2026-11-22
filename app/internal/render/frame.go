// Package render draws the video overlay shared by preview and export.
package render

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
	"mygo-uke/internal/uke"
)

func text(c *ui.Context, value string, size float32, color ui.Color) ui.Element {
	return withFont(ui.Text(c, value).FontSize(size).TextColor(color))
}

func mono(c *ui.Context, value string, size float32, color ui.Color) ui.Element {
	return ui.Text(c, value).FontSize(size).TextColor(color).Font("monospace")
}

func label(c *ui.Context, value string, size float32, color ui.Color, weight int) ui.Element {
	return withFont(ui.Text(c, value).FontSize(size).TextColor(color).FontWeight(weight))
}

// Spec is one output frame. Background is the decoded video, already scaled
// for the layout (full frame for vertical, 607×1080 for horizontal).
type Spec struct {
	Width       int
	Height      int
	VideoWidth  int
	VideoHeight int
	Background  *ui.Bitmap
	Project     *project.Project
	Time        float64
	Orientation int
}

const (
	ink       = "#FFF8EA"
	panel     = "#17181C"
	panelEdge = "#2A2C31"
)

// Frame draws one full frame. Vertical is a 9:16 canvas with a top chord
// strip and a subtitle at 70% height. Horizontal keeps the 607×1080 portrait
// video centered and gives both sides to the current chord and the score.
func Frame(spec Spec) image.Image {
	if spec.Width <= 0 {
		spec.Width = 1080
	}
	if spec.Height <= 0 {
		spec.Height = 1920
	}
	if spec.VideoWidth <= 0 {
		spec.VideoWidth = spec.Width
	}
	if spec.VideoHeight <= 0 {
		spec.VideoHeight = spec.Height
	}
	style := styleFor(spec.Project)
	accent := colorOr(style.Accent)
	accentInk := inkFor(style.Accent)

	view := func(c *ui.Context) {
		ui.Box(c).Size(float32(spec.Width), float32(spec.Height)).
			Background(ui.Hex("#0F1013")).Children(func() {
			if spec.Orientation == 1 {
				horizontal(c, spec, style, accent, accentInk)
				return
			}
			vertical(c, spec, style, accent, accentInk)
		})
	}
	tester := ui.NewTester(view, spec.Width, spec.Height)
	tester.Frame()
	return tester.Image()
}

func vertical(c *ui.Context, spec Spec, style project.Style, accent, accentInk ui.Color) {
	w, h := float32(spec.Width), float32(spec.Height)
	scrim(c, ui.Rect{X: 0, Y: 0, W: w, H: h * 0.26}, true)
	scrim(c, ui.Rect{X: 0, Y: h * 0.55, W: w, H: h * 0.45}, false)
	if spec.Background != nil {
		ui.Image(c, spec.Background).Absolute().Left(0).Top(0).Size(w, h)
	}

	p := spec.Project
	bar, index := barAt(p, spec.Time)
	right := w*0.889 - w*0.09
	top := h * 0.056
	chipH := h * 0.047
	currentW := h * 0.053
	margin := w * 0.065

	row := ui.Row(c).Absolute().Left(margin).Top(top).Width(right - margin).
		Gap(w * 0.012).AlignItems(ui.Start)
	row.Children(func() {
		labels := barChords(bar)
		if len(labels) == 0 {
			labels = []string{"—"}
		}
		for i, item := range labels {
			chip(c, item, currentW, chipH, float32(style.ChordSize)*h/1920,
				i == 0, accent, accentInk)
		}
		for _, item := range upcomingLabels(p, index, 2) {
			chip(c, item, h*0.044, chipH*0.82, float32(style.ChordSize)*0.72*h/1920,
				false, accent, accentInk)
		}
	})
	fingering(c, spec, bar, top, w, h, accent)

	if line, _, ok := lineAt(p, spec.Time); ok && strings.TrimSpace(line.Text) != "" {
		subtitle(c, line.Text, spec, style)
	}
}

func horizontal(c *ui.Context, spec Spec, style project.Style, accent, accentInk ui.Color) {
	w, h := float32(spec.Width), float32(spec.Height)
	videoW := float32(spec.VideoWidth)
	videoH := float32(spec.VideoHeight)
	videoX := (w - videoW) / 2
	sideW := videoX

	ui.Box(c).Absolute().Left(0).Top(0).Width(w).Height(h).
		Draw(func(painter *ui.Painter, r ui.Rect) {
			painter.Fill(r, ui.Hex(panel), 0)
			if sideW > 0 {
				painter.Line(sideW, 0, sideW, h, 1, ui.Hex(panelEdge))
				painter.Line(w-sideW, 0, w-sideW, h, 1, ui.Hex(panelEdge))
			}
		})
	if spec.Background != nil {
		ui.Image(c, spec.Background).Absolute().Left(videoX).Top(0).Size(videoW, videoH)
	}

	p := spec.Project
	bar, index := barAt(p, spec.Time)
	pad := h * 0.032
	left := ui.Column(c).Absolute().Left(pad).Top(pad).Width(sideW - pad*2).
		Height(h - pad*2).Gap(h * 0.009)
	left.Children(func() {
		label(c, "当前和弦", h*0.021, ui.RGBA(255, 248, 234, 0.58), 500)
		name := "—"
		if len(bar.Chords) > 0 {
			name = bar.Chords[0]
		}
		label(c, name, h*0.072, ui.Hex(ink), 650)
		mono(c, fmt.Sprintf("原调 %s · Capo %d", keyOf(p), capoOf(p)), h*0.019,
			ui.RGBA(255, 248, 234, 0.62))
		mono(c, fmt.Sprintf("第 %d 小节 · %s", index+1, patternOf(p)), h*0.018,
			ui.RGBA(255, 248, 234, 0.44))
		ui.Box(c).Height(h * 0.024)
		fingeringBox(c, bar, p, h*0.2, h*0.23, accent, accentInk)
	})

	rightX := videoX + videoW + pad
	right := ui.Column(c).Absolute().Left(rightX).Top(pad).Width(w - rightX - pad).
		Height(h - pad*2).Gap(h * 0.012)
	right.Children(func() {
		label(c, titleOf(p), h*0.028, ui.Hex(ink), 650).SingleLine()
		mono(c, fmt.Sprintf("%.0f BPM · %d/%d · %s 调", tempoOf(p),
			timeSig(p)[0], timeSig(p)[1], keyOf(p)), h*0.018,
			ui.RGBA(255, 248, 234, 0.46))
		ui.Box(c).Height(h * 0.008)
		scoreLines(c, spec, style, accent, h)
	})
}

func scoreLines(c *ui.Context, spec Spec, style project.Style, accent ui.Color, h float32) {
	p := spec.Project
	if len(p.Lines) == 0 {
		barRows(c, p, spec.Time, h, accent)
		return
	}
	_, active, _ := lineAt(p, spec.Time)
	for i := active - 2; i <= active+3; i++ {
		if i < 0 || i >= len(p.Lines) {
			continue
		}
		line := p.Lines[i]
		row := ui.Column(c).FillWidth().Gap(h * 0.005).Padding(h * 0.009).
			Radius(h * 0.007)
		if i == active {
			row.Background(ui.RGBA(243, 181, 68, 0.14))
		}
		row.Children(func() {
			chords := "—"
			if bar, _, ok := score.BarAt(p.Bars, line.Start); ok {
				chords = strings.Join(barChords(bar), "  ")
			}
			mono(c, chords, h*0.021, accent)
			size := float32(style.SubSize) * h / 1920
			if size < h*0.02 {
				size = h * 0.02
			}
			label(c, line.Text, size, ui.Hex(ink), 500).SingleLine()
		})
	}
}

func barRows(c *ui.Context, p *project.Project, t float64, h float32, accent ui.Color) {
	_, active := barAt(p, t)
	for i := active - 1; i <= active+5; i++ {
		if i < 0 || i >= len(p.Bars) {
			continue
		}
		bar := p.Bars[i]
		row := ui.Row(c).FillWidth().Gap(h * 0.012).AlignItems(ui.Center).
			Padding(h * 0.008).Radius(h * 0.006)
		if i == active {
			row.Background(ui.RGBA(243, 181, 68, 0.14))
		}
		row.Children(func() {
			mono(c, fmt.Sprintf("%02d", i+1), h*0.018, ui.RGBA(255, 248, 234, 0.38))
			label(c, strings.Join(barChords(bar), " / "), h*0.024, accent, 600)
			ui.Box(c).Grow(1)
			mono(c, score.TimeText(bar.Start), h*0.018, ui.RGBA(255, 248, 234, 0.42))
		})
	}
}

func subtitle(c *ui.Context, value string, spec Spec, style project.Style) {
	w, h := float32(spec.Width), float32(spec.Height)
	left := w * 0.055
	right := w * 0.111
	usable := w - left - right
	size := float32(style.SubSize) * h / 1920
	if size < h*0.018 {
		size = h * 0.018
	}
	n := float32(len([]rune(value)))
	if n > 0 && size*n > usable*0.92 {
		size = usable * 0.92 / n
	}
	if size < h*0.016 {
		size = h * 0.016
	}
	ui.Box(c).Absolute().Left(left).Right(right).Top(h * 0.70).Children(func() {
		ui.Row(c).Center().Children(func() {
			ui.Row(c).Padding(h*0.009, h*0.012, h*0.009, h*0.012).
				Radius(h * 0.007).Background(ui.RGBA(10, 9, 8, 0.66)).Children(func() {
				label(c, value, size, ui.Hex(ink), 600).
					TextAlign(ui.Center).Width(usable - h*0.024)
			})
		})
	})
}

func chip(c *ui.Context, value string, w, h, size float32, current bool, accent, inkColor ui.Color) {
	if value == "" {
		value = "—"
	}
	if size < h*0.28 {
		size = h * 0.28
	}
	box := ui.Column(c).Width(w).Height(h).Radius(h * 0.16).Center()
	if current {
		box.Background(accent)
	} else {
		box.Border(1, ui.RGBA(255, 255, 255, 0.20)).
			Background(ui.RGBA(16, 14, 12, 0.64))
	}
	color := ui.RGBA(255, 248, 234, 0.82)
	if current {
		color = inkColor
	}
	box.Children(func() {
		label(c, value, size, color, 720).SingleLine()
	})
}

func fingering(c *ui.Context, spec Spec, bar score.Bar, top, w, h float32, accent ui.Color) {
	boxW := w * 0.09
	box := ui.Column(c).Absolute().Right(w*0.111).Top(top).Width(boxW).
		Padding(h*0.005).Radius(h*0.007).Gap(h*0.002).
		Border(1, ui.RGBA(255, 255, 255, 0.20)).
		Background(ui.RGBA(16, 14, 12, 0.68))
	box.Children(func() {
		fingeringBox(c, bar, spec.Project, boxW-h*0.01, h*0.062, accent,
			ui.RGBA(255, 248, 234, 0.62))
	})
}

func fingeringBox(
	c *ui.Context,
	bar score.Bar,
	p *project.Project,
	w, h float32,
	accent, labelColor ui.Color,
) {
	shape, ok := shapeFor(bar, p)
	box := ui.Box(c).Width(w).Height(h)
	box.Draw(func(painter *ui.Painter, r ui.Rect) {
		if ok {
			diagram(painter, r, shape, accent)
			return
		}
		painter.Text(r.X+r.W/2-7, r.Y+r.H/2+5, "?", 16, ui.RGBA(255, 248, 234, 0.44))
	})
	if ok {
		text(c, shape.Name, h*0.07, labelColor).TextAlign(ui.Center).Width(w)
	}
}

func shapeFor(bar score.Bar, p *project.Project) (uke.Fingering, bool) {
	if len(bar.Chords) == 0 {
		return uke.Fingering{}, false
	}
	chord, ok := uke.Normalize(bar.Chords[0])
	if !ok {
		return uke.Fingering{}, false
	}
	return uke.Shape(chord, capoOf(p))
}

func diagram(p *ui.Painter, r ui.Rect, shape uke.Fingering, accent ui.Color) {
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
			p.Stroke(ui.Rect{X: x - 2.5, Y: top - 8, W: 5, H: 5},
				ui.RGBA(255, 248, 234, 0.72), 2.5, 1)
		case fret >= base && fret <= base+3:
			cy := top + spanY*(float32(fret-base)+0.5)
			p.Fill(ui.Rect{X: x - 3, Y: cy - 3, W: 6, H: 6}, accent, 3)
		}
	}
}

func scrim(c *ui.Context, r ui.Rect, top bool) {
	ui.Box(c).Absolute().Left(r.X).Top(r.Y).Width(r.W).Height(r.H).
		Draw(func(p *ui.Painter, _ ui.Rect) {
			if top {
				p.FillGradient(r, ui.LinearGradient{
					Stops: []ui.GradientStop{
						{Color: ui.RGBA(0, 0, 0, 0.52), At: 0},
						{Color: ui.RGBA(0, 0, 0, 0), At: 1},
					},
					Angle: 180,
				}, 0)
				return
			}
			p.FillGradient(r, ui.LinearGradient{
				Stops: []ui.GradientStop{
					{Color: ui.RGBA(0, 0, 0, 0), At: 0},
					{Color: ui.RGBA(0, 0, 0, 0.48), At: 1},
				},
				Angle: 180,
			}, 0)
		})
}

func barAt(p *project.Project, t float64) (score.Bar, int) {
	if p == nil {
		return score.Bar{}, 0
	}
	if bar, index, ok := score.BarAt(p.Bars, t); ok {
		return bar, index
	}
	return score.Bar{}, 0
}

func barChords(bar score.Bar) []string {
	if len(bar.Chords) == 0 {
		return nil
	}
	return bar.Chords
}

func upcomingLabels(p *project.Project, index, n int) []string {
	if p == nil || n <= 0 {
		return nil
	}
	var out []string
	for i := index + 1; i < len(p.Bars) && len(out) < n; i++ {
		out = append(out, barChords(p.Bars[i])...)
	}
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func lineAt(p *project.Project, t float64) (project.Line, int, bool) {
	if p == nil || len(p.Lines) == 0 {
		return project.Line{}, -1, false
	}
	for i, line := range p.Lines {
		if t >= line.Start && t <= line.End {
			return line, i, true
		}
	}
	index := 0
	for i, line := range p.Lines {
		if line.Start <= t {
			index = i
		}
	}
	return p.Lines[index], index, true
}

func styleFor(p *project.Project) project.Style {
	style := project.Style{ChordSize: 64, SubSize: 60, Accent: "#F3B544"}
	if p == nil {
		return style
	}
	if p.Style.ChordSize > 0 {
		style.ChordSize = p.Style.ChordSize
	}
	if p.Style.SubSize > 0 {
		style.SubSize = p.Style.SubSize
	}
	if p.Style.Accent != "" {
		style.Accent = p.Style.Accent
	}
	return style
}

func colorOr(hex string) ui.Color {
	if hex == "" {
		hex = "#F3B544"
	}
	return ui.Hex(hex)
}

func inkFor(hex string) ui.Color {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 6 {
		if value, err := strconv.ParseUint(hex, 16, 32); err == nil {
			r := float64(value>>16&0xFF) / 255
			g := float64(value>>8&0xFF) / 255
			b := float64(value&0xFF) / 255
			if 0.299*r+0.587*g+0.114*b < 0.52 {
				return ui.RGBA(255, 248, 240, 0.96)
			}
		}
	}
	return ui.Hex("#241A08")
}

// InkOn returns readable text for an accent background.
func InkOn(hex string) ui.Color { return inkFor(hex) }

func keyOf(p *project.Project) string {
	if p == nil || p.Key == "" {
		return "—"
	}
	return p.Key
}

func capoOf(p *project.Project) int {
	if p == nil {
		return 0
	}
	return p.Capo
}

func titleOf(p *project.Project) string {
	if p == nil || p.Title == "" {
		return "未命名"
	}
	return p.Title
}

func tempoOf(p *project.Project) float64 {
	if p == nil {
		return 0
	}
	return p.Tempo
}

func timeSig(p *project.Project) [2]int {
	if p == nil || p.TimeSignature[0] == 0 {
		return [2]int{4, 4}
	}
	return p.TimeSignature
}

func patternOf(p *project.Project) string {
	if p == nil || p.Pattern == "" {
		return "—"
	}
	return p.Pattern
}
