// Package media wraps the ffmpeg commands the import pipeline needs.
package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Duration returns a media file's duration in seconds.
func Duration(ctx context.Context, path string) (float64, error) {
	output, err := run(
		ctx,
		env("MYGO_UKE_FFPROBE", "ffprobe"),
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	if err != nil {
		return 0, err
	}
	value := parseDuration(string(output))
	if value <= 0 {
		return 0, fmt.Errorf("ffprobe: invalid duration %q", strings.TrimSpace(string(output)))
	}
	return value, nil
}

func parseDuration(output string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(output), 64)
	if err != nil {
		return 0
	}
	return value
}

// ExtractAudio writes a 22.05kHz mono 128kbps mp3 for chordd.
func ExtractAudio(ctx context.Context, source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	_, err := run(
		ctx,
		env("MYGO_UKE_FFMPEG", "ffmpeg"),
		"-nostdin",
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-i", source,
		"-map", "0:a:0",
		"-vn",
		"-ac", "1",
		"-ar", "22050",
		"-b:a", "128k",
		destination,
	)
	return err
}

func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("%s: %s", filepath.Base(name), message)
	}
	return output, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
