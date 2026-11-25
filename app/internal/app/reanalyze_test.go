package app

import (
	"path/filepath"
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

func TestChartForReanalyzeRestoresLegacyTextChart(t *testing.T) {
	a := newApp()
	a.proj = project.New("legacy")
	a.proj.Duration = 10
	a.proj.ChartName = "粘贴文本"
	a.proj.ChartText = "F G\n踮起脚尖 提起裙边\nC\n舞步翩翩 呼吸浅浅"
	a.proj.Lines = []project.Line{{
		Start: 5, End: 10, Text: "舞步翩翩 呼吸浅浅",
		Chords: []project.ChordMark{{At: 0, Label: "C"}},
	}}

	chart, restore := a.chartForReanalyze()
	if chart == nil || len(chart.Lines) != 2 {
		t.Fatalf("chart lines = %+v, want 2 restored lines", chart)
	}
	if chart.Lines[0].Lyrics != "踮起脚尖 提起裙边" {
		t.Fatalf("first line = %q", chart.Lines[0].Lyrics)
	}
	if chartOriginReady(restore) {
		t.Fatalf("text chart should not need OCR, got %+v", restore)
	}
}

func TestChartForReanalyzeRestoresLegacyImageChart(t *testing.T) {
	a := newApp()
	a.proj = project.New("legacy")
	a.proj.Duration = 10
	a.proj.ChartName = "chart.png"
	a.proj.ChartImage = "chart.png"
	a.projPath = filepath.Join(t.TempDir(), "project.json")
	a.proj.Lines = []project.Line{{
		Start: 5, End: 10, Text: "舞步翩翩",
		Chords: []project.ChordMark{{At: 0, Label: "C"}},
	}}

	_, restore := a.chartForReanalyze()
	if !chartOriginReady(restore) {
		t.Fatal("image chart should be restored from the stored copy")
	}
	if restore.Path != filepath.Join(filepath.Dir(a.projPath), "chart.png") {
		t.Fatalf("restore path = %q", restore.Path)
	}
}

func TestChartForReanalyzeKeepsSnapshotChart(t *testing.T) {
	a := newApp()
	a.proj = project.New("current")
	a.proj.Duration = 10
	a.proj.ChartImage = "chart.png"
	a.proj.ChartText = "F\n不会被重新解析"
	a.proj.ChartLines = []project.ChartLine{{
		Lyrics: "好想能看到",
		Chords: []project.ChordMark{{At: 0, Label: "F"}},
	}}
	a.proj.ChartSnapshot = project.ChartSnapshotVersion

	chart, restore := a.chartForReanalyze()
	if chart == nil || len(chart.Lines) != 1 || chart.Lines[0].Lyrics != "好想能看到" {
		t.Fatalf("chart = %+v", chart)
	}
	if chartOriginReady(restore) {
		t.Fatalf("snapshot chart should not be restored, got %+v", restore)
	}
}

func TestChartForReanalyzeRebuildsIncompleteSnapshot(t *testing.T) {
	a := newApp()
	a.proj = project.New("legacy")
	a.proj.Duration = 10
	a.proj.ChartImage = "chart.png"
	a.proj.ChartLines = []project.ChartLine{{
		Lyrics: "舞步翩翩",
		Chords: []project.ChordMark{{At: 0, Label: "C"}},
	}}
	a.projPath = filepath.Join(t.TempDir(), "project.json")

	_, restore := a.chartForReanalyze()
	if !chartOriginReady(restore) {
		t.Fatal("an old snapshot must be rebuilt from the stored chart")
	}
}
