package asr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
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
	lastReport := -1.0
	emit := func(value float64) {
		if progress == nil {
			return
		}
		if value < 0.99 && value-lastReport < 0.02 {
			return
		}
		lastReport = value
		progress(value)
	}
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
			emit(float64(sent) / float64(info.Size()) * 0.9)
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
	var last []Utterance
	for {
		select {
		case result, ok := <-client.Results():
			if !ok {
				return nil, errors.New("asr: 连接关闭，但没有收到定稿")
			}
			if result.Err != nil {
				return nil, result.Err
			}
			if len(result.Utterances) > 0 {
				last = result.Utterances
			}
			if !result.Final {
				continue
			}
			utterances := result.Utterances
			if len(utterances) == 0 {
				utterances = last
			}
			lines := linesFrom(utterances, duration)
			lines = splitLongLines(lines)
			if len(lines) == 0 {
				return nil, fmt.Errorf(
					"asr: 定稿没有句级 utterances（文本：%q）",
					strings.TrimSpace(result.Text),
				)
			}
			emit(1)
			return lines, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// splitLongLines turns a long utterance into subtitle-sized lines. Singing
// often has no long pauses, so the ASR may return one 20-second utterance
// with several punctuation marks. Times are apportioned by text length.
func splitLongLines(lines []Line) []Line {
	var out []Line
	for _, line := range lines {
		parts := splitText(line.Text)
		if len(parts) <= 1 {
			out = append(out, line)
			continue
		}
		total := 0
		for _, part := range parts {
			total += len([]rune(part))
		}
		if total == 0 {
			continue
		}
		start := line.Start
		for i, part := range parts {
			end := line.End
			if i < len(parts)-1 {
				share := float64(len([]rune(part))) / float64(total)
				end = start + (line.End-line.Start)*share
			}
			if end <= start {
				end = start + 0.2
			}
			out = append(out, Line{Start: start, End: end, Text: part})
			start = end
		}
	}
	return out
}

func splitText(text string) []string {
	var sentences []string
	var current strings.Builder
	flush := func() {
		value := strings.TrimSpace(current.String())
		if value != "" {
			sentences = append(sentences, value)
		}
		current.Reset()
	}
	for _, r := range text {
		current.WriteRune(r)
		switch r {
		case '。', '！', '？', '!', '?', '；', ';', '\n':
			flush()
		}
	}
	flush()

	var out []string
	for _, sentence := range sentences {
		out = append(out, splitClauses(sentence)...)
	}
	return out
}

func splitClauses(text string) []string {
	if len([]rune(text)) <= 18 {
		return []string{text}
	}
	var clauses []string
	var current strings.Builder
	flush := func() {
		value := strings.TrimSpace(current.String())
		if value != "" {
			clauses = append(clauses, value)
		}
		current.Reset()
	}
	for _, r := range text {
		current.WriteRune(r)
		switch r {
		case '，', ',', '、':
			flush()
		}
	}
	flush()
	if len(clauses) <= 1 {
		return hardSplit(text, 18)
	}
	var out []string
	for _, clause := range clauses {
		out = append(out, hardSplit(clause, 18)...)
	}
	return out
}

func hardSplit(text string, size int) []string {
	runes := []rune(text)
	if len(runes) <= size {
		return []string{text}
	}
	var out []string
	for len(runes) > 0 {
		n := min(size, len(runes))
		out = append(out, strings.TrimSpace(string(runes[:n])))
		runes = runes[n:]
	}
	return out
}

func linesFrom(utterances []Utterance, duration float64) []Line {
	type group struct {
		start    float64
		end      float64
		text     string
		definite bool
	}
	groups := make([]group, 0, len(utterances))
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
		if end <= start {
			continue
		}
		// The service revises a sentence in place: several utterances share
		// one start and grow in end time/text. Keep the best version.
		index := -1
		for i := range groups {
			if abs(groups[i].start-start) <= 0.3 {
				index = i
				break
			}
		}
		if index < 0 {
			groups = append(groups, group{
				start: start, end: end, text: text, definite: item.Definite,
			})
			continue
		}
		g := &groups[index]
		better := item.Definite && !g.definite
		if item.Definite == g.definite {
			better = end > g.end || (end == g.end && len([]rune(text)) > len([]rune(g.text)))
		}
		if better {
			g.end = end
			g.text = text
			g.definite = item.Definite
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].start < groups[j].start
	})
	lines := make([]Line, 0, len(groups))
	for _, g := range groups {
		start, end := g.start, g.end
		if duration > 0 {
			start = min(start, duration)
			end = min(end, duration)
		}
		if end <= start {
			continue
		}
		lines = append(lines, Line{Start: start, End: end, Text: g.text})
	}
	return lines
}

func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
