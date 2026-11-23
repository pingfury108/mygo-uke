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

const (
	// lyricPreviewScale and lyricPreviewAlpha size the next phrase like the
	// second line of an MV lyric: readable, but quieter than the current one.
	lyricPreviewScale = 0.78
	lyricPreviewAlpha = 0.60
	// lyricPanelAlpha and lyricPreviewPanelAlpha keep the current line's
	// backing darker than the next line's.
	lyricPanelAlpha        = 0.46
	lyricPreviewPanelAlpha = 0.24
	// lyricFade softens the change from one phrase to the next.
	lyricFade = 0.14
	// lyricSafeBottom keeps the block out of the platform's bottom bar.
	lyricSafeBottom = 0.85
	// lyricSideMargin is the left and right inset of left/right alignment.
	lyricSideMargin = 0.07
)

// DrawLyricOverlay draws a two-line MV lyric. The phrase being sung sits on
// top in a dark pill: characters appear one at a time and stay, the newest
// one in the accent color. The next phrase waits below in smaller, dimmer
// type. The pill and both lines follow the style's alignment.
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
	index, phrase, ok := line.PhraseAt(spec.Time)
	if !ok || strings.TrimSpace(phrase.Text) == "" {
		return
	}

	size := float32(spec.Style.SubSize) * frame.H / 1920
	if size < frame.H*0.018 {
		size = frame.H * 0.018
	}
	size = fitLyricSize(p, frame, phrase.Text, size)

	age := spec.Time - phrase.Start
	fade := float32(1)
	if age < lyricFade {
		fade = float32(min(max(age/lyricFade, 0), 1))
	}
	if fade <= 0.02 {
		return
	}

	block := lyricBlockFor(p, line, phrase, spec.Time, size, spec.Ink, spec.Accent, fade)

	_, nextPhrase, hasNext := spec.Project.NextPhrase(spec.LineIndex, index)
	nextSize := fitLyricSize(p, frame, nextPhrase.Text, size*lyricPreviewScale)
	var nextW, nextH float32
	if hasNext {
		nextW, nextH = p.MeasureText(0, previewSpan(nextPhrase.Text, nextSize, spec.Ink, 1))
	}

	align := spec.Style.Alignment()
	padX := size * 0.42
	padY := block.padY
	gap := size * 0.30
	nextPadX := nextSize * 0.38
	nextPadY := nextH * 0.16
	reserveNext := max(nextH+nextPadY*2, size*lyricPreviewScale*1.4)

	pillW := block.fullW + padX*2
	pillH := block.textH + padY*2
	var pillX float32
	switch align {
	case "left", "stagger":
		pillX = frame.X + frame.W*lyricSideMargin
	case "right":
		pillX = frame.X + frame.W*(1-lyricSideMargin) - pillW
	default:
		pillX = frame.X + (frame.W-pillW)/2
	}

	top := float32(spec.Style.SubtitleY) / 100
	if top <= 0 {
		top = 0.78
	}
	top = min(max(top, 0.5), 0.9)
	textY := frame.Y + frame.H*top
	bottom := frame.Y + frame.H*lyricSafeBottom
	if textY+pillH+gap+reserveNext > bottom {
		textY = bottom - pillH - gap - reserveNext
	}
	textY = max(textY, frame.Y+frame.H*0.45)

	p.Fill(ui.Rect{X: pillX, Y: textY - padY, W: pillW, H: pillH},
		withAlpha(ui.RGBA(10, 9, 8, 1), lyricPanelAlpha*fade), size*0.42)
	if block.textW > 0 {
		textX := pillX + padX
		if align == "center" {
			textX = pillX + (pillW-block.textW)/2
		}
		p.RichText(textX, textY, 0, block.spans...)
	}

	if hasNext {
		var nextX float32
		switch align {
		case "left":
			nextX = pillX + padX
		case "right":
			nextX = pillX + padX + block.fullW - nextW
		case "stagger":
			nextX = frame.X + frame.W*(1-lyricSideMargin) - nextW
		default:
			nextX = pillX + (pillW-nextW)/2
		}
		nextY := textY + block.textH + padY + gap
		p.Fill(ui.Rect{
			X: nextX - nextPadX,
			Y: nextY - nextPadY,
			W: nextW + nextPadX*2,
			H: nextH + nextPadY*2,
		}, withAlpha(ui.RGBA(10, 9, 8, 1), lyricPreviewPanelAlpha*fade), nextSize*0.42)
		p.RichText(nextX, nextY, 0,
			previewSpan(nextPhrase.Text, nextSize, spec.Ink, lyricPreviewAlpha*fade))
	}
}

type lyricBlock struct {
	spans []ui.Span
	textW float32
	fullW float32
	textH float32
	padY  float32
}

func lyricBlockFor(p *ui.Painter, line project.Line, phrase project.Phrase, at float64, size float32, ink, accent ui.Color, alpha float32) lyricBlock {
	fullW, textH := p.MeasureText(0, lyricSpan(phrase.Text, size, ink, 650))
	if textH <= 0 {
		textH = size
	}
	spans := phraseSpans(line, phrase, at, size, withAlpha(ink, alpha), withAlpha(accent, alpha))
	textW, _ := p.MeasureText(0, spans...)
	return lyricBlock{
		spans: spans,
		textW: textW,
		fullW: fullW,
		textH: textH,
		padY:  textH * 0.16,
	}
}

func fitLyricSize(p *ui.Painter, frame ui.Rect, value string, size float32) float32 {
	if strings.TrimSpace(value) == "" || size <= 0 {
		return size
	}
	width, _ := p.MeasureText(0, lyricSpan(value, size, ui.Transparent, 650))
	if maxWidth := frame.W * 0.86; width > maxWidth && width > 0 {
		size *= maxWidth / width
	}
	return size
}

// phraseSpans returns the runes of the phrase that have been sung by t, with
// the newest one in the accent color. Unsung runes are left out so the
// phrase appears one character at a time.
func phraseSpans(line project.Line, phrase project.Phrase, t float64, size float32, ink, accent ui.Color) []ui.Span {
	runes := []rune(line.Text)
	times := line.RuneTimes()
	current := line.CurrentRune(t)

	var spans []ui.Span
	var text strings.Builder
	color := ink
	flush := func() {
		if text.Len() == 0 {
			return
		}
		spans = append(spans, lyricSpan(text.String(), size, color, 650))
		text.Reset()
	}
	for i := phrase.From; i < phrase.To && i < len(runes); i++ {
		if times[i] > t+1e-9 {
			break
		}
		if unicode.IsSpace(runes[i]) {
			continue
		}
		next := ink
		if i == current {
			next = accent
		}
		if next != color {
			flush()
			color = next
		}
		text.WriteRune(runes[i])
	}
	flush()
	return spans
}

func previewSpan(value string, size float32, color ui.Color, alpha float32) ui.Span {
	span := lyricSpan(value, size, withAlpha(color, alpha), 600)
	span.Shadow = ui.NewTextShadow(0, size*0.035, size*0.16, withAlpha(ui.RGBA(0, 0, 0, 1), 0.55*alpha))
	return span
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
