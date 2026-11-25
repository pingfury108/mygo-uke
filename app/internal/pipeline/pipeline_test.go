package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
	"mygo-uke/internal/sheet"
)

type fakeMedia struct{}

func (fakeMedia) Duration(context.Context, string) (float64, error) { return 100, nil }

func (fakeMedia) ExtractPCM(_ context.Context, _, destination string) error {
	return os.WriteFile(destination, []byte("pcm"), 0o600)
}

type fakeTranscriber struct {
	err error
}

type timedTranscriber struct {
	lines []project.Line
}

func (t timedTranscriber) Transcribe(
	_ context.Context, _ string, _ float64, progress func(float64),
) ([]project.Line, error) {
	progress(1)
	return t.lines, nil
}

func (f fakeTranscriber) Transcribe(
	_ context.Context, _ string, _ float64, progress func(float64),
) ([]project.Line, error) {
	progress(0.8)
	if f.err != nil {
		return nil, f.err
	}
	return []project.Line{
		{Start: 0, End: 3.6, Text: "第一句"},
		{Start: 3.6, End: 7.2, Text: "第二句"},
	}, nil
}

func TestImportBuildsSubtitlesWithoutAudioChordAnalysis(t *testing.T) {
	source := filepath.Join(t.TempDir(), "夏夜来信.mov")
	if err := os.WriteFile(source, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	var stages []string

	built, path, err := Import(
		context.Background(), source, fakeMedia{}, fakeTranscriber{}, nil, root,
		func(progress Progress) { stages = append(stages, progress.Stage) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if built.Title != "夏夜来信" || built.Duration != 100 {
		t.Fatalf("project = %+v", built)
	}
	if len(built.Chords) != 0 {
		t.Fatalf("audio analysis leaked chords: %+v", built.Chords)
	}
	if len(built.Lines) != 2 || built.Lines[1].Text != "第二句" {
		t.Fatalf("lines = %+v", built.Lines)
	}
	if stages[len(stages)-1] != "完成" {
		t.Fatalf("stages = %v", stages)
	}

	loaded, err := project.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Title != "夏夜来信" || len(loaded.Chords) != 0 {
		t.Fatalf("loaded = %+v", loaded)
	}
}

func TestImportAppliesChartAndBuildsBarsWithoutASR(t *testing.T) {
	source := filepath.Join(t.TempDir(), "song.mov")
	if err := os.WriteFile(source, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	chart := &sheet.Sheet{
		Key:           "C",
		BPM:           120,
		TimeSignature: [2]int{3, 4},
		Lines: []sheet.Line{
			{Lyrics: "第一句", Chords: []project.ChordMark{{At: 0, Label: "C"}}},
			{Lyrics: "第二句", Chords: []project.ChordMark{{At: 0, Label: "Am"}, {At: 2, Label: "F"}}},
		},
	}

	built, _, err := Import(
		context.Background(), source, fakeMedia{}, nil, chart, t.TempDir(), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if built.Key != "C" || built.Tempo != 120 || built.TimeSignature != [2]int{3, 4} {
		t.Fatalf("chart metadata = key %q tempo %v meter %v", built.Key, built.Tempo, built.TimeSignature)
	}
	if len(built.Lines) != 2 {
		t.Fatalf("lines = %+v", built.Lines)
	}
	if len(built.Chords) != 3 || built.Chords[0].Source != "chart" {
		t.Fatalf("chords = %+v", built.Chords)
	}
	if len(built.Beats) == 0 || len(built.Downbeats) == 0 || len(built.Bars) == 0 {
		t.Fatalf("grid = beats %d downbeats %d bars %d",
			len(built.Beats), len(built.Downbeats), len(built.Bars))
	}
}

func TestReanalyzeRetimesChartLyrics(t *testing.T) {
	source := filepath.Join(t.TempDir(), "song.mov")
	if err := os.WriteFile(source, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	edited := project.New("song")
	edited.Duration = 100
	edited.Lines = []project.Line{
		{Start: 10, End: 14, Text: "第一句", Chords: []project.ChordMark{{At: 0, Label: "C"}}},
		{Start: 20, End: 24, Text: "第二句", Chords: []project.ChordMark{{At: 0, Label: "Am"}}},
	}
	edited.LinesEdited = true
	edited.Chords = []score.ChordSegment{
		{Start: 10, End: 20, Label: "C", Source: "chart"},
		{Start: 20, End: 24, Label: "Am", Source: "chart"},
	}

	chart := sheet.FromProject(edited)
	built, _, err := Import(
		context.Background(), source, fakeMedia{}, fakeTranscriber{}, chart, t.TempDir(), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	merged := project.MergeReanalyze(edited, built)
	if len(merged.Lines) != 2 || merged.Lines[0].Start != 0 || merged.Lines[1].Start != 3.6 {
		t.Fatalf("lines kept stale timing: %+v", merged.Lines)
	}
	if len(merged.Chords) != 2 || merged.Chords[0].Label != "C" || merged.Chords[1].Label != "Am" {
		t.Fatalf("chart chords = %+v", merged.Chords)
	}
	if merged.Chords[0].Start != merged.Lines[0].Start {
		t.Fatalf("chord timing = %+v lines = %+v", merged.Chords, merged.Lines)
	}
}

func TestReanalyzeKeepsChordOnlySections(t *testing.T) {
	source := filepath.Join(t.TempDir(), "song.mov")
	if err := os.WriteFile(source, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	edited := project.New("song")
	edited.Duration = 100
	edited.Lines = []project.Line{
		{Start: 10, End: 14, Text: "第一句", Chords: []project.ChordMark{{At: 0, Label: "C"}}},
		{Start: 20, End: 24, Text: "第二句", Chords: []project.ChordMark{{At: 0, Label: "Am"}}},
	}
	edited.LinesEdited = true
	edited.Chords = []score.ChordSegment{
		{Start: 0, End: 5, Label: "F", Source: "chart"},
		{Start: 5, End: 10, Label: "G", Source: "chart"},
		{Start: 10, End: 20, Label: "C", Source: "chart"},
		{Start: 20, End: 24, Label: "Am", Source: "chart"},
		{Start: 24, End: 30, Label: "C", Source: "chart"},
	}
	transcriber := timedTranscriber{lines: []project.Line{
		{Start: 10, End: 14, Text: "第一句"},
		{Start: 20, End: 24, Text: "第二句"},
	}}

	built, _, err := Import(
		context.Background(), source, fakeMedia{}, transcriber, sheet.FromProject(edited), t.TempDir(), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	merged := project.MergeReanalyze(edited, built)
	if len(merged.Lines) != 2 {
		t.Fatalf("chord-only sections leaked into subtitles: %+v", merged.Lines)
	}
	labels := make([]string, 0, len(merged.Chords))
	for _, segment := range merged.Chords {
		labels = append(labels, segment.Label)
	}
	if len(labels) != 5 || labels[0] != "F" || labels[1] != "G" || labels[4] != "C" {
		t.Fatalf("chords = %v", labels)
	}
}

func TestImportKeepsChartWhenASRFails(t *testing.T) {
	source := filepath.Join(t.TempDir(), "song.mov")
	if err := os.WriteFile(source, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	chart := &sheet.Sheet{Lines: []sheet.Line{{
		Lyrics: "第一句",
		Chords: []project.ChordMark{{At: 0, Label: "C"}},
	}}}
	built, _, err := Import(
		context.Background(), source, fakeMedia{}, fakeTranscriber{err: os.ErrDeadlineExceeded},
		chart, t.TempDir(), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Lines) != 1 || len(built.Chords) == 0 {
		t.Fatalf("lines=%d chords=%d", len(built.Lines), len(built.Chords))
	}
	if len(built.Warnings) != 1 {
		t.Fatalf("warnings = %v", built.Warnings)
	}
}

func TestSourceTitle(t *testing.T) {
	if got := sourceTitle("/tmp/mmexport1795328663661.mp4", 36.2); got != "弹唱 00:36" {
		t.Fatalf("wechat title = %q", got)
	}
	if got := sourceTitle("/tmp/晴天.mov", 200); got != "晴天" {
		t.Fatalf("normal title = %q", got)
	}
}
