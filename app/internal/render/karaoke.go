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

// DrawLyricOverlay draws the rune being sung right now. The previous rune
// disappears when the next one starts, so the subtitle stays a single
// character instead of accumulating a whole sentence. Chords are intentionally
// not drawn here; the stage header already shows the current chord.
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
	current := line.CurrentRune(spec.Time)
	runes := []rune(line.Text)
	if current < 0 || current >= len(runes) {
		return
	}
	times := line.RuneTimes()
	next := line.End
	for i := current + 1; i < len(runes); i++ {
		if unicode.IsSpace(runes[i]) || unicode.IsPunct(runes[i]) {
			continue
		}
		if times[i] > times[current] {
			next = times[i]
			break
		}
	}
	if next <= spec.Time {
		next = spec.Time + 0.25
	}
	duration := next - times[current]
	if duration <= 0 {
		duration = 0.25
	}
	fadeIn := min(0.08, duration*0.35)
	fadeOut := min(0.08, duration*0.35)
	age := spec.Time - times[current]
	remaining := next - spec.Time
	alpha := float32(1)
	if fadeIn > 0 {
		alpha = min(alpha, float32(age/fadeIn))
	}
	if fadeOut > 0 {
		alpha = min(alpha, float32(remaining/fadeOut))
	}
	alpha = max(alpha, 0)
	if alpha <= 0.02 {
		return
	}
	scale := float32(1)
	if fadeIn > 0 && age < fadeIn {
		scale = 0.86 + 0.14*float32(age/fadeIn)
	}

	size := float32(spec.Style.SubSize) * frame.H / 1920
	if size < frame.H*0.018 {
		size = frame.H * 0.018
	}
	size *= scale
	glyph := string(runes[current])
	width, height := p.MeasureText(0, lyricSpan(glyph, size, ui.Transparent, 650))
	if width <= 0 {
		return
	}
	if height <= 0 {
		height = size
	}
	if width > frame.W*0.6 {
		size *= frame.W * 0.6 / width
		width, height = p.MeasureText(0, lyricSpan(glyph, size, ui.Transparent, 650))
	}

	top := float32(spec.Style.SubtitleY) / 100
	if top <= 0 {
		top = 0.78
	}
	top = min(max(top, 0.5), 0.84)
	x := frame.X + (frame.W-width)/2
	y := frame.Y + frame.H*top
	padX := size * 0.34
	padY := height * 0.16
	card := ui.Rect{
		X: x - padX,
		Y: y - padY,
		W: width + padX*2,
		H: height + padY*2,
	}
	p.Fill(card, withAlpha(ui.RGBA(10, 9, 8, 1), alpha*0.68), size*0.30)
	p.RichText(x, y, 0, lyricSpan(glyph, size, withAlpha(spec.Ink, alpha), 650))
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
