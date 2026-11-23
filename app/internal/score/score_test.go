package score

import (
	"math"
	"reflect"
	"testing"
)

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

func TestStabilizeDownbeatsKeepsHealthyTrack(t *testing.T) {
	beats := steps(0, 0.5, 81)
	downbeats := steps(0, 2, 21)
	got := StabilizeDownbeats(beats, downbeats, 40)
	if !reflect.DeepEqual(got, downbeats) {
		t.Errorf("healthy track changed: %v", got)
	}
}

func TestStabilizeDownbeatsRebuildsFragmentedTrack(t *testing.T) {
	beats := steps(0, 0.5, 81)
	downbeats := []float64{
		0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5,
		6, 7.5, 9, 10.5, 12, 13.5, 15, 16.5, 18, 19.5,
		21, 22.5, 24, 25.5, 27, 28.5, 30, 31.5, 33, 34.5,
		36, 37.5, 39,
	}
	got := StabilizeDownbeats(beats, downbeats, 40)
	if len(got) >= len(downbeats) {
		t.Fatalf("fragmented track not rebuilt: %v", got)
	}
	if math.Abs(got[0]) > 1e-9 {
		t.Errorf("grid starts at %v, want 0", got[0])
	}
	for i := 1; i < len(got); i++ {
		if gap := got[i] - got[i-1]; math.Abs(gap-1.5) > 1e-9 {
			t.Fatalf("gap %d = %v, want 1.5", i, gap)
		}
	}
}

func TestStabilizeDownbeatsLeavesShortInputAlone(t *testing.T) {
	beats := []float64{0, 1, 2}
	downbeats := []float64{0, 1, 2}
	if got := StabilizeDownbeats(beats, downbeats, 3); !reflect.DeepEqual(got, downbeats) {
		t.Errorf("short input changed: %v", got)
	}
}

func TestSnapToBeatsMovesCloseEdges(t *testing.T) {
	beats := []float64{0, 0.5, 1, 1.5}
	got := SnapToBeats(
		beats,
		[]ChordSegment{{Start: 0.06, End: 0.98, Label: "C", Conf: 1}},
		0.15,
	)
	if got[0].Start != 0 || got[0].End != 1 {
		t.Errorf("edges = %v-%v, want 0-1", got[0].Start, got[0].End)
	}
}

func TestVoteBarAggregatesCandidates(t *testing.T) {
	bars := BuildBars(
		[]float64{0, 2},
		[]ChordSegment{
			{Start: 0, End: 1.5, Label: "C", Conf: 0.8, Candidates: []string{"G", "Am"}},
			{Start: 1.5, End: 2, Label: "C", Conf: 0.8, Candidates: []string{"Am"}},
		},
		2,
	)
	if len(bars) != 1 {
		t.Fatalf("got %d bars, want 1", len(bars))
	}
	if len(bars[0].Candidates) != 2 {
		t.Fatalf("candidates = %v, want 2", bars[0].Candidates)
	}
	if bars[0].Candidates[0] != "Am" || bars[0].Candidates[1] != "G" {
		t.Errorf("candidates = %v, want [Am G]", bars[0].Candidates)
	}
}

func steps(start, step float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = start + step*float64(i)
	}
	return out
}
