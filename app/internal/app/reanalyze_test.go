package app

import (
	"testing"

	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
)

func TestMergeAfterReanalyzeKeepsOldTimingWhenASRFails(t *testing.T) {
	edited := project.New("test")
	edited.Duration = 8
	edited.Lines = []project.Line{{
		Start: 0, End: 4, Text: "好想能看到",
		Chords: []project.ChordMark{{At: 0, Label: "F"}},
	}}
	edited.LinesEdited = true
	edited.Chords = []score.ChordSegment{{Start: 0, End: 4, Label: "F", Source: "chart"}}

	built := project.New("test")
	built.Duration = 8
	built.Lines = []project.Line{{
		Start: 4, End: 8, Text: "好想能看到",
		Chords: []project.ChordMark{{At: 0, Label: "F"}},
	}}
	built.Chords = []score.ChordSegment{{Start: 4, End: 8, Label: "F", Source: "chart"}}
	built.Warnings = []string{"字幕识别失败：timeout"}

	merged := mergeAfterReanalyze(edited, built)
	if merged.Lines[0].Start != 0 {
		t.Fatalf("failed ASR replaced timing: %+v", merged.Lines)
	}
}
