package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

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
		ctx, source, chordd.New(url), FFmpeg{}, t.TempDir(), nil,
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
