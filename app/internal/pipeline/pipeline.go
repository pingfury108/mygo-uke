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

// Import extracts audio, waits for chordd, and saves a project.
func Import(
	ctx context.Context,
	source string,
	analyzer Analyzer,
	media Media,
	transcriber Transcriber,
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
	pcm := filepath.Join(work, "asr.pcm")

	send(report, "抽取音轨", 0.06)
	if err := media.ExtractAudio(ctx, absolute, audio); err != nil {
		return nil, "", err
	}
	if transcriber != nil {
		if err := media.ExtractPCM(ctx, absolute, pcm); err != nil {
			return nil, "", err
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	progress := &progressTracker{report: report}
	type chordResult struct {
		value chordd.Result
		err   error
	}
	type lineResult struct {
		value []project.Line
		err   error
	}
	chords := make(chan chordResult, 1)
	go func() {
		result, err := analyzer.Analyze(runCtx, audio, duration, func(value float64) {
			progress.send("分析和弦", 0.16+0.72*clamp(value))
		})
		chords <- chordResult{value: result, err: err}
	}()
	lines := make(chan lineResult, 1)
	if transcriber == nil {
		lines <- lineResult{}
	} else {
		go func() {
			result, err := transcriber.Transcribe(
				runCtx, pcm, duration,
				func(value float64) {
					progress.send("转写字幕", 0.16+0.72*clamp(value))
				},
			)
			lines <- lineResult{value: result, err: err}
		}()
	}

	chord := <-chords
	if chord.err != nil {
		cancel()
		return nil, "", chord.err
	}
	line := <-lines
	var warnings []string
	if line.err != nil {
		warnings = append(warnings, "字幕识别失败："+line.err.Error())
	}

	title := sourceTitle(absolute, duration)
	built := fromResult(title, absolute, duration, chord.value, line.value)
	built.Warnings = warnings
	path := filepath.Join(project.NewDir(root, title), "project.json")
	send(report, "保存工程", 0.95)
	if err := project.Save(path, built); err != nil {
		return nil, "", err
	}
	send(report, "完成", 1)
	return built, path, nil
}

func fromResult(
	title, source string,
	duration float64,
	result chordd.Result,
	lines []project.Line,
) *project.Project {
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
			Start:      start,
			End:        end,
			Label:      chord.Label,
			Conf:       chord.Conf,
			Candidates: append([]string(nil), chord.Candidates...),
		})
	}
	p.RebuildBars()
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

func (FFmpeg) ExtractAudio(ctx context.Context, source, destination string) error {
	return media.ExtractAudio(ctx, source, destination)
}

func (FFmpeg) ExtractPCM(ctx context.Context, source, destination string) error {
	return media.ExtractPCM(ctx, source, destination)
}
