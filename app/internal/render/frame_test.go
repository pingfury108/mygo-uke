package render

import (
	"testing"

	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
)

func sample() *project.Project {
	p := project.New("渲染测试")
	p.Duration = 9
	p.Tempo = 120
	p.Key = "C"
	p.Downbeats = []float64{0, 3, 6}
	p.Chords = []score.ChordSegment{
		{Start: 0, End: 3, Label: "C", Conf: 0.9},
		{Start: 3, End: 6, Label: "G", Conf: 0.9},
		{Start: 6, End: 9, Label: "Am", Conf: 0.9},
	}
	p.Lines = []project.Line{
		{Start: 0, End: 3, Text: "第一句"},
		{Start: 3, End: 6, Text: "第二句"},
		{Start: 6, End: 9, Text: "第三句"},
	}
	p.RebuildBars()
	return p
}

func TestBarAndLineAt(t *testing.T) {
	p := sample()
	bar, index := barAt(p, 4)
	if index != 1 || len(bar.Chords) == 0 || bar.Chords[0] != "G" {
		t.Fatalf("barAt(4) = %#v index=%d", bar, index)
	}
	line, lineIndex, ok := lineAt(p, 4)
	if !ok || lineIndex != 1 || line.Text != "第二句" {
		t.Fatalf("lineAt(4) = %#v index=%d ok=%v", line, lineIndex, ok)
	}
	if got := upcomingLabels(p, 0, 2); len(got) != 2 || got[0] != "G" || got[1] != "Am" {
		t.Fatalf("upcomingLabels = %#v", got)
	}
}

func TestFrameVerticalAndHorizontal(t *testing.T) {
	p := sample()
	for _, item := range []struct {
		name        string
		w, h        int
		vw, vh      int
		orientation int
	}{
		{"vertical", 540, 960, 540, 960, 0},
		{"horizontal", 960, 540, 304, 540, 1},
	} {
		t.Run(item.name, func(t *testing.T) {
			img := Frame(Spec{
				Width:       item.w,
				Height:      item.h,
				VideoWidth:  item.vw,
				VideoHeight: item.vh,
				Project:     p,
				Time:        4,
				Orientation: item.orientation,
			})
			bounds := img.Bounds()
			if bounds.Dx() != item.w || bounds.Dy() != item.h {
				t.Fatalf("bounds = %v", bounds)
			}
			if _, _, _, a := img.At(item.w/2, item.h/2).RGBA(); a == 0 {
				t.Fatal("frame is transparent")
			}
		})
	}
}
