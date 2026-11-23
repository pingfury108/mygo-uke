// Package sheet turns a printed ukulele chart into timed project lines.
package sheet

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"mygo-uke/internal/project"
	"mygo-uke/internal/uke"
)

// Line is one lyric line with the chords printed above it.
type Line struct {
	Chords chordList `json:"chords"`
	Lyrics string    `json:"lyrics"`
}

// chordList accepts chord objects with positions, plain label arrays, and
// "F G" strings, which models mix up. Missing positions are filled over the
// lyric line later.
type chordList []project.ChordMark

func (c *chordList) UnmarshalJSON(data []byte) error {
	var objects []struct {
		At    *int   `json:"at"`
		Index *int   `json:"index"`
		Label string `json:"label"`
		Chord string `json:"chord"`
		Text  string `json:"text"`
	}
	if json.Unmarshal(data, &objects) == nil {
		out := make([]project.ChordMark, 0, len(objects))
		for _, item := range objects {
			label := firstNonEmpty(item.Label, item.Chord, item.Text)
			at := -1
			if item.At != nil {
				at = *item.At
			} else if item.Index != nil {
				at = *item.Index
			}
			if strings.TrimSpace(label) == "" {
				continue
			}
			out = append(out, project.ChordMark{At: at, Label: label})
		}
		if len(out) > 0 {
			*c = out
			return nil
		}
	}
	var list []string
	if json.Unmarshal(data, &list) == nil {
		out := make([]project.ChordMark, 0, len(list))
		for _, label := range list {
			out = append(out, project.ChordMark{At: -1, Label: label})
		}
		*c = out
		return nil
	}
	var text string
	if json.Unmarshal(data, &text) == nil {
		fields := strings.Fields(text)
		out := make([]project.ChordMark, 0, len(fields))
		for _, label := range fields {
			out = append(out, project.ChordMark{At: -1, Label: label})
		}
		*c = out
		return nil
	}
	return fmt.Errorf("chords 既不是数组也不是字符串")
}

// Sheet is a parsed chart before it is aligned to the audio.
type Sheet struct {
	Key           string  `json:"key"`
	Capo          int     `json:"capo"`
	BPM           float64 `json:"bpm,omitempty"`
	TimeSignature [2]int  `json:"timeSignature,omitempty"`
	Lines         []Line  `json:"lines"`
}

var fence = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")

// Decode reads the JSON object a model returned, tolerating fences and prose.
func Decode(raw string) (*Sheet, error) {
	text := extractJSON(raw)
	if text == "" {
		return nil, fmt.Errorf("没有识别出曲谱内容")
	}
	var parsed Sheet
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		var lines []Line
		if err := json.Unmarshal([]byte(text), &lines); err != nil {
			return nil, fmt.Errorf("曲谱内容无法解析：%w", err)
		}
		parsed.Lines = lines
	}
	parsed.normalize()
	if len(parsed.Lines) == 0 {
		return nil, fmt.Errorf("曲谱没有可用歌词行")
	}
	return &parsed, nil
}

func extractJSON(raw string) string {
	text := strings.TrimSpace(raw)
	if match := fence.FindStringSubmatch(text); match != nil {
		text = strings.TrimSpace(match[1])
	}
	object := strings.IndexByte(text, '{')
	array := strings.IndexByte(text, '[')
	if array >= 0 && (object < 0 || array < object) {
		if end := strings.LastIndexByte(text, ']'); end > array {
			return text[array : end+1]
		}
	}
	if object < 0 {
		return ""
	}
	end := strings.LastIndexByte(text, '}')
	if end <= object {
		return ""
	}
	return text[object : end+1]
}

