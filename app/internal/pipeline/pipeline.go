// Package pipeline turns one source video into a saved project.json.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mygo-uke/internal/chordd"
	"mygo-uke/internal/media"
	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
)

// Analyzer is the chordd client method Import needs.
type Analyzer interface {
	Analyze(
		ctx context.Context,
		audioPath string,
		duration float64,
		progress func(float64),
	) (chordd.Result, error)
}

// Media is the ffmpeg surface Import needs, so tests can avoid processes.
type Media interface {
	Duration(ctx context.Context, path string) (float64, error)
	ExtractAudio(ctx context.Context, source, destination string) error
}

// Progress is one import update for the UI.
type Progress struct {
	Stage string
	Value float64
}

// Import extracts audio, waits for chordd, and saves a project.
func Import(
	ctx context.Context,
	source string,
	analyzer Analyzer,
	media Media,
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
	audio := filepath.Join(work, "audio.mp3")

	send(report, "抽取音轨", 0.06)
	if err := media.ExtractAudio(ctx, absolute, audio); err != nil {
		return nil, "", err
	}
	send(report, "分析和弦", 0.16)

	result, err := analyzer.Analyze(ctx, audio, duration, func(value float64) {
		send(report, "分析和弦", 0.16+0.72*clamp(value))
	})
	if err != nil {
		return nil, "", err
	}

	title := strings.TrimSuffix(filepath.Base(absolute), filepath.Ext(absolute))
	built := fromResult(title, absolute, duration, result)
	path := filepath.Join(project.NewDir(root, title), "project.json")
	send(report, "保存工程", 0.95)
	if err := project.Save(path, built); err != nil {
		return nil, "", err
	}
	send(report, "完成", 1)
	return built, path, nil
}

func fromResult(title, source string, duration float64, result chordd.Result) *project.Project {
	p := project.New(title)
	p.Source = source
	p.Duration = duration
	p.Tempo = result.Tempo
	p.Key = result.Key
	if result.TimeSignature[0] > 0 && result.TimeSignature[1] > 0 {
		p.TimeSignature = result.TimeSignature
	}
	p.Beats = append([]float64(nil), result.Beats...)
	p.Downbeats = append([]float64(nil), result.Downbeats...)
	for _, chord := range result.Chords {
		if chord.Label == "" || chord.End <= chord.Start {
			continue
		}
		start := clampSeconds(chord.Start, duration)
		end := clampSeconds(chord.End, duration)
		if end <= start {
			continue
		}
		p.Chords = append(p.Chords, score.ChordSegment{
			Start: start,
			End:   end,
			Label: chord.Label,
			Conf:  chord.Conf,
		})
	}
	p.RebuildBars()
	return p
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

func (FFmpeg) ExtractAudio(ctx context.Context, source, destination string) error {
	return media.ExtractAudio(ctx, source, destination)
}
