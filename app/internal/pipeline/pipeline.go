// Package pipeline turns one source video into a saved project.json.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"mygo-uke/internal/asr"
	"mygo-uke/internal/media"
	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
	"mygo-uke/internal/sheet"
)

// Media is the ffmpeg surface Import needs, so tests can avoid processes.
type Media interface {
	Duration(ctx context.Context, path string) (float64, error)
	ExtractPCM(ctx context.Context, source, destination string) error
}

// Transcriber turns the 16kHz PCM sidecar into sentence-level lines.
type Transcriber interface {
	Transcribe(
		ctx context.Context,
		pcmPath string,
		duration float64,
		progress func(float64),
	) ([]project.Line, error)
}

// ASRTranscriber adapts the Volcengine streaming client to the pipeline.
type ASRTranscriber struct {
	Config asr.Config
}

func (t ASRTranscriber) Transcribe(
	ctx context.Context,
	pcmPath string,
	duration float64,
	progress func(float64),
) ([]project.Line, error) {
	lines, err := asr.TranscribeFile(ctx, t.Config, pcmPath, duration, progress)
	if err != nil {
		return nil, err
	}
	out := make([]project.Line, 0, len(lines))
	for _, line := range lines {
		out = append(out, project.Line{
			Start: line.Start,
			End:   line.End,
			Text:  line.Text,
		})
	}
	return out, nil
}

// Progress is one import update for the UI.
type Progress struct {
	Stage string
	Value float64
}

// Import extracts the ASR track, optionally aligns a chart, and saves a
// project. Audio chord and beat analysis is intentionally not part of this
// path: the printed chart is the source of truth for chords.
func Import(
	ctx context.Context,
	source string,
	media Media,
	transcriber Transcriber,
	chart *sheet.Sheet,
	root string,
	report func(Progress),
) (*project.Project, string, error) {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return nil, "", err
	}
	if _, err := os.Stat(absolute); err != nil {
		return nil, "", fmt.Errorf("source video: %w", err)
	}

	send(report, "读取视频", 0.02)
	duration, err := media.Duration(ctx, absolute)
	if err != nil {
		return nil, "", err
	}

	work, err := os.MkdirTemp("", "mygo-uke-import-*")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(work)
	pcm := filepath.Join(work, "asr.pcm")

	if transcriber != nil {
		send(report, "抽取音轨", 0.08)
		if err := media.ExtractPCM(ctx, absolute, pcm); err != nil {
			return nil, "", err
		}
	}

	progress := &progressTracker{report: report}
	var warnings []string
	var lines []project.Line
	if transcriber != nil {
		var err error
		lines, err = transcriber.Transcribe(
			ctx, pcm, duration,
			func(value float64) {
				progress.send("转写字幕", 0.12+0.7*clamp(value))
			},
		)
		if err != nil {
			warnings = append(warnings, "字幕识别失败："+err.Error())
		}
	}

	title := sourceTitle(absolute, duration)
	built := fromLines(title, absolute, duration, lines)
	if chart != nil {
		send(report, "对齐曲谱", 0.84)
		sheet.Apply(built, chart)
	}
	built.Warnings = warnings
	built.RebuildBars()
	path := filepath.Join(project.NewDir(root, title), "project.json")
	send(report, "保存工程", 0.95)
	if err := project.Save(path, built); err != nil {
		return nil, "", err
	}
	send(report, "完成", 1)
	return built, path, nil
}

func fromLines(
	title, source string,
	duration float64,
	lines []project.Line,
) *project.Project {
	p := project.New(title)
	p.Source = source
	p.Duration = duration
	for _, line := range lines {
		text := strings.TrimSpace(line.Text)
		start := clampSeconds(line.Start, duration)
		end := clampSeconds(line.End, duration)
		if text == "" || end <= start {
			continue
		}
		p.Lines = append(p.Lines, project.Line{
			Start: start,
			End:   end,
			Text:  text,
			Conf:  line.Conf,
		})
	}
	sort.SliceStable(p.Lines, func(i, j int) bool {
		return p.Lines[i].Start < p.Lines[j].Start
	})
	return p
}

type progressTracker struct {
	mu     sync.Mutex
	last   float64
	report func(Progress)
}

func (p *progressTracker) send(stage string, value float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if value < p.last {
		value = p.last
	}
	p.last = value
	send(p.report, stage, value)
}

func clampSeconds(value, duration float64) float64 {
	if value < 0 {
		return 0
	}
	if duration > 0 && value > duration {
		return duration
	}
	return value
}

func sourceTitle(path string, duration float64) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	lower := strings.ToLower(base)
	for _, prefix := range []string{"mmexport", "wx_camera_", "rpreplay_final", "img_"} {
		if strings.HasPrefix(lower, prefix) {
			return "弹唱 " + score.TimeText(duration)
		}
	}
	return base
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func send(report func(Progress), stage string, value float64) {
	if report != nil {
		report(Progress{Stage: stage, Value: clamp(value)})
	}
}

// FFmpeg is the production Media implementation.
type FFmpeg struct{}

func (FFmpeg) Duration(ctx context.Context, path string) (float64, error) {
	return media.Duration(ctx, path)
}

func (FFmpeg) ExtractPCM(ctx context.Context, source, destination string) error {
	return media.ExtractPCM(ctx, source, destination)
}