func (s *Sheet) normalize() {
	s.Key = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s.Key), "调"))
	if s.Capo < 0 {
		s.Capo = 0
	}
	if s.BPM < 20 || s.BPM > 300 {
		s.BPM = 0
	}
	if s.TimeSignature[0] <= 0 || s.TimeSignature[1] <= 0 {
		s.TimeSignature = [2]int{4, 4}
	}
	lines := make([]Line, 0, len(s.Lines))
	for _, line := range s.Lines {
		lyrics := strings.TrimSpace(line.Lyrics)
		chords := project.CleanChordMarks([]project.ChordMark(line.Chords), lyrics)
		if lyrics == "" && len(chords) == 0 {
			continue
		}
		lines = append(lines, Line{Chords: chords, Lyrics: lyrics})
	}
	s.Lines = lines
}

// ParseText reads the common "chord row above lyric row" text layout.
func ParseText(text string) (*Sheet, error) {
	var out Sheet
	pending := ""
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || isSectionHeader(line) {
			continue
		}
		if bpm, ok := parseBPM(line); ok {
			out.BPM = bpm
			continue
		}
		if signature, ok := parseTimeSignature(line); ok {
			out.TimeSignature = signature
			continue
		}
		if key, ok := parseKey(line); ok {
			out.Key = key
			continue
		}
		if capo, ok := parseCapo(line); ok {
			out.Capo = capo
			continue
		}
		if tokens := chordTokens(line); len(tokens) > 0 {
			pending = line
			continue
		}
		lyrics, inline := inlineChords(line)
		marks := inline
		if len(marks) == 0 && pending != "" {
			marks = anchorsFromRows(pending, lyrics)
		}
		out.Lines = append(out.Lines, Line{Chords: marks, Lyrics: lyrics})
		pending = ""
	}
	if pending != "" {
		out.Lines = append(out.Lines, Line{Chords: chordTokens(pending), Lyrics: ""})
	}
	out.normalize()
	if len(out.Lines) == 0 {
		return nil, fmt.Errorf("没有识别到曲谱行")
	}
	return &out, nil
}

var (
	bpmPattern   = regexp.MustCompile(`(?i)^(?:bpm|tempo|速度|♩)\s*[:=]?\s*(\d{2,3})(?:\s*bpm)?$`)
	bpmSuffix    = regexp.MustCompile(`(?i)^(\d{2,3})\s*bpm$`)
	meterPattern = regexp.MustCompile(`^(?:拍号\s*[:=]?\s*)?(\d{1,2})\s*/\s*(\d{1,2})$`)
	capoPattern  = regexp.MustCompile(`(?i)^capo\s*[:=]?\s*(\d{1,2})$`)
)

func parseBPM(line string) (float64, bool) {
	match := bpmPattern.FindStringSubmatch(line)
	if match == nil {
		match = bpmSuffix.FindStringSubmatch(line)
	}
	if match == nil {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	if err != nil || value < 20 || value > 300 {
		return 0, false
	}
	return float64(value), true
}

func parseTimeSignature(line string) ([2]int, bool) {
	match := meterPattern.FindStringSubmatch(line)
	if match == nil {
		return [2]int{}, false
	}
	top, errTop := strconv.Atoi(match[1])
	bottom, errBottom := strconv.Atoi(match[2])
	if errTop != nil || errBottom != nil || top < 2 || top > 12 || bottom < 2 || bottom > 16 {
		return [2]int{}, false
	}
	return [2]int{top, bottom}, true
}

func parseKey(line string) (string, bool) {
	text := strings.TrimSpace(line)
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "key") {
		text = strings.TrimSpace(text[len("key"):])
		text = strings.TrimSpace(strings.TrimPrefix(text, ":"))
	} else if strings.HasPrefix(text, "原调") {
		text = strings.TrimSpace(strings.TrimPrefix(text, "原调"))
		text = strings.TrimSpace(strings.TrimPrefix(text, ":"))
	} else if !strings.HasSuffix(text, "调") || len([]rune(text)) > 8 {
		return "", false
	}
	text = strings.TrimSpace(strings.TrimSuffix(text, "调"))
	if text == "" {
		return "", false
	}
	return text, true
}

