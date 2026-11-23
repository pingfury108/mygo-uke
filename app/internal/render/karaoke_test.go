package render

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/project"
)

func TestPhraseSpansRevealCharactersOneAtATime(t *testing.T) {
	line := project.Line{Start: 0, End: 5, Text: "好想能看到"}
	phrases := line.Phrases()
	ink := ui.RGBA(255, 248, 234, 1)
	accent := ui.RGBA(243, 181, 68, 1)

	spans := phraseSpans(line, phrases[0], 1.2, 40, ink, accent)
	if len(spans) != 2 {
		t.Fatalf("spans = %+v", spans)
	}
	if spans[0].Text != "好" || spans[0].Color != ink {
		t.Errorf("sung span = %+v", spans[0])
	}
	if spans[1].Text != "想" || spans[1].Color != accent {
		t.Errorf("current span = %+v", spans[1])
	}
}

func TestPhraseSpansMoveAccentToNewestCharacter(t *testing.T) {
	line := project.Line{Start: 0, End: 5, Text: "好想能看到"}
	phrases := line.Phrases()
	ink := ui.RGBA(255, 248, 234, 1)
	accent := ui.RGBA(243, 181, 68, 1)

	spans := phraseSpans(line, phrases[0], 3.5, 40, ink, accent)
	if len(spans) != 2 {
		t.Fatalf("spans = %+v", spans)
	}
	if spans[0].Text != "好想能" || spans[0].Color != ink {
		t.Errorf("sung span = %+v", spans[0])
	}
	if spans[1].Text != "看" || spans[1].Color != accent {
		t.Errorf("current span = %+v", spans[1])
	}
}

func TestDrawLyricOverlayDrawsCurrentPhrase(t *testing.T) {
	p := project.New("test")
	p.Lines = []project.Line{{
		Start: 0,
		End:   10,
		Text:  "好想能看到 你嘴角微笑",
		CharTimes: []float64{
			0, 1, 2, 3, 4, 4, 5, 6, 7, 8, 9,
		},
	}}

	blank := blankFrame()
	band := image.Rect(80, 680, 460, 860)
	if diffPixels(drawLyricFrame(p, -1), blank, band) != 0 {
		t.Fatal("overlay drew outside the line")
	}
	if diffPixels(drawLyricFrame(p, 6.5), blank, band) == 0 {
		t.Fatal("second phrase did not draw")
	}
}

func drawLyricFrame(p *project.Project, at float64) image.Image {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Absolute().Left(0).Top(0).Width(540).Height(960).
			Draw(func(painter *ui.Painter, r ui.Rect) {
				DrawLyricOverlay(painter, r, LyricOverlay{
					Project:   p,
					LineIndex: 0,
					Time:      at,
					Style:     p.Style,
					Accent:    ui.Hex("#F3B544"),
					Ink:       ui.Hex("#FFF8EA"),
				})
			})
	}, 540, 960)
	tt.Frame()
	return tt.Image()
}

func blankFrame() image.Image {
	tt := ui.NewTester(func(c *ui.Context) {}, 540, 960)
	tt.Frame()
	return tt.Image()
}

func diffPixels(a, b image.Image, rect image.Rectangle) int {
	n := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				n++
			}
		}
	}
	return n
}
