package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	"mygo-uke/internal/asr"
	"mygo-uke/internal/chordd"
	"mygo-uke/internal/project"
)

// Run with a real chordd and ffmpeg:
//
//	MYGO_UKE_SOURCE=/path/song.mp3 go test ./internal/pipeline -run TestImportIntegration -v
func TestImportIntegration(t *testing.T) {
	source := os.Getenv("MYGO_UKE_SOURCE")
	if source == "" {
		t.Skip("set MYGO_UKE_SOURCE to run the real import")
	}
	url := os.Getenv("MYGO_UKE_CHORDD")
	if url == "" {
		url = chorddBaseURL()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	p, path, err := Import(
		ctx, source, chordd.New(url), FFmpeg{}, nil, t.TempDir(), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if p.Duration <= 0 || len(p.Chords) == 0 || len(p.Bars) == 0 {
		t.Fatalf("project duration=%v chords=%d bars=%d", p.Duration, len(p.Chords), len(p.Bars))
	}
	if _, err := project.Load(path); err != nil {
		t.Fatal(err)
	}
	t.Logf(
		"duration=%.1fs tempo=%.0f key=%s chords=%d bars=%d downbeats=%d",
		p.Duration, p.Tempo, p.Key, len(p.Chords), len(p.Bars), len(p.Downbeats),
	)
}

func chorddBaseURL() string {
	if url := os.Getenv("MYGO_UKE_CHORDD"); url != "" {
		return url
	}
	return "http://MiWiFi-R1350-srv.local:8790"
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
		ctx, source, fakeAnalyzer{}, FFmpeg{}, transcriber, t.TempDir(), nil,
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
