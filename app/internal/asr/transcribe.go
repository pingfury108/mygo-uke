package asr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Line is one finalized sentence with seconds-based timing.
type Line struct {
	Start float64
	End   float64
	Text  string
}

// TranscribeFile streams a 16kHz/16bit/mono raw PCM file through the
// bidirectional ASR endpoint and returns the final sentence list.
func TranscribeFile(
	ctx context.Context,
	cfg Config,
	path string,
	duration float64,
	progress func(float64),
) ([]Line, error) {
	cfg = cfg.WithDefaults()
	cfg.EnableNonStream = true
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return nil, errors.New("asr: PCM 文件为空")
	}

	client := New(cfg)
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	defer client.Close()

	frame := make([]byte, PCMFrameBytes)
	var sent int64
	for {
		n, readErr := io.ReadFull(file, frame)
		if n > 0 {
			chunk := frame[:n]
			if n < len(frame) {
				padded := make([]byte, PCMFrameBytes)
				copy(padded, chunk)
				chunk = padded
			}
			if err := client.SendFrame(ctx, chunk); err != nil {
				return nil, err
			}
			sent += int64(n)
			if progress != nil {
				progress(float64(sent) / float64(info.Size()) * 0.9)
			}
		}
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	if err := client.SendEnd(ctx); err != nil {
		return nil, err
	}
	var finalized []Utterance
	for {
		select {
		case result, ok := <-client.Results():
			if !ok {
				return nil, errors.New("asr: 连接关闭，但没有收到定稿")
			}
			if result.Err != nil {
				return nil, result.Err
			}
			finalized = mergeUtterances(finalized, result.Utterances)
			if !result.Final {
				continue
			}
			lines := linesFrom(finalized, duration)
			if len(lines) == 0 {
				return nil, fmt.Errorf(
					"asr: 定稿没有句级 utterances（文本：%q）",
					strings.TrimSpace(result.Text),
				)
			}
			if progress != nil {
				progress(1)
			}
			return lines, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func mergeUtterances(dst, src []Utterance) []Utterance {
	type key struct {
		start int64
		end   int64
		text  string
	}
	seen := make(map[key]bool, len(dst))
	for _, item := range dst {
		seen[key{int64(item.StartMS), int64(item.EndMS), item.Text}] = true
	}
	for _, item := range src {
		k := key{int64(item.StartMS), int64(item.EndMS), item.Text}
		if seen[k] {
			continue
		}
		seen[k] = true
		dst = append(dst, item)
	}
	return dst
}

func linesFrom(utterances []Utterance, duration float64) []Line {
	lines := make([]Line, 0, len(utterances))
	for _, item := range utterances {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		start := item.StartMS / 1000
		end := item.EndMS / 1000
		if start < 0 {
			start = 0
		}
		if duration > 0 {
			start = min(start, duration)
			end = min(end, duration)
		}
		if end <= start {
			continue
		}
		lines = append(lines, Line{Start: start, End: end, Text: text})
	}
	return lines
}
