package project

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"mygo-uke/internal/score"
)

// ChordSpan is a chord covering a rune range [From, To).
type ChordSpan struct {
	From      int
	To        int
	Label     string
	Inherited bool
}

// Phrase is one display segment of a lyric line. The chart's spaces mark
// where the singer breathes, so they also mark where the subtitle changes
// instead of revealing a whole sentence at once.
type Phrase struct {
	From  int
	To    int
	Text  string
	Start float64
	End   float64
}

// Phrases splits the line into display segments. Spaces from the chart win;
// a line without spaces falls back to chord changes, and a line with neither
// stays a single phrase.
func (l Line) Phrases() []Phrase {
	runes := []rune(l.Text)
	if len(runes) == 0 {
		return nil
	}
	ranges := spaceRanges(runes)
	if len(ranges) < 2 {
		ranges = chordRanges(l, runes)
	}
	if len(ranges) == 0 {
		ranges = [][2]int{{0, len(runes)}}
	}
	times := l.RuneTimes()
	phrases := make([]Phrase, 0, len(ranges))
	for i, item := range ranges {
		from, to := item[0], item[1]
		if from < 0 || to > len(runes) || to <= from {
			continue
		}
		start := times[from]
		end := l.End
		if i+1 < len(ranges) {
			end = times[ranges[i+1][0]]
		}
		if end < start {
			end = start
		}
		phrases = append(phrases, Phrase{
			From:  from,
			To:    to,
			Text:  string(runes[from:to]),
			Start: start,
			End:   end,
		})
	}
	return phrases
}

// PhraseAt returns the phrase playing at t.
func (l Line) PhraseAt(t float64) (int, Phrase, bool) {
	phrases := l.Phrases()
	for i, phrase := range phrases {
		if t >= phrase.Start-1e-9 && t < phrase.End-1e-9 {
			return i, phrase, true
		}
	}
	if len(phrases) > 0 && t >= phrases[len(phrases)-1].Start && t <= l.End+1e-9 {
		return len(phrases) - 1, phrases[len(phrases)-1], true
	}
	return 0, Phrase{}, false
}

// NextPhrase returns the phrase that follows the one at index on the
// timeline, crossing into the following lines.
func (p *Project) NextPhrase(lineIndex, phraseIndex int) (int, Phrase, bool) {
	if p == nil || lineIndex < 0 || lineIndex >= len(p.Lines) {
		return 0, Phrase{}, false
	}
	phrases := p.Lines[lineIndex].Phrases()
	if phraseIndex >= 0 && phraseIndex+1 < len(phrases) {
		return lineIndex, phrases[phraseIndex+1], true
	}
	for i := lineIndex + 1; i < len(p.Lines); i++ {
		if next := p.Lines[i].Phrases(); len(next) > 0 {
			return i, next[0], true
		}
	}
	return 0, Phrase{}, false
}

// PrevPhrase returns the phrase before the one at index on the timeline,
// crossing into the previous lines.
func (p *Project) PrevPhrase(lineIndex, phraseIndex int) (int, Phrase, bool) {
	if p == nil || lineIndex < 0 || lineIndex >= len(p.Lines) {
		return 0, Phrase{}, false
	}
	if phraseIndex > 0 {
		phrases := p.Lines[lineIndex].Phrases()
		if phraseIndex <= len(phrases) {
			return lineIndex, phrases[phraseIndex-1], true
		}
	}
	for i := lineIndex - 1; i >= 0; i-- {
		if prev := p.Lines[i].Phrases(); len(prev) > 0 {
			return i, prev[len(prev)-1], true
		}
	}
	return 0, Phrase{}, false
}

