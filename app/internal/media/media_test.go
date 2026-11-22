package media

import (
	"context"
	"os"
	"testing"
)

func TestParseDurationOutput(t *testing.T) {
	output := "  Duration: 00:01:12.34, start: 0.000000, bitrate: 128 kb/s"
	if got := parseFFmpegDuration(output); got != 72.34 {
		t.Fatalf("parseFFmpegDuration = %v", got)
	}
	if got := parseFFmpegDuration("no duration"); got != 0 {
		t.Fatalf("invalid = %v", got)
	}
}

func TestDurationIntegration(t *testing.T) {
	source := os.Getenv("MYGO_UKE_MEDIA_SOURCE")
	if source == "" {
		t.Skip("set MYGO_UKE_MEDIA_SOURCE to probe a real file")
	}
	duration, err := Duration(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if duration <= 0 {
		t.Fatalf("duration = %v", duration)
	}
	t.Logf("duration = %.3fs", duration)
}
