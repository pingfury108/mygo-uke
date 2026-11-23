package render

import (
	"math"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/project"
)

// LyricOverlay describes the current lyric line on a video frame.
type LyricOverlay struct {
	Project   *project.Project
	LineIndex int
	Time      float64
	Style     project.Style
	Accent    ui.Color
	Ink       ui.Color
}

// DrawLyricOverlay draws a karaoke-style line: the text appears one rune at
// a time, while chords sit over the runes they cover.
func DrawLyricOverlay(p *ui.Painter, frame ui.Rect, spec LyricOverlay) {
	if p == nil || spec.Project == nil || frame.W <= 0 || frame.H <= 0 {
		return
	}
	if spec.LineIndex < 0 || spec.LineIndex >= len(spec.Project.Lines) {
		return
	}
	line := spec.Project.Lines[spec.LineIndex]
	if strings.TrimSpace(line.Text) == "" {
		return
	}
	if spec.Time < line.Start || spec.Time > line.End {
		return
	}
	visible := line.VisibleRunes(spec.Time)
	current := line.CurrentRune(spec.Time)
	if visible <= 0 || current < 0 {
		return
	}

	runes := []rune(line.Text)
	size := float32(spec.Style.SubSize) * frame.H / 1920
	if size < frame.H*0.018 {
		size = frame.H * 0.018
	}
	maxWidth := frame.W * 0.78
	widths, total := measureRunes(p, runes, size)
	if total > maxWidth && total > 0 {
		size *= maxWidth / total
		if size < frame.H*0.016 {
			size = frame.H * 0.016
		}
		widths, total = measureRunes(p, runes, size)
	}

	offsets := make([]float32, len(runes)+1)
	for i, width := range widths {
		offsets[i+1] = offsets[i] + width
	}
	total = offsets[len(runes)]
	x0 := frame.X + (frame.W-total)/2
	if x0 < frame.X+frame.W*0.05 {
		x0 = frame.X + frame.W*0.05
	}

	top := float32(spec.Style.SubtitleY) / 100
	if top <= 0 {
		top = 0.78
	}
	top = min(max(top, 0.5), 0.84)
	textTop := frame.Y + frame.H*top
	chordSize := size * 0.50
	if chordSize < frame.H*0.011 {
		chordSize = frame.H * 0.011
	}
	pad := size * 0.34
	gap := size * 0.18
	chordHeight := chordSize * 1.25
	_, textHeight := p.MeasureText(0, lyricSpan("国", size, spec.Ink, 600))
	if textHeight <= 0 {
		textHeight = size
	}

	spans := spec.Project.LineChordSpans(spec.LineIndex)
	cardTop := textTop - pad
	cardHeight := textHeight + pad*2
	if len(spans) > 0 {
		cardTop = textTop - gap - chordHeight - pad
		cardHeight = chordHeight + gap + textHeight + pad*2
	}
	card := ui.Rect{
		X: x0 - pad,
		Y: cardTop,
		W: total + pad*2,
		H: cardHeight,
	}
	p.Fill(card, ui.RGBA(10, 9, 8, 0.68), size*0.22)

	if len(spans) > 0 {
		chordY := cardTop + pad
		activeSpan, hasActive := project.ChordAtRune(spans, current)
		for _, span := range spans {
			from, to := span.From, span.To
			if from < 0 {
				from = 0
			}
			if from >= len(runes) {
				continue
			}
			if to > len(runes) {
				to = len(runes)
			}
			if to <= from {
				continue
			}
			x := x0 + offsets[from]
			width := offsets[to] - offsets[from]
			active := hasActive && span.From == activeSpan.From && span.To == activeSpan.To
			color := withAlpha(spec.Accent, 0.42)
			if active {
				color = spec.Accent
				p.Fill(ui.Rect{X: x - size*0.04, Y: chordY - chordSize*0.08,
					W: width + size*0.08, H: chordHeight}, ui.RGBA(243, 181, 68, 0.14), size*0.1)
			}
			p.RichText(x, chordY, 0, lyricSpan(span.Label, chordSize, color, 650))
			lineY := chordY + chordHeight - chordSize*0.08
			p.Line(x, lineY, x+width, lineY, max(1, size*0.035), withAlpha(color, 0.62))
		}
	}

	times := line.RuneTimes()
	for i := 0; i < visible && i < len(runes); i++ {
		r := runes[i]
		if unicode.IsSpace(r) {
			continue
		}
		x := x0 + offsets[i]
		color := spec.Ink
		charSize := size
		y := textTop
		if i == current {
			age := spec.Time - times[i]
			alpha := float32(1)
			if age < 0.14 {
				alpha = float32(math.Max(0.18, age/0.14))
				charSize = size * (0.86 + 0.14*alpha)
				y += size * 0.10 * (1 - alpha)
			}
			color = withAlpha(spec.Accent, alpha)
		}
		p.RichText(x, y, 0, lyricSpan(string(r), charSize, color, 600))
	}
}

func measureRunes(p *ui.Painter, runes []rune, size float32) ([]float32, float32) {
	widths := make([]float32, len(runes))
	total := float32(0)
	for i, r := range runes {
		width, _ := p.MeasureText(0, lyricSpan(string(r), size, ui.Transparent, 600))
		if width <= 0 && unicode.IsSpace(r) {
			width = size * 0.32
		}
		widths[i] = width
		total += width
	}
	return widths, total
}

func lyricSpan(value string, size float32, color ui.Color, weight int) ui.Span {
	return ui.Span{
		Text:   value,
		Font:   fontFamily,
		Size:   size,
		Weight: weight,
		Color:  color,
	}
}

func withAlpha(color ui.Color, alpha float32) ui.Color {
	alpha = min(max(alpha, 0), 1)
	color.A = uint8(math.Round(float64(alpha) * 255))
	return color
}
