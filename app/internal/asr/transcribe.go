package asr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"unicode"
)

// Line is one finalized sentence with seconds-based timing.
type Line struct {
	Start     float64
	End       float64
	Text      string
	CharTimes []float64
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
		if splitLineWithTimes(line, parts, &out) {
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
		words    []Word
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
				start: start, end: end, text: text, definite: item.Definite, words: item.Words,
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
			g.words = item.Words
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].start < groups[j].start
	})
	lines := make([]Line, 0, len(groups))
	for _, g := range groups {
		start, end := g.start, g.end
		if wordStart, wordEnd, ok := wordSpan(g.words); ok {
			start, end = wordStart, wordEnd
		}
		if duration > 0 {
			start = min(start, duration)
			end = min(end, duration)
		}
		if end <= start {
			continue
		}
		lines = append(lines, Line{
			Start:     start,
			End:       end,
			Text:      g.text,
			CharTimes: charTimesFromWords(g.text, g.words, start, end),
		})
	}
	return lines
}

func wordSpan(words []Word) (float64, float64, bool) {
	start, end := math.Inf(1), math.Inf(-1)
	for _, word := range words {
		if word.EndMS <= word.StartMS {
			continue
		}
		start = math.Min(start, word.StartMS/1000)
		end = math.Max(end, word.EndMS/1000)
	}
	if math.IsInf(start, 1) || end <= start {
		return 0, 0, false
	}
	return start, end, true
}

// splitLineWithTimes keeps word-level timings when one utterance is longer
// than a subtitle line. It returns false when the parts cannot be located in
// the original text, so the caller can fall back to proportional splitting.
func splitLineWithTimes(line Line, parts []string, out *[]Line) bool {
	runes := []rune(line.Text)
	if len(line.CharTimes) != len(runes) || len(parts) == 0 {
		return false
	}
	var split []Line
	search := 0
	for i, part := range parts {
		partRunes := []rune(part)
		offset := indexRunes(runes[search:], partRunes)
		if offset < 0 {
			return false
		}
		start := search + offset
		end := start + len(partRunes)
		partStart := line.CharTimes[start]
		partEnd := line.End
		if i+1 < len(parts) {
			if next := indexRunes(runes[end:], []rune(parts[i+1])); next >= 0 {
				partEnd = line.CharTimes[end+next]
			}
		}
		if partEnd <= partStart {
			partEnd = partStart + 0.2
		}
		split = append(split, Line{
			Start:     partStart,
			End:       partEnd,
			Text:      part,
			CharTimes: append([]float64(nil), line.CharTimes[start:end]...),
		})
		search = end
	}
	*out = append(*out, split...)
	return true
}

func indexRunes(haystack, needle []rune) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// charTimesFromWords turns word/character timestamps into a start time for
// every rune in the sentence. Punctuation inherits the previous character.
func charTimesFromWords(text string, words []Word, start, end float64) []float64 {
	runes := []rune(text)
	if len(runes) == 0 || len(words) == 0 {
		return nil
	}
	type token struct {
		r     rune
		start float64
	}
	var tokens []token
	for _, word := range words {
		if word.EndMS <= word.StartMS {
			continue
		}
		var kept []rune
		for _, r := range []rune(word.Text) {
			if singableRune(r) {
				kept = append(kept, unicode.ToLower(r))
			}
		}
		if len(kept) == 0 {
			continue
		}
		step := (word.EndMS - word.StartMS) / float64(len(kept))
		for i, r := range kept {
			tokens = append(tokens, token{
				r:     r,
				start: (word.StartMS + step*float64(i)) / 1000,
			})
		}
	}
	if len(tokens) == 0 {
		return nil
	}
	times := make([]float64, len(runes))
	for i := range times {
		times[i] = -1
	}
	next := 0
	for i, r := range runes {
		if !singableRune(r) {
			continue
		}
		target := unicode.ToLower(r)
		for next < len(tokens) && tokens[next].r != target {
			next++
		}
		if next >= len(tokens) {
			break
		}
		times[i] = clampTime(tokens[next].start, start, end)
		next++
	}
	return fillRuneTimes(runes, times, start, end)
}

func fillRuneTimes(runes []rune, times []float64, start, end float64) []float64 {
	first := -1.0
	for _, value := range times {
		if value >= 0 {
			first = value
			break
		}
	}
	if first < 0 {
		return nil
	}
	prev := -1
	for i := range runes {
		if times[i] >= 0 {
			prev = i
			continue
		}
		if !singableRune(runes[i]) {
			if prev >= 0 {
				times[i] = times[prev]
			} else {
				times[i] = first
			}
			continue
		}
		next := -1
		for j := i + 1; j < len(times); j++ {
			if times[j] >= 0 {
				next = j
				break
			}
		}
		switch {
		case prev >= 0 && next >= 0:
			times[i] = times[prev] + (times[next]-times[prev])*float64(i-prev)/float64(next-prev)
		case prev >= 0:
			times[i] = times[prev] + (end-times[prev])/float64(max(1, len(runes)-1-prev))
		default:
			times[i] = first
		}
	}
	for i := 1; i < len(times); i++ {
		if times[i] < times[i-1] {
			times[i] = times[i-1]
		}
	}
	return times
}

func singableRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func clampTime(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
