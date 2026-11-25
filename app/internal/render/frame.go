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
// for the layout (full frame for vertical, a portrait strip for horizontal).
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
	canvas    = "#0F1013"
)

// Frame draws one full frame. Vertical is a 9:16 canvas with a top chord
// strip and a configurable subtitle near the lower third. Horizontal keeps
// the portrait video centered and gives both sides to the current chord
// and the score.
func Frame(spec Spec) image.Image {
	spec = normalized(spec)
	view := func(c *ui.Context) {
		ui.Box(c).Size(float32(spec.Width), float32(spec.Height)).
			Background(ui.Hex(canvas)).Children(func() {
			Backdrop(c, spec)
			Overlay(c, spec)
		})
	}
	tester := ui.NewTester(view, spec.Width, spec.Height)
	tester.Frame()
	return tester.Image()
}

// Backdrop draws what sits below the overlay: the side panels of the
// horizontal layout and the decoded video frame.
func Backdrop(c *ui.Context, spec Spec) {
	spec = normalized(spec)
	w, h := float32(spec.Width), float32(spec.Height)
	videoW := float32(spec.VideoWidth)
	videoH := float32(spec.VideoHeight)
	videoX := float32(0)
	if spec.Orientation == 1 {
		videoX = (w - videoW) / 2
		sideW := videoX
		ui.Box(c).Absolute().Left(0).Top(0).Width(w).Height(h).
			Draw(func(painter *ui.Painter, r ui.Rect) {
				painter.Fill(r, ui.Hex(panel), 0)
				if sideW > 0 {
					painter.Line(sideW, 0, sideW, h, 1, ui.Hex(panelEdge))
					painter.Line(w-sideW, 0, w-sideW, h, 1, ui.Hex(panelEdge))
				}
			})
	}
	if spec.Background != nil {
		ui.Image(c, spec.Background).Absolute().Left(videoX).Top(0).
			Size(videoW, videoH)
	}
}

// Overlay draws everything above the video: scrims, chords, fingering,
// lyrics and the score. It has no background of its own, so preview and
// GPU export can place it over any decoded frame.
func Overlay(c *ui.Context, spec Spec) {
	spec = normalized(spec)
	style := styleFor(spec.Project)
	accent := colorOr(style.Accent)
	accentInk := inkFor(style.Accent)
	if spec.Orientation == 1 {
		horizontal(c, spec, style, accent, accentInk)
		return
	}
	vertical(c, spec, style, accent, accentInk)
}

func normalized(spec Spec) Spec {
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
	return spec
}

// HorizontalVideoWidth returns the width of the portrait video centered
// in a horizontal frame of height: 9:16, rounded down to an even width
// for 4:2:0 chroma.
func HorizontalVideoWidth(height int) int {
	w := height * 607 / 1080
	return w - w%2
}

func vertical(c *ui.Context, spec Spec, style project.Style, accent, accentInk ui.Color) {
	w, h := float32(spec.Width), float32(spec.Height)
	scrim(c, ui.Rect{X: 0, Y: 0, W: w, H: h * 0.26}, true)
	scrim(c, ui.Rect{X: 0, Y: h * 0.55, W: w, H: h * 0.45}, false)

	p := spec.Project
	bar, _ := barAt(p, spec.Time)
	top := h * 0.054
	margin := w * 0.065
	right := w*0.75 - h*0.016
	size := float32(style.ChordSize) * h / 1920 * 1.32

	row := ui.Row(c).Absolute().Left(margin).Top(top).Width(right - margin).
		Gap(h * 0.010).AlignItems(ui.Start)
	row.Children(func() {
		labels, upcoming := chordLabelsAt(p, spec.Time, 2)
		for _, item := range labels {
			currentChip(c, item, size, p, accent, accentInk)
		}
		for _, item := range upcoming {
			nextChip(c, item, size*0.77)
		}
	})
	fingering(c, spec, chordLabelAt(p, spec.Time, bar), w, h, accent)

	if line, index, ok := lineAt(p, spec.Time); ok && strings.TrimSpace(line.Text) != "" &&
		spec.Time >= line.Start && spec.Time <= line.End {
		ui.Box(c).Absolute().Left(0).Top(0).Width(w).Height(h).
			Draw(func(painter *ui.Painter, r ui.Rect) {
				DrawLyricOverlay(painter, r, LyricOverlay{
					Project:   p,
					LineIndex: index,
					Time:      spec.Time,
					Style:     style,
					Accent:    accent,
					Ink:       ui.Hex(ink),
				})
			})
	}
}

