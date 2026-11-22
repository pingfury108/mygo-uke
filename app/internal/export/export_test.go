package export

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
)

func sampleProject() *project.Project {
	p := project.New("导出测试")
	p.Duration = 6
	p.Downbeats = []float64{0, 3}
	p.Chords = []score.ChordSegment{
		{Start: 0, End: 3, Label: "C", Conf: 0.9},
		{Start: 3, End: 6, Label: "G", Conf: 0.9},
	}
	p.Lines = []project.Line{
		{Start: 0, End: 3, Text: "第一句"},
		{Start: 3, End: 6, Text: "第二句"},
	}
	p.RebuildBars()
	return p
}

func TestExportIntegration(t *testing.T) {
	source := os.Getenv("MYGO_UKE_EXPORT_SOURCE")
	if source == "" {
		t.Skip("set MYGO_UKE_EXPORT_SOURCE to run a real ffmpeg export")
	}
	output := os.Getenv("MYGO_UKE_EXPORT_OUT")
	if output == "" {
		output = filepath.Join(t.TempDir(), "out.mp4")
	}
	orientation := 0
	expected := "1080,1920"
	if os.Getenv("MYGO_UKE_EXPORT_ORIENTATION") == "1" {
		orientation = 1
		expected = "1920,1080"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := Run(ctx, sampleProject(), source, output, orientation, nil); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output is empty")
	}

	probe := exec.Command(
		"ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0",
		output,
	)
	raw, err := probe.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != expected {
		t.Fatalf("video size = %q", got)
	}
}

func TestWriteSRT(t *testing.T) {
	p := sampleProject()
	path := filepath.Join(t.TempDir(), "out.srt")
	if err := WriteSRT(p, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "第一句") || !strings.Contains(string(data), "第二句") {
		t.Fatalf("srt = %q", data)
	}
	if got := SRTName("/tmp/song.mp4"); got != "/tmp/song.srt" {
		t.Fatalf("SRTName = %q", got)
	}
}
