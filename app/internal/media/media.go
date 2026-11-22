// Package media wraps the ffmpeg commands the import pipeline needs.
package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Duration returns a media file's duration in seconds.
func Duration(ctx context.Context, path string) (float64, error) {
	output, _ := exec.CommandContext(
		ctx, FFmpegPath(), "-nostdin", "-hide_banner", "-i", path,
	).CombinedOutput()
	value := parseFFmpegDuration(string(output))
	if value <= 0 {
		return 0, fmt.Errorf("ffmpeg: invalid duration %q", strings.TrimSpace(string(output)))
	}
	return value, nil
}

var durationLine = regexp.MustCompile(`Duration:\s*(\d+):(\d+):(\d+(?:\.\d+)?)`)

func parseFFmpegDuration(output string) float64 {
	match := durationLine.FindStringSubmatch(output)
	if match == nil {
		return 0
	}
	hours, _ := strconv.Atoi(match[1])
	minutes, _ := strconv.Atoi(match[2])
	seconds, _ := strconv.ParseFloat(match[3], 64)
	return float64(hours*3600+minutes*60) + seconds
}

// ExtractAudio writes a 22.05kHz mono 128kbps mp3 for chordd.
func ExtractAudio(ctx context.Context, source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	_, err := run(
		ctx,
		FFmpegPath(),
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

// ExtractPCM writes 16kHz mono signed 16-bit little-endian PCM for ASR.
func ExtractPCM(ctx context.Context, source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	_, err := run(
		ctx,
		FFmpegPath(),
		"-nostdin",
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-i", source,
		"-map", "0:a:0",
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-f", "s16le",
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