func spaceRanges(runes []rune) [][2]int {
	var ranges [][2]int
	start := -1
	for i, r := range runes {
		if unicode.IsSpace(r) {
			if start >= 0 {
				ranges = append(ranges, [2]int{start, i})
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		ranges = append(ranges, [2]int{start, len(runes)})
	}
	return ranges
}

// chordRanges splits on chord anchors, merging one-character segments so a
// phrase never flickers on and off for a single syllable.
func chordRanges(l Line, runes []rune) [][2]int {
	marks := CleanChordMarks(l.Chords, l.Text)
	if len(marks) < 2 {
		return nil
	}
	var ranges [][2]int
	for i, mark := range marks {
		from := mark.At
		to := len(runes)
		if i+1 < len(marks) {
			to = marks[i+1].At
		}
		if from < 0 || to > len(runes) || to <= from {
			continue
		}
		ranges = append(ranges, [2]int{from, to})
	}
	if len(ranges) < 2 {
		return nil
	}
	merged := ranges[:0]
	for _, item := range ranges {
		if len(merged) > 0 && singable(runes[merged[len(merged)-1][0]:merged[len(merged)-1][1]]) < 2 {
			merged[len(merged)-1][1] = item[1]
			continue
		}
		merged = append(merged, item)
	}
	if len(merged) > 1 && singable(runes[merged[len(merged)-1][0]:merged[len(merged)-1][1]]) < 2 {
		merged[len(merged)-2][1] = merged[len(merged)-1][1]
		merged = merged[:len(merged)-1]
	}
	if len(merged) < 2 {
		return nil
	}
	return merged
}

func singable(runes []rune) int {
	n := 0
	for _, r := range runes {
		if timingWeight(r) > 0 {
			n++
		}
	}
	return n
}

// RuneTimes gives every rune its own start time inside the line. Spaces and
// punctuation get no time of their own and appear with the previous rune.
func (l Line) RuneTimes() []float64 {
	runes := []rune(l.Text)
	times := make([]float64, len(runes))
	if len(runes) == 0 {
		return times
	}
	if len(l.CharTimes) == len(runes) {
		copy(times, l.CharTimes)
		return times
	}
	total := 0.0
	for _, r := range runes {
		total += timingWeight(r)
	}
	if total <= 0 || l.End <= l.Start {
		for i := range times {
			times[i] = l.Start
		}
		return times
	}
	elapsed := 0.0
	for i, r := range runes {
		weight := timingWeight(r)
		if weight <= 0 {
			if i == 0 {
				times[i] = l.Start
			} else {
				times[i] = times[i-1]
			}
			continue
		}
		times[i] = l.Start + (l.End-l.Start)*elapsed/total
		elapsed += weight
	}
	return times
}

// VisibleRunes is how many runes have appeared at t.
func (l Line) VisibleRunes(t float64) int {
	times := l.RuneTimes()
	if len(times) == 0 || t < l.Start {
		return 0
	}
	if t >= l.End {
		return len(times)
	}
	n := 0
	for n < len(times) && times[n] <= t+1e-9 {
		n++
	}
	return n
}

// TextAt is the karaoke prefix visible at t.
func (l Line) TextAt(t float64) string {
	runes := []rune(l.Text)
	if t < l.Start {
		return ""
	}
	if t >= l.End {
		return l.Text
	}
	n := l.VisibleRunes(t)
	if n > len(runes) {
		n = len(runes)
	}
	if n < 0 {
		n = 0
	}
	return string(runes[:n])
}

// CurrentRune is the last singable rune that has started by t.
func (l Line) CurrentRune(t float64) int {
	runes := []rune(l.Text)
	times := l.RuneTimes()
	index := -1
	for i := range runes {
		if timingWeight(runes[i]) <= 0 {
			continue
		}
		if times[i] > t+1e-9 {
			break
		}
		index = i
	}
	return index
}

// ChordAtRune returns the chord covering the rune, when the line has one.
func ChordAtRune(spans []ChordSpan, index int) (ChordSpan, bool) {
	for _, span := range spans {
		if index >= span.From && index < span.To {
			return span, true
		}
	}
	return ChordSpan{}, false
}

// CleanChordMarks validates positions against the lyric and gives old
// chord lists without positions an approximate spread over the line.
func CleanChordMarks(marks []ChordMark, text string) []ChordMark {
	runes := []rune(text)
	out := make([]ChordMark, 0, len(marks))
	for _, mark := range marks {
		label := strings.TrimSpace(mark.Label)
		if label == "" || label == "|" || label == "-" {
			continue
		}
		mark.Label = label
		out = append(out, mark)
	}
	if len(out) == 0 {
		return nil
	}
	if len(runes) == 0 {
		for i := range out {
			out[i].At = 0
			out[i].Approx = true
		}
		return out
	}

	valid := true
	for _, mark := range out {
		if mark.At < 0 || mark.At >= len(runes) {
			valid = false
			break
		}
	}
	if !valid {
		positions := make([]int, 0, len(runes))
		for i, r := range runes {
			if timingWeight(r) > 0 {
				positions = append(positions, i)
			}
		}
		if len(positions) == 0 {
			positions = make([]int, len(runes))
			for i := range positions {
				positions[i] = i
			}
		}
		for i := range out {
			at := 0
			if len(out) > 1 {
				at = int(math.Round(float64(i) * float64(len(positions)-1) / float64(len(out)-1)))
			}
			out[i].At = positions[at]
			out[i].Approx = true
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	cleaned := out[:0]
	for _, mark := range out {
		if mark.At < 0 {
			mark.At = 0
		}
		if mark.At >= len(runes) {
			mark.At = len(runes) - 1
		}
		if len(cleaned) > 0 && cleaned[len(cleaned)-1].At == mark.At {
			cleaned[len(cleaned)-1] = mark
			continue
		}
		cleaned = append(cleaned, mark)
	}
	return cleaned
}

// LineChordSpans expands a line's marks into ranges. A chord that began on
// an earlier line is inherited from the effective chord timeline.
func (p *Project) LineChordSpans(index int) []ChordSpan {
	if p == nil || index < 0 || index >= len(p.Lines) {
		return nil
	}
	line := p.Lines[index]
	count := len([]rune(line.Text))
	if count == 0 {
		return nil
	}
	marks := CleanChordMarks(line.Chords, line.Text)
	inherited := ""
	if len(marks) == 0 || marks[0].At > 0 {
		if segment, _, ok := score.ChordAt(p.Chords, line.Start+1e-4); ok {
			inherited = segment.Label
		}
	}
	var spans []ChordSpan
	if inherited != "" && (len(marks) == 0 || marks[0].At > 0) {
		to := count
		if len(marks) > 0 && marks[0].At < to {
			to = marks[0].At
		}
		if to > 0 {
			spans = append(spans, ChordSpan{From: 0, To: to, Label: inherited, Inherited: true})
		}
	}
	for i, mark := range marks {
		from := mark.At
		to := count
		if i+1 < len(marks) && marks[i+1].At > from {
			to = marks[i+1].At
		}
		if to > count {
			to = count
		}
		if to <= from {
			continue
		}
		spans = append(spans, ChordSpan{From: from, To: to, Label: mark.Label})
	}
	return spans
}

// RebuildChartChords refreshes chart-sourced timing after a lyric line is
// edited. Audio-sourced chords outside the chart stay untouched.
func (p *Project) RebuildChartChords() {
	if p == nil {
		return
	}
	type event struct {
		start float64
		label string
	}
	type span struct {
		start float64
		end   float64
	}
	var events []event
	var chartSpans []span
	for _, line := range p.Lines {
		marks := CleanChordMarks(line.Chords, line.Text)
		if len(marks) == 0 || line.End <= line.Start {
			continue
		}
		chartSpans = append(chartSpans, span{start: line.Start, end: line.End})
		times := line.RuneTimes()
		for _, mark := range marks {
			at := mark.At
			if at < 0 {
				at = 0
			}
			if at >= len(times) {
				at = len(times) - 1
			}
			start := line.Start
			if at >= 0 && at < len(times) {
				start = times[at]
			}
			events = append(events, event{start: start, label: mark.Label})
		}
	}
	kept := make([]score.ChordSegment, 0, len(p.Chords))
	for _, segment := range p.Chords {
		covered := false
		for _, item := range chartSpans {
			if segment.Start < item.end && segment.End > item.start {
				covered = true
				break
			}
		}
		if !covered {
			kept = append(kept, segment)
		}
	}
	if len(events) == 0 {
		p.Chords = kept
		p.RebuildBars()
		return
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].start < events[j].start })
	dedup := events[:0]
	for _, item := range events {
		if len(dedup) > 0 && math.Abs(dedup[len(dedup)-1].start-item.start) < 1e-6 {
			dedup[len(dedup)-1].label = item.label
			continue
		}
		dedup = append(dedup, item)
	}
	for i, item := range dedup {
		end := p.Duration
		if i+1 < len(dedup) {
			end = dedup[i+1].start
		}
		if end <= item.start {
			continue
		}
		kept = append(kept, score.ChordSegment{
			Start:  item.start,
			End:    end,
			Label:  item.label,
			Conf:   1,
			Source: "chart",
		})
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Start < kept[j].Start })
	p.Chords = kept
	p.RebuildBars()
}

// HasChartChords reports whether the project is driven by an imported chart.
func (p *Project) HasChartChords() bool {
	if p == nil {
		return false
	}
	for _, line := range p.Lines {
		if len(line.Chords) > 0 {
			return true
		}
	}
	for _, segment := range p.Chords {
		if segment.Source == "chart" {
			return true
		}
	}
	return false
}

// SetChartChordAt edits the chart chord active at t. When t falls inside a
// lyric line, the edit becomes a character anchor; otherwise it edits the
// chart segment directly.
func (p *Project) SetChartChordAt(t float64, label string) bool {
	if p == nil {
		return false
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return false
	}
	for i := range p.Lines {
		line := &p.Lines[i]
		if t < line.Start || t > line.End || strings.TrimSpace(line.Text) == "" {
			continue
		}
		at := line.CurrentRune(t)
		if at < 0 {
			at = 0
		}
		marks := CleanChordMarks(line.Chords, line.Text)
		replaced := false
		for k := range marks {
			if marks[k].At == at {
				marks[k].Label = label
				marks[k].Approx = false
				replaced = true
				break
			}
		}
		if !replaced {
			marks = append(marks, ChordMark{At: at, Label: label})
			sort.SliceStable(marks, func(a, b int) bool { return marks[a].At < marks[b].At })
		}
		line.Chords = marks
		return true
	}
	for i := range p.Chords {
		segment := &p.Chords[i]
		if t >= segment.Start && t < segment.End {
			segment.Label = label
			segment.Source = "chart"
			segment.Conf = 1
			return true
		}
	}
	return false
}

func timingWeight(r rune) float64 {
	switch {
	case unicode.IsSpace(r):
		return 0
	case unicode.IsPunct(r), unicode.IsSymbol(r):
		return 0
	case unicode.IsLetter(r), unicode.IsDigit(r):
		return 1
	default:
		return 1
	}
}