func horizontal(c *ui.Context, spec Spec, style project.Style, accent, accentInk ui.Color) {
	w, h := float32(spec.Width), float32(spec.Height)
	videoW := float32(spec.VideoWidth)
	videoX := (w - videoW) / 2
	sideW := videoX

	p := spec.Project
	bar, index := barAt(p, spec.Time)
	pad := h * 0.032
	left := ui.Column(c).Absolute().Left(pad).Top(pad).Width(sideW - pad*2).
		Height(h - pad*2).Gap(h * 0.009)
	left.Children(func() {
		label(c, "当前和弦", h*0.021, ui.RGBA(255, 248, 234, 0.58), 500)
		name := chordLabelAt(p, spec.Time, bar)
		if name == "" {
			name = "—"
		}
		label(c, name, h*0.072, ui.Hex(ink), 650)
		mono(c, fmt.Sprintf("原调 %s · Capo %d", keyOf(p), capoOf(p)), h*0.019,
			ui.RGBA(255, 248, 234, 0.62))
		mono(c, fmt.Sprintf("第 %d 小节", index+1), h*0.018,
			ui.RGBA(255, 248, 234, 0.44))
		ui.Box(c).Height(h * 0.024)
		fingeringBox(c, chordLabelAt(p, spec.Time, bar), p, h*0.2, h*0.23, accent, accentInk)
	})

	rightX := videoX + videoW + pad
	right := ui.Column(c).Absolute().Left(rightX).Top(pad).Width(w - rightX - pad).
		Height(h - pad*2).Gap(h * 0.012)
	right.Children(func() {
		label(c, titleOf(p), h*0.028, ui.Hex(ink), 650).SingleLine()
		mono(c, fmt.Sprintf("%s · %d/%d · %s 调", tempoText(p),
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
			chords := chordSpanLabels(p.LineChordSpans(i))
			mono(c, chords, h*0.021, accent)
			size := float32(style.SubSize) * h / 1920
			if size < h*0.02 {
				size = h * 0.02
			}
			value := line.Text
			if i == active && spec.Time >= line.Start && spec.Time <= line.End {
				value = line.TextAt(spec.Time)
			}
			label(c, value, size, ui.Hex(ink), 500).SingleLine()
		})
	}
}

func chordSpanLabels(spans []project.ChordSpan) string {
	var labels []string
	for _, span := range spans {
		if len(labels) == 0 || labels[len(labels)-1] != span.Label {
			labels = append(labels, span.Label)
		}
	}
	if len(labels) == 0 {
		return "—"
	}
	return strings.Join(labels, "  ")
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

func currentChip(c *ui.Context, value string, size float32, p *project.Project, accent, inkColor ui.Color) {
	if value == "" {
		value = "—"
	}
	box := ui.Column(c).Padding(size*0.32, size*0.46, size*0.27, size*0.46).
		Radius(size*0.32).Background(accent).
		Shadow(0, size*0.45, size*1.1, 0, ui.RGBA(0, 0, 0, 0.24))
	box.Children(func() {
		label(c, value, size, inkColor, 720).SingleLine()
		mono(c, beatRow(p), size*0.30, withAlpha(inkColor, 0.66)).SingleLine()
	})
}

func nextChip(c *ui.Context, value string, size float32) {
	if value == "" {
		value = "—"
	}
	box := ui.Column(c).Padding(size*0.29, size*0.41, size*0.24, size*0.41).
		Radius(size*0.41).
		Border(max(1, size*0.06), ui.RGBA(255, 255, 255, 0.14)).
		Background(ui.RGBA(16, 14, 12, 0.42))
	box.Children(func() {
		label(c, value, size, ui.RGBA(255, 248, 234, 0.82), 720).SingleLine()
	})
}

// beatRow is the "1 · 2 · 3 · 4" strip under the current chord, as the
// preview has always shown it, generated from the project's time signature.
func beatRow(p *project.Project) string {
	beats := 4
	if p != nil && p.TimeSignature[0] > 0 && p.TimeSignature[0] <= 12 {
		beats = p.TimeSignature[0]
	}
	parts := make([]string, beats)
	for i := range parts {
		parts[i] = strconv.Itoa(i + 1)
	}
	return strings.Join(parts, " · ")
}

func fingering(c *ui.Context, spec Spec, labelValue string, w, h float32, accent ui.Color) {
	shape, ok := shapeFor(labelValue, spec.Project)
	boxW := w * 0.22
	pad := w * 0.018
	box := ui.Column(c).Absolute().Right(w*0.055).Top(h*0.052).Width(boxW).
		Padding(pad, pad, pad*0.8, pad).Radius(w*0.032).
		Border(max(1, w*0.0036), ui.RGBA(255, 255, 255, 0.14)).
		Background(ui.RGBA(16, 14, 12, 0.46)).Gap(w * 0.007)
	box.Children(func() {
		diagramBox := ui.Box(c).Height(boxW).FillWidth()
		diagramBox.Draw(func(painter *ui.Painter, r ui.Rect) {
			if ok {
				diagram(painter, r, shape, accent)
			}
		})
		if ok {
			text(c, shape.Name, boxW*0.113, ui.RGBA(255, 248, 234, 0.78)).
				TextAlign(ui.Center).Width(boxW - 2*pad)
		}
	})
}

func fingeringBox(
	c *ui.Context,
	labelValue string,
	p *project.Project,
	w, h float32,
	accent, labelColor ui.Color,
) {
	shape, ok := shapeFor(labelValue, p)
	box := ui.Box(c).Width(w).Height(h)
	box.Draw(func(painter *ui.Painter, r ui.Rect) {
		if ok {
			diagram(painter, r, shape, accent)
		}
	})
	if ok {
		text(c, shape.Name, h*0.07, labelColor).TextAlign(ui.Center).Width(w)
	}
}

func shapeFor(labelValue string, p *project.Project) (uke.Fingering, bool) {
	chord, ok := uke.Normalize(labelValue)
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
	unit := max(1, r.W/60)
	lineColor := ui.RGBA(255, 255, 255, 0.30)
	for i := 0; i < 4; i++ {
		x := left + spanX*float32(i)
		p.Line(x, top, x, bottom, unit, lineColor)
	}
	for k := 0; k <= 4; k++ {
		y := top + spanY*float32(k)
		if k == 0 {
			p.Line(left, y, right, y, 2*unit, ui.RGBA(255, 255, 255, 0.52))
		} else {
			p.Line(left, y, right, y, unit, lineColor)
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
			p.Text(x-2.5*unit, top-5*unit, "×", 7*unit, ui.RGBA(255, 248, 234, 0.62))
		case fret == 0:
			p.Stroke(ui.Rect{X: x - 2.5*unit, Y: top - 8*unit, W: 5 * unit, H: 5 * unit},
				ui.RGBA(255, 248, 234, 0.72), 2.5*unit, unit)
		case fret >= base && fret <= base+3:
			cy := top + spanY*(float32(fret-base)+0.5)
			p.Fill(ui.Rect{X: x - 3*unit, Y: cy - 3*unit, W: 6 * unit, H: 6 * unit},
				accent, 3*unit)
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

func chordLabelAt(p *project.Project, t float64, bar score.Bar) string {
	if p != nil {
		if segment, _, ok := score.ChordAt(p.Chords, t); ok {
			return segment.Label
		}
	}
	if len(bar.Chords) > 0 {
		return bar.Chords[0]
	}
	return ""
}

func chordLabelsAt(p *project.Project, t float64, upcoming int) (labels, next []string) {
	bar, _ := barAt(p, t)
	current := chordLabelAt(p, t, bar)
	if current == "" {
		labels = []string{"—"}
	} else {
		labels = []string{current}
	}
	if p == nil || upcoming <= 0 {
		return labels, nil
	}
	segment, index, ok := score.ChordAt(p.Chords, t)
	if !ok {
		index = len(p.Chords) - 1
		for i, item := range p.Chords {
			if item.Start >= t {
				index = i - 1
				break
			}
		}
	}
	last := segment.Label
	for i := index + 1; i < len(p.Chords) && len(next) < upcoming; i++ {
		label := p.Chords[i].Label
		if label == "" || label == last {
			continue
		}
		next = append(next, label)
		last = label
	}
	return labels, next
}

func lineAt(p *project.Project, t float64) (project.Line, int, bool) {
	if p == nil || len(p.Lines) == 0 {
		return project.Line{}, -1, false
	}
	for i, line := range p.Lines {
		if t >= line.Start && t < line.End {
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
	style := project.Style{ChordSize: 64, SubSize: 60, SubtitleY: 78, Accent: "#F3B544", Align: "stagger"}
	if p == nil {
		return style
	}
	if p.Style.ChordSize > 0 {
		style.ChordSize = p.Style.ChordSize
	}
	if p.Style.SubSize > 0 {
		style.SubSize = p.Style.SubSize
	}
	if p.Style.SubtitleY > 0 {
		style.SubtitleY = p.Style.SubtitleY
	}
	if p.Style.Accent != "" {
		style.Accent = p.Style.Accent
	}
	if p.Style.Align != "" {
		style.Align = p.Style.Align
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

func tempoText(p *project.Project) string {
	if p == nil || p.Tempo <= 0 {
		return "— BPM"
	}
	return fmt.Sprintf("%.0f BPM", p.Tempo)
}

func timeSig(p *project.Project) [2]int {
	if p == nil || p.TimeSignature[0] == 0 {
		return [2]int{4, 4}
	}
	return p.TimeSignature
}
