package asr

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mygo-uke/internal/media"
)

// Run with a Volcengine key and any audio/video with speech:
//
//	MYGO_UKE_VOLCANO_API_KEY=... MYGO_UKE_ASR_SOURCE=/path/speech.aiff \
//	  go test ./internal/asr -run TestTranscribeLive -v -count=1
func TestTranscribeLive(t *testing.T) {
	key := os.Getenv("MYGO_UKE_VOLCANO_API_KEY")
	source := os.Getenv("MYGO_UKE_ASR_SOURCE")
	if key == "" || source == "" {
		t.Skip("set MYGO_UKE_VOLCANO_API_KEY and MYGO_UKE_ASR_SOURCE to run live ASR")
	}
	cfg := Config{
		APIKey:     key,
		WSURL:      os.Getenv("MYGO_UKE_VOLCANO_WS_URL"),
		ResourceID: os.Getenv("MYGO_UKE_VOLCANO_RESOURCE_ID"),
		Model:      os.Getenv("MYGO_UKE_VOLCANO_MODEL"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := TestConnection(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	duration, err := media.Duration(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	pcm := filepath.Join(t.TempDir(), "asr.pcm")
	if err := media.ExtractPCM(ctx, source, pcm); err != nil {
		t.Fatal(err)
	}
	lines, err := TranscribeFile(ctx, cfg, pcm, duration, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) == 0 {
		t.Fatal("no sentence lines")
	}
	for _, line := range lines {
		t.Logf("%.2f–%.2fs %s", line.Start, line.End, line.Text)
	}
}
