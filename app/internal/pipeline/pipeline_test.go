package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mygo-uke/internal/chordd"
	"mygo-uke/internal/project"
)

type fakeMedia struct{}

func (fakeMedia) Duration(context.Context, string) (float64, error) { return 100, nil }

func (fakeMedia) ExtractAudio(_ context.Context, _, destination string) error {
	return os.WriteFile(destination, []byte("audio"), 0o600)
}

func (fakeMedia) ExtractPCM(_ context.Context, _, destination string) error {
	return os.WriteFile(destination, []byte("pcm"), 0o600)
}

type fakeAnalyzer struct{}

func (fakeAnalyzer) Analyze(
	_ context.Context, _ string, _ float64, progress func(float64),
) (chordd.Result, error) {
	progress(0.5)
	return chordd.Result{
		Tempo:         68,
		TimeSignature: [2]int{4, 4},
		Key:           "C",
		Beats:         []float64{0, 0.9, 1.8, 2.7},
		Downbeats:     []float64{0, 3.6, 7.2},
		Chords: []chordd.Chord{
			{Start: 0, End: 3.6, Label: "C", Conf: 0.9},
			{Start: 3.6, End: 7.2, Label: "G", Conf: 0.8},
		},
	}, nil
}

type fakeTranscriber struct {
	err error
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

func TestImportBuildsAndSavesProject(t *testing.T) {
	source := filepath.Join(t.TempDir(), "夏夜来信.mov")
	if err := os.WriteFile(source, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	var stages []string

	built, path, err := Import(
		context.Background(), source, fakeAnalyzer{}, fakeMedia{},
		fakeTranscriber{}, root,
		func(progress Progress) { stages = append(stages, progress.Stage) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if built.Title != "夏夜来信" || built.Duration != 100 || built.Key != "C" {
		t.Fatalf("project = %+v", built)
	}
	if len(built.Chords) != 2 || len(built.Bars) == 0 {
		t.Fatalf("chords = %d bars = %d", len(built.Chords), len(built.Bars))
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
	if loaded.Title != "夏夜来信" || loaded.Chords[1].Label != "G" {
		t.Fatalf("loaded = %+v", loaded)
	}
}

func TestImportKeepsChordsWhenASRFails(t *testing.T) {
	source := filepath.Join(t.TempDir(), "song.mov")
	if err := os.WriteFile(source, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	built, _, err := Import(
		context.Background(), source, fakeAnalyzer{}, fakeMedia{},
		fakeTranscriber{err: os.ErrDeadlineExceeded}, t.TempDir(), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Chords) == 0 || len(built.Lines) != 0 {
		t.Fatalf("chords=%d lines=%d", len(built.Chords), len(built.Lines))
	}
	if len(built.Warnings) != 1 {
		t.Fatalf("warnings = %v", built.Warnings)
	}
}
