package export

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	renderplugin "github.com/egoist/mygo/plugins/render"

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
	if err := Run(ctx, sampleProject(), source, output, orientation, QualityStandard, nil); err != nil {
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

func TestLayout(t *testing.T) {
	width, height, videoWidth, videoHeight := layout(0)
	if width != 1080 || height != 1920 || videoWidth != 1080 || videoHeight != 1920 {
		t.Fatalf("vertical layout = %d×%d video %d×%d",
			width, height, videoWidth, videoHeight)
	}
	width, height, videoWidth, videoHeight = layout(1)
	if width != 1920 || height != 1080 || videoWidth != 606 || videoHeight != 1080 {
		t.Fatalf("horizontal layout = %d×%d video %d×%d",
			width, height, videoWidth, videoHeight)
	}
}

func TestRendererFor(t *testing.T) {
	t.Setenv("MYGO_UKE_RENDERER", "cpu")
	if got := rendererFor(); got != renderplugin.CPU {
		t.Fatalf("renderer = %v", got)
	}
	t.Setenv("MYGO_UKE_RENDERER", "gpu")
	if got := rendererFor(); got != renderplugin.GPU {
		t.Fatalf("renderer = %v", got)
	}
	t.Setenv("MYGO_UKE_RENDERER", "")
	t.Setenv("MYGO_UKE_ENCODER", "cpu")
	if got := rendererFor(); got != renderplugin.Auto {
		t.Fatalf("renderer = %v", got)
	}
	if got := encoderFor(); got != renderplugin.FFmpeg {
		t.Fatalf("encoder = %v", got)
	}
	t.Setenv("MYGO_UKE_ENCODER", "")
	if got := rendererFor(); got != renderplugin.Auto {
		t.Fatalf("renderer = %v", got)
	}
	if got := encoderFor(); got != renderplugin.System {
		t.Fatalf("encoder = %v", got)
	}
}

func TestMuxArgs(t *testing.T) {
	args := strings.Join(muxArgs("/tmp/video.mp4", "/tmp/source.mp4", "/tmp/out.mp4", 128_000), " ")
	for _, want := range []string{"-c:v copy", "-c:a aac", "-b:a 128k", "+faststart", "-shortest"} {
		if !strings.Contains(args, want) {
			t.Fatalf("mux args = %q, missing %q", args, want)
		}
	}
}

func TestQualityProfiles(t *testing.T) {
	if Profile(QualitySmall).Bitrate >= Profile(QualityStandard).Bitrate ||
		Profile(QualityStandard).Bitrate >= Profile(QualityOriginal).Bitrate {
		t.Fatalf("profiles are not ordered: %+v", qualityProfiles)
	}
	if Profile(QualityStandard).Bitrate < 8_000_000 || Profile(QualityOriginal).Bitrate != 12_441_600 {
		t.Fatalf("quality profiles are too low: %+v", qualityProfiles)
	}
	if Profile(Quality(99)).Name != "原画质" {
		t.Fatalf("invalid quality = %+v", Profile(99))
	}
	if ParseQuality("original") != QualityOriginal || ParseQuality("high") != QualityOriginal ||
		ParseQuality("small") != QualitySmall || ParseQuality("nope") != QualityOriginal {
		t.Fatalf("ParseQuality failed")
	}
}
