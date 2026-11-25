package sheet

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestParseImageLive(t *testing.T) {
	path := os.Getenv("MYGO_UKE_CHART_IMAGE")
	if path == "" {
		t.Skip("set MYGO_UKE_CHART_IMAGE")
	}
	cfg := Config{
		BaseURL: os.Getenv("MYGO_UKE_SHEET_BASE_URL"),
		APIKey:  os.Getenv("MYGO_UKE_SHEET_API_KEY"),
		Model:   os.Getenv("MYGO_UKE_SHEET_MODEL"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := client.ParseImage(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("lines=%d key=%q", len(parsed.Lines), parsed.Key)
	for i, line := range parsed.Lines {
		t.Logf("%d: %q", i, line.Lyrics)
	}
}
