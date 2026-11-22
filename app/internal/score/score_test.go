package score

import "testing"

func TestBuildBarsVotesByOverlap(t *testing.T) {
	downbeats := []float64{0, 2, 4, 6}
	chords := []ChordSegment{
		{Start: 0, End: 1.6, Label: "C", Conf: 0.9},
		{Start: 1.6, End: 4, Label: "G", Conf: 0.8},
		{Start: 4, End: 6, Label: "Am", Conf: 0.85},
	}
	bars := BuildBars(downbeats, chords, 8)
	if len(bars) != 4 {
		t.Fatalf("got %d bars, want 4", len(bars))
	}
	if bars[0].Chords[0] != "C" {
		t.Errorf("bar 0 = %v, want C", bars[0].Chords)
	}
	if bars[1].Chords[0] != "G" {
		t.Errorf("bar 1 = %v, want G", bars[1].Chords)
	}
	if bars[2].Chords[0] != "Am" {
		t.Errorf("bar 2 = %v, want Am", bars[2].Chords)
	}
	if !bars[3].NeedsReview {
		t.Errorf("bar 3 has no chord and should need review")
	}
}

func TestBuildBarsKeepsLateSecondChord(t *testing.T) {
	bars := BuildBars(
		[]float64{0, 4},
		[]ChordSegment{
			{Start: 0, End: 3.0, Label: "C", Conf: 0.9},
			{Start: 3.0, End: 4, Label: "G", Conf: 0.9},
		},
		4,
	)
	if len(bars) != 1 {
		t.Fatalf("got %d bars, want 1", len(bars))
	}
	if len(bars[0].Chords) != 2 || bars[0].Chords[1] != "G" {
		t.Errorf("chords = %v, want [C G]", bars[0].Chords)
	}
}

func TestBarAtClamps(t *testing.T) {
	bars := BuildBars([]float64{1, 3, 5}, []ChordSegment{{Start: 0, End: 5, Label: "C", Conf: 1}}, 6)
	if b, i, ok := BarAt(bars, 0); !ok || i != 0 || b.Start != 1 {
		t.Errorf("BarAt(0) = %v %d %v", b, i, ok)
	}
	if b, i, ok := BarAt(bars, 99); !ok || i != 2 || b.Start != 5 {
		t.Errorf("BarAt(99) = %v %d %v", b, i, ok)
	}
}