func parseCapo(line string) (int, bool) {
	match := capoPattern.FindStringSubmatch(strings.TrimSpace(line))
	if match == nil {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	if err != nil || value < 0 || value > 12 {
		return 0, false
	}
	return value, true
}

func isSectionHeader(line string) bool {
	return strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") ||
		strings.HasPrefix(line, "【") && strings.HasSuffix(line, "】")
}

func chordTokens(line string) []project.ChordMark {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	out := make([]project.ChordMark, 0, len(fields))
	for _, field := range fields {
		if field == "|" {
			continue
		}
		if _, ok := uke.Normalize(field); !ok {
			return nil
		}
		out = append(out, project.ChordMark{At: -1, Label: field})
	}
	return out
}

type chordToken struct {
	column int
	label  string
}

func chordColumns(line string) []chordToken {
	var out []chordToken
	column := 0
	start := -1
	var label strings.Builder
	flush := func() {
		if start < 0 {
			return
		}
		value := label.String()
		if value != "" && value != "|" {
			if _, ok := uke.Normalize(value); ok {
				out = append(out, chordToken{column: start, label: value})
			}
		}
		start = -1
		label.Reset()
	}
	for _, r := range line {
		if r == ' ' || r == '\t' {
			flush()
			column += displayWidth(r)
			continue
		}
		if start < 0 {
			start = column
		}
		label.WriteRune(r)
		column += displayWidth(r)
	}
	flush()
	return out
}

func anchorsFromRows(chordRow, lyrics string) []project.ChordMark {
	tokens := chordColumns(chordRow)
	if len(tokens) == 0 {
		return nil
	}
	runes := []rune(lyrics)
	if len(runes) == 0 {
		return chordTokens(chordRow)
	}
	columns := make([]int, len(runes)+1)
	column := 0
	for i, r := range runes {
		columns[i] = column
		column += displayWidth(r)
	}
	columns[len(runes)] = column

	out := make([]project.ChordMark, 0, len(tokens))
	for _, token := range tokens {
		at := 0
		for i := range runes {
			if token.column >= columns[i] {
				at = i
			}
			if token.column >= columns[i] && token.column < columns[i+1] {
				at = i
				break
			}
		}
		for at < len(runes)-1 && !singable(runes[at]) {
			at++
		}
		out = append(out, project.ChordMark{At: at, Label: token.label})
	}
	return out
}

func inlineChords(line string) (string, []project.ChordMark) {
	runes := []rune(line)
	var text strings.Builder
	var marks []project.ChordMark
	at := 0
	for i := 0; i < len(runes); i++ {
		if runes[i] == '[' {
			if end := indexRune(runes[i+1:], ']'); end >= 0 {
				label := strings.TrimSpace(string(runes[i+1 : i+1+end]))
				if _, ok := uke.Normalize(label); ok {
					marks = append(marks, project.ChordMark{At: at, Label: label})
					i += end + 1
					continue
				}
			}
		}
		text.WriteRune(runes[i])
		at++
	}
	return strings.TrimSpace(text.String()), marks
}

func indexRune(runes []rune, target rune) int {
	for i, r := range runes {
		if r == target {
			return i
		}
	}
	return -1
}

func displayWidth(r rune) int {
	switch {
	case r == '\t':
		return 4
	case r == 0:
		return 0
	case r < 32:
		return 0
	case r >= 0x1100 && (r <= 0x115F ||
		r == 0x2329 || r == 0x232A ||
		r >= 0x2E80 && r <= 0xA4CF ||
		r >= 0xAC00 && r <= 0xD7A3 ||
		r >= 0xF900 && r <= 0xFAFF ||
		r >= 0xFE30 && r <= 0xFE6F ||
		r >= 0xFF00 && r <= 0xFF60 ||
		r >= 0xFFE0 && r <= 0xFFE6 ||
		r >= 0x1F300 && r <= 0x1FAFF):
		return 2
	default:
		return 1
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func singable(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
