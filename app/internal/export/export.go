// Package export renders project overlays onto video frames and muxes audio.
package export

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/project"
	"mygo-uke/internal/render"
)

const (
	fps = 30
	crf = 18
)

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
	width, height := 1080, 1920
	videoWidth, videoHeight := width, height
	if orientation == 1 {
		width, height = 1920, 1080
		videoWidth, videoHeight = height*607/1080, height
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		return err
	}

	filter := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,fps=%d",
		videoWidth, videoHeight, videoWidth, videoHeight, fps,
	)
	decoder := exec.CommandContext(
		ctx,
		ffmpegPath(),
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", source,
		"-vf", filter,
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-",
	)
	decoderOut, err := decoder.StdoutPipe()
	if err != nil {
		return err
	}
	var decoderLog bytes.Buffer
	decoder.Stderr = &decoderLog
	if err := decoder.Start(); err != nil {
		return err
	}
	decoderWaited := false
	defer func() {
		if !decoderWaited && decoder.Process != nil {
			_ = decoder.Process.Kill()
			_ = decoder.Wait()
		}
	}()

	encoder := exec.CommandContext(
		ctx,
		ffmpegPath(),
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-y",
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", width, height),
		"-r", fmt.Sprint(fps),
		"-i", "-",
		"-i", source,
		"-map", "0:v:0",
		"-map", "1:a:0?",
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", fmt.Sprint(crf),
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "192k",
		"-movflags", "+faststart",
		"-shortest",
		output,
	)
	encoderIn, err := encoder.StdinPipe()
	if err != nil {
		return err
	}
	var encoderLog bytes.Buffer
	encoder.Stderr = &encoderLog
	if err := encoder.Start(); err != nil {
		return err
	}

	frameSize := videoWidth * videoHeight * 4
	buffer := make([]byte, frameSize)
	bitmap := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, videoWidth, videoHeight)))
	frame := 0
	var writeErr error
	for {
		if _, err := io.ReadFull(decoderOut, buffer); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				writeErr = fmt.Errorf("export decoder: %w", err)
			}
			break
		}
		pixels := &image.RGBA{
			Pix:    buffer,
			Stride: videoWidth * 4,
			Rect:   image.Rect(0, 0, videoWidth, videoHeight),
		}
		bitmap.Update(pixels)
		seconds := float64(frame) / fps
		out := render.Frame(render.Spec{
			Width:       width,
			Height:      height,
			VideoWidth:  videoWidth,
			VideoHeight: videoHeight,
			Background:  bitmap,
			Project:     p,
			Time:        seconds,
			Orientation: orientation,
		})
		if err := writeFrame(encoderIn, out, width); err != nil {
			writeErr = fmt.Errorf("export encoder: %w", err)
			break
		}
		frame++
		if progress != nil && p.Duration > 0 {
			progress(min(0.99, seconds/p.Duration))
		}
	}

	_ = encoderIn.Close()
	encoderErr := encoder.Wait()
	if writeErr != nil && decoder.Process != nil {
		_ = decoder.Process.Kill()
	}
	decoderErr := decoder.Wait()
	decoderWaited = true
	if writeErr != nil {
		return writeErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if decoderErr != nil {
		return fmt.Errorf("export decoder: %w: %s", decoderErr, strings.TrimSpace(decoderLog.String()))
	}
	if encoderErr != nil {
		return fmt.Errorf("export encoder: %w: %s", encoderErr, strings.TrimSpace(encoderLog.String()))
	}
	if frame == 0 {
		return errors.New("export: decoder produced no frames")
	}
	if progress != nil {
		progress(1)
	}
	return nil
}

func writeFrame(w io.Writer, frame image.Image, width int) error {
	rgba, ok := frame.(*image.RGBA)
	if !ok {
		return fmt.Errorf("unexpected frame type %T", frame)
	}
	stride := width * 4
	if rgba.Stride == stride {
		_, err := w.Write(rgba.Pix)
		return err
	}
	row := make([]byte, stride)
	for y := rgba.Rect.Min.Y; y < rgba.Rect.Max.Y; y++ {
		start := rgba.PixOffset(rgba.Rect.Min.X, y)
		copy(row, rgba.Pix[start:start+stride])
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func ffmpegPath() string {
	if value := os.Getenv("MYGO_UKE_FFMPEG"); value != "" {
		return value
	}
	return "ffmpeg"
}
