package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	"mygo-uke/internal/asr"
	"mygo-uke/internal/project"
	"mygo-uke/internal/sheet"
)

// Run with ffmpeg:
//
//	MYGO_UKE_SOURCE=/path/song.mp3 go test ./internal/pipeline -run TestImportIntegration -v
func TestImportIntegration(t *testing.T) {
	source := os.Getenv("MYGO_UKE_SOURCE")
	if source == "" {
		t.Skip("set MYGO_UKE_SOURCE to run the real import")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	p, path, err := Import(
		ctx, source, FFmpeg{}, nil, nil, t.TempDir(), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if p.Duration <= 0 {
		t.Fatalf("project duration=%v", p.Duration)
	}
	if _, err := project.Load(path); err != nil {
		t.Fatal(err)
	}
	t.Logf("duration=%.1fs bars=%d", p.Duration, len(p.Bars))
}

// Run with a Volcengine key and speech audio:
//
//	MYGO_UKE_VOLCANO_API_KEY=... MYGO_UKE_ASR_SOURCE=/path/speech.aiff \
//	  go test ./internal/pipeline -run TestImportLiveASR -v -count=1
func TestImportLiveASR(t *testing.T) {
	key := os.Getenv("MYGO_UKE_VOLCANO_API_KEY")
	source := os.Getenv("MYGO_UKE_ASR_SOURCE")
	if key == "" || source == "" {
		t.Skip("set MYGO_UKE_VOLCANO_API_KEY and MYGO_UKE_ASR_SOURCE to run live ASR import")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	transcriber := ASRTranscriber{Config: asr.Config{
		APIKey:     key,
		WSURL:      os.Getenv("MYGO_UKE_VOLCANO_WS_URL"),
		ResourceID: os.Getenv("MYGO_UKE_VOLCANO_RESOURCE_ID"),
		Model:      os.Getenv("MYGO_UKE_VOLCANO_MODEL"),
	}}
	p, path, err := Import(
		ctx, source, FFmpeg{}, transcriber, nil, t.TempDir(), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Lines) == 0 {
		t.Fatalf("lines = %d: %+v", len(p.Lines), p.Lines)
	}
	if len(p.Warnings) != 0 {
		t.Fatalf("warnings = %v", p.Warnings)
	}
	loaded, err := project.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Lines) != len(p.Lines) {
		t.Fatalf("saved lines = %d", len(loaded.Lines))
	}
	for _, line := range p.Lines {
		t.Logf("%.2f–%.2fs %s", line.Start, line.End, line.Text)
	}
}

// Run with a Volcengine key and a song:
//
//	MYGO_UKE_SOURCE=/path/song.mp4 MYGO_UKE_VOLCANO_API_KEY=... MYGO_UKE_CHART_TEXT=... \
//	  go test ./internal/pipeline -run TestImportFullIntegration -v -count=1
func TestImportFullIntegration(t *testing.T) {
	source := os.Getenv("MYGO_UKE_SOURCE")
	key := os.Getenv("MYGO_UKE_VOLCANO_API_KEY")
	if source == "" || key == "" {
		t.Skip("set MYGO_UKE_SOURCE and MYGO_UKE_VOLCANO_API_KEY to run the full import")
	}
	root := os.Getenv("MYGO_UKE_PROJECTS")
	if root == "" {
		root = t.TempDir()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var chart *sheet.Sheet
	if text := os.Getenv("MYGO_UKE_CHART_TEXT"); text != "" {
		var err error
		chart, err = sheet.ParseText(text)
		if err != nil {
			t.Fatal(err)
		}
	}
	transcriber := ASRTranscriber{Config: asr.Config{
		APIKey:     key,
		WSURL:      os.Getenv("MYGO_UKE_VOLCANO_WS_URL"),
		ResourceID: os.Getenv("MYGO_UKE_VOLCANO_RESOURCE_ID"),
		Model:      os.Getenv("MYGO_UKE_VOLCANO_MODEL"),
	}}
	p, path, err := Import(
		ctx,
		source,
		FFmpeg{},
		transcriber,
		chart,
		root,
		func(progress Progress) {
			t.Logf("%s %.0f%%", progress.Stage, progress.Value*100)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings) > 0 {
		t.Fatalf("warnings = %v", p.Warnings)
	}
	if len(p.Lines) == 0 {
		t.Fatal("no sentence lines")
	}
	if _, err := project.Load(path); err != nil {
		t.Fatal(err)
	}
	t.Logf(
		"project=%s duration=%.1fs tempo=%.0f key=%s chords=%d bars=%d lines=%d warnings=%v",
		path, p.Duration, p.Tempo, p.Key, len(p.Chords), len(p.Bars), len(p.Lines), p.Warnings,
	)
	for _, line := range p.Lines {
		t.Logf("line %.2f–%.2fs %s", line.Start, line.End, line.Text)
	}
}
