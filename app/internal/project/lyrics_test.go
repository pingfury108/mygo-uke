package project

import (
	"testing"

	"mygo-uke/internal/score"
)

func TestLineRevealsCharactersOverTime(t *testing.T) {
	line := Line{Start: 10, End: 14, Text: "好想能看到"}
	times := line.RuneTimes()
	if len(times) != 5 || times[0] != 10 || times[4] != 13.2 {
		t.Fatalf("times = %v", times)
	}
	if got := line.TextAt(10.4); got != "好" {
		t.Errorf("TextAt(10.4) = %q", got)
	}
	if got := line.TextAt(11); got != "好想" {
		t.Errorf("TextAt(11) = %q", got)
	}
	if got := line.TextAt(14); got != "好想能看到" {
		t.Errorf("TextAt(14) = %q", got)
	}
}

func TestPunctuationAppearsWithPreviousCharacter(t *testing.T) {
	line := Line{Start: 0, End: 2, Text: "好，想"}
	if got := line.TextAt(0.4); got != "好，" {
		t.Fatalf("TextAt(0.4) = %q", got)
	}
}

func TestRuneTimesUsesCharTimes(t *testing.T) {
	line := Line{
		Start:     0,
		End:       10,
		Text:      "好想",
		CharTimes: []float64{1, 4},
	}
	times := line.RuneTimes()
	if len(times) != 2 || times[0] != 1 || times[1] != 4 {
		t.Fatalf("times = %v", times)
	}
}

func TestLineChordSpansCoverCharacterRanges(t *testing.T) {
	p := New("test")
	p.Lines = []Line{{
		Start: 0, End: 10, Text: "好想能看到你嘴角微笑",
		Chords: []ChordMark{{At: 0, Label: "F"}, {At: 5, Label: "G"}},
	}}
	p.Chords = []score.ChordSegment{
		{Start: 0, End: 5, Label: "F", Source: "chart"},
		{Start: 5, End: 10, Label: "G", Source: "chart"},
	}
	spans := p.LineChordSpans(0)
	if len(spans) != 2 {
		t.Fatalf("spans = %+v", spans)
	}
	if spans[0].From != 0 || spans[0].To != 5 || spans[0].Label != "F" {
		t.Errorf("span 0 = %+v", spans[0])
	}
	if spans[1].From != 5 || spans[1].To != 10 || spans[1].Label != "G" {
		t.Errorf("span 1 = %+v", spans[1])
	}
}

func TestLineChordSpansInheritPreviousChord(t *testing.T) {
	p := New("test")
	p.Lines = []Line{{
		Start: 0, End: 4, Text: "好想能看到",
		Chords: []ChordMark{{At: 2, Label: "G"}},
	}}
	p.Chords = []score.ChordSegment{{Start: 0, End: 4, Label: "F", Source: "chart"}}
	spans := p.LineChordSpans(0)
	if len(spans) != 2 || !spans[0].Inherited || spans[0].Label != "F" || spans[0].To != 2 {
		t.Fatalf("spans = %+v", spans)
	}
}

func TestCleanChordMarksDistributesUnknownPositions(t *testing.T) {
	marks := CleanChordMarks([]ChordMark{
		{At: -1, Label: "F"},
		{At: -1, Label: "G"},
	}, "好想能看到")
	if len(marks) != 2 || marks[0].At != 0 || marks[1].At != 4 {
		t.Fatalf("marks = %+v", marks)
	}
	if !marks[0].Approx || !marks[1].Approx {
		t.Fatalf("expected approximate marks: %+v", marks)
	}
}

func TestRebuildChartChordsKeepsInstrumentalSegments(t *testing.T) {
	p := New("test")
	p.Duration = 20
	p.Lines = []Line{{
		Start: 5, End: 10, Text: "好想能看到",
		Chords: []ChordMark{{At: 0, Label: "G"}},
	}}
	p.Chords = []score.ChordSegment{
		{Start: 0, End: 4, Label: "C", Source: "chart"},
		{Start: 5, End: 10, Label: "F", Source: "chart"},
	}
	p.RebuildChartChords()
	var labels []string
	for _, segment := range p.Chords {
		labels = append(labels, segment.Label)
	}
	if len(labels) != 2 || labels[0] != "C" || labels[1] != "G" {
		t.Fatalf("chords = %+v", p.Chords)
	}
}

func TestMergeKeepsChartChordsOverAudio(t *testing.T) {
	edited := New("test")
	edited.Duration = 4
	edited.Lines = []Line{{
		Start: 0, End: 4, Text: "好想能看到",
		Chords: []ChordMark{{At: 0, Label: "F"}, {At: 2, Label: "G"}},
	}}
	edited.LinesEdited = true
	edited.Chords = []score.ChordSegment{
		{Start: 0, End: 2, Label: "F", Source: "chart"},
		{Start: 2, End: 4, Label: "G", Source: "chart"},
	}
	built := New("test")
	built.Duration = 4
	built.Beats = []float64{0, 1, 2, 3, 4}
	built.Downbeats = []float64{0}
	built.Lines = []Line{{Start: 0, End: 4, Text: "旧字幕"}}
	built.Chords = []score.ChordSegment{{Start: 0, End: 4, Label: "C", Source: "audio"}}
	built.RebuildBars()

	merged := Merge(edited, built)
	if len(merged.Chords) != 2 || merged.Chords[0].Label != "F" || merged.Chords[1].Label != "G" {
		t.Fatalf("chords = %+v", merged.Chords)
	}
	for _, segment := range merged.Chords {
		if segment.Source != "chart" {
			t.Fatalf("audio chord survived: %+v", merged.Chords)
		}
	}
	if len(merged.Lines) != 1 || len(merged.Lines[0].Chords) != 2 {
		t.Fatalf("lines = %+v", merged.Lines)
	}
}

func TestSetChartChordAtUpdatesCharacterAnchor(t *testing.T) {
	p := New("test")
	p.Duration = 4
	p.Lines = []Line{{
		Start: 0, End: 4, Text: "好想能看到",
		Chords: []ChordMark{{At: 0, Label: "F"}},
	}}
	p.Chords = []score.ChordSegment{{Start: 0, End: 4, Label: "F", Source: "chart"}}
	if !p.SetChartChordAt(2.1, "G") {
		t.Fatal("SetChartChordAt returned false")
	}
	if len(p.Lines[0].Chords) != 2 || p.Lines[0].Chords[1].At != 2 || p.Lines[0].Chords[1].Label != "G" {
		t.Fatalf("marks = %+v", p.Lines[0].Chords)
	}
	p.RebuildChartChords()
	if len(p.Chords) != 2 || p.Chords[0].Label != "F" || p.Chords[1].Label != "G" {
		t.Fatalf("chords = %+v", p.Chords)
	}
}
