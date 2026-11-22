// Package export renders project overlays onto video frames and muxes audio.
package export

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	renderplugin "github.com/egoist/mygo/plugins/render"
	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/media"
	"mygo-uke/internal/project"
	"mygo-uke/internal/render"
)

const fps = 30

// Run exports p from source into output. orientation 0 is vertical.
func Run(
	ctx context.Context,
	p *project.Project,
	source, output string,
	orientation int,
	progress func(float64),
) error {
	if p == nil {
		return errors.New("export: project is nil")
	}
	if source == "" {
		return errors.New("export: source video is empty")
	}
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("export source: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		return err
	}
	width, height, videoWidth, videoHeight := layout(orientation)
	frames, err := frameCount(ctx, source, p.Duration)
	if err != nil {
		return err
	}

	renderCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	renderer := rendererFor()
	format := feedI420
	if renderer == renderplugin.CPU {
		format = feedRGBA
	}
	feed := newFrameFeed(renderCtx, cancel, source, videoWidth, videoHeight, format)
	defer feed.Close()

	debug := os.Getenv("MYGO_UKE_EXPORT_DEBUG") != ""
	started := time.Now()
	var rendererName string
	comp := renderplugin.Composition{
		Width: width, Height: height, FPS: fps, Frames: frames,
		Background: ui.Hex("#0F1013"),
		View: func(c *ui.Context, f renderplugin.Frame) {
			spec := render.Spec{
				Width:       width,
				Height:      height,
				VideoWidth:  videoWidth,
				VideoHeight: videoHeight,
				Background:  feed.At(f.Index),
				Project:     p,
				Time:        f.Seconds(),
				Orientation: orientation,
			}
			render.Backdrop(c, spec)
			render.Overlay(c, spec)
		},
	}
	tmpVideo := output + ".render.mp4"
	_ = os.Remove(tmpVideo)
	defer os.Remove(tmpVideo)
	renderVideo := func(renderer renderplugin.Renderer, encoder renderplugin.Encoder) error {
		options := renderplugin.Options{
			Renderer: renderer,
			Encoder:  encoder,
			FFmpeg:   media.FFmpegPath(),
			Progress: func(pr renderplugin.Progress) {
				if debug && pr.Renderer != rendererName {
					rendererName = pr.Renderer
					fmt.Fprintf(os.Stderr, "export: renderer=%s\n", rendererName)
				}
				if progress != nil && pr.Frames > 0 {
					progress(min(0.9, float64(pr.Done)/float64(pr.Frames)*0.9))
				}
			},
		}
		return renderplugin.Video(renderCtx, comp, tmpVideo, options)
	}

	encoder := encoderFor()
	err = renderVideo(renderer, encoder)
	if err != nil && encoder == renderplugin.System && !forcedGPU() &&
		!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) &&
		feed.Err() == nil {
		if debug {
			fmt.Fprintf(os.Stderr, "export: system encoder failed, retrying on CPU: %v\n", err)
		}
		feed.Reset(feedRGBA)
		if progress != nil {
			progress(0)
		}
		_ = os.Remove(tmpVideo)
		err = renderVideo(renderplugin.CPU, renderplugin.FFmpeg)
	}
	if err != nil {
		if feedErr := feed.Err(); feedErr != nil {
			return feedErr
		}
		return fmt.Errorf("export render: %w", err)
	}
	if debug {
		fmt.Fprintf(os.Stderr, "export: render=%s frames=%d\n",
			time.Since(started).Round(time.Millisecond), frames)
	}
	if progress != nil {
		progress(0.92)
	}
	muxStarted := time.Now()
	if err := mux(ctx, tmpVideo, source, output); err != nil {
		return err
	}
	if debug {
		fmt.Fprintf(os.Stderr, "export: mux=%s total=%s output=%s\n",
			time.Since(muxStarted).Round(time.Millisecond),
			time.Since(started).Round(time.Millisecond), output)
	}
	if progress != nil {
		progress(1)
	}
	return nil
}

func layout(orientation int) (width, height, videoWidth, videoHeight int) {
	if orientation == 1 {
		return 1920, 1080, render.HorizontalVideoWidth(1080), 1080
	}
	return 1080, 1920, 1080, 1920
}

func frameCount(ctx context.Context, source string, fallback float64) (int, error) {
	seconds := fallback
	if value, err := media.Duration(ctx, source); err == nil && value > 0 {
		seconds = value
	}
	frames := int(math.Round(seconds * fps))
	if frames <= 0 {
		return 0, errors.New("export: source video has no frames")
	}
	return frames, nil
}

func rendererFor() renderplugin.Renderer {
	switch strings.ToLower(os.Getenv("MYGO_UKE_RENDERER")) {
	case "cpu":
		return renderplugin.CPU
	case "gpu":
		return renderplugin.GPU
	}
	return renderplugin.Auto
}

func encoderFor() renderplugin.Encoder {
	switch strings.ToLower(os.Getenv("MYGO_UKE_ENCODER")) {
	case "cpu", "libx264":
		return renderplugin.FFmpeg
	}
	return renderplugin.System
}

func forcedGPU() bool {
	return strings.EqualFold(os.Getenv("MYGO_UKE_RENDERER"), "gpu")
}

func mux(ctx context.Context, video, source, output string) error {
	tmp := output + ".mux.mp4"
	_ = os.Remove(tmp)
	defer os.Remove(tmp)
	cmd := exec.CommandContext(ctx, media.FFmpegPath(), muxArgs(video, source, tmp)...)
	var log bytes.Buffer
	cmd.Stderr = &log
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("export mux: %w: %s", err, strings.TrimSpace(log.String()))
	}
	return os.Rename(tmp, output)
}

func muxArgs(video, source, output string) []string {
	return []string{
		"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-i", video,
		"-i", source,
		"-map", "0:v:0",
		"-map", "1:a:0?",
		"-c:v", "copy",
		"-c:a", "aac",
		"-b:a", "192k",
		"-movflags", "+faststart",
		"-shortest",
		output,
	}
}
