package sheet

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
)

// TimedLine is one chart line placed on the song timeline.
type TimedLine struct {
	Chords []project.ChordMark
	Lyrics string
	Start  float64
	End    float64
}

// Align matches chart lines to the ASR transcript and gives each line a span.
func Align(s *Sheet, asr []project.Line, duration float64) []TimedLine {
	if len(s.Lines) == 0 || duration <= 0 {
		return nil
	}
	found := anchors(s.Lines, asr, duration)
	out := make([]TimedLine, 0, len(s.Lines))
	for i, line := range s.Lines {
		start := clamp(found[i].start, 0, duration)
		end := clamp(found[i].end, 0, duration)
		if end <= start {
			end = math.Min(duration, start+0.5)
		}
		out = append(out, TimedLine{
			Chords: line.Chords,
			Lyrics: line.Lyrics,
			Start:  round(start),
			End:    round(end),
		})
	}
	return out
}

// Chords turns each line's character-anchored chords into absolute time
// segments. A chord lasts until the next anchor, including across a line
// boundary where the next line does not start a new chord.
func Chords(lines []TimedLine, duration float64) []score.ChordSegment {
	type event struct {
		start float64
		label string
	}
	var events []event
	for _, line := range lines {
		if len(line.Chords) == 0 || line.End <= line.Start {
			continue
		}
		marks := project.CleanChordMarks(line.Chords, line.Lyrics)
		if len(marks) == 0 {
			continue
		}
		if strings.TrimSpace(line.Lyrics) == "" {
			start := line.Start
			for _, mark := range marks {
				events = append(events, event{start: start, label: mark.Label})
				start += (line.End - line.Start) / float64(len(marks))
			}
			continue
		}
		times := (project.Line{Start: line.Start, End: line.End, Text: line.Lyrics}).RuneTimes()
		for _, mark := range marks {
			at := mark.At
			if at < 0 {
				at = 0
			}
			if at >= len(times) {
				at = len(times) - 1
			}
			events = append(events, event{start: times[at], label: mark.Label})
		}
	}
	if len(events) == 0 {
		return nil
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].start < events[j].start })
	dedup := events[:0]
	for _, item := range events {
		if item.start < 0 {
			item.start = 0
		}
		if duration > 0 && item.start > duration {
			continue
		}
		if len(dedup) > 0 && math.Abs(dedup[len(dedup)-1].start-item.start) < 1e-6 {
			dedup[len(dedup)-1].label = item.label
			continue
		}
		dedup = append(dedup, item)
	}
	out := make([]score.ChordSegment, 0, len(dedup))
	for i, item := range dedup {
		end := duration
		if i+1 < len(dedup) {
			end = dedup[i+1].start
		}
		if end <= item.start {
			continue
		}
		out = append(out, score.ChordSegment{
			Start:  round(item.start),
			End:    round(end),
			Label:  item.label,
			Conf:   1,
			Source: "chart",
		})
	}
	return out
}

// Apply replaces the project's lyric and chord timeline with the chart.
func Apply(p *project.Project, s *Sheet) {
	timed := Align(s, p.Lines, p.Duration)
	if len(timed) == 0 {
		return
	}
	lines := make([]project.Line, 0, len(timed))
	for _, line := range timed {
		if strings.TrimSpace(line.Lyrics) == "" {
			continue
		}
		lines = append(lines, project.Line{
			Start:  line.Start,
			End:    line.End,
			Text:   line.Lyrics,
			Conf:   1,
			Chords: append([]project.ChordMark(nil), line.Chords...),
		})
	}
	if len(lines) > 0 {
		p.Lines = lines
		p.LinesEdited = true
		p.Warnings = nil
	}
	if chords := Chords(timed, p.Duration); len(chords) > 0 {
		p.Chords = chords
	}
	if s.Key != "" {
		p.Key = s.Key
	}
	if s.Capo > 0 {
		p.Capo = s.Capo
	}
	applyGrid(p, s, timed)
	p.RebuildBars()
	for i := range p.Bars {
		if len(p.Bars[i].Chords) == 0 {
			continue
		}
		p.Bars[i].Manual = true
		p.Bars[i].NeedsReview = false
		p.Bars[i].Conf = 1
	}
}

// applyGrid builds the beat and bar grid without audio analysis. A chart
// with a printed BPM gives an even grid; otherwise each chart chord change
// starts a bar, which keeps the editor usable without pretending to know
// the real downbeats.
func applyGrid(p *project.Project, s *Sheet, lines []TimedLine) {
	if p == nil {
		return
	}
	if s.BPM > 0 {
		beat := 60 / s.BPM
		if beat <= 0 {
			return
		}
		signature := s.TimeSignature
		if signature[0] <= 0 || signature[1] <= 0 {
			signature = [2]int{4, 4}
		}
		p.Tempo = s.BPM
		p.TimeSignature = signature
		p.Beats = p.Beats[:0]
		for t := 0.0; t <= p.Duration+1e-6; t += beat {
			p.Beats = append(p.Beats, round(t))
		}
		p.Downbeats = p.Downbeats[:0]
		for i := 0; i < len(p.Beats); i += signature[0] {
			p.Downbeats = append(p.Downbeats, p.Beats[i])
		}
		return
	}
	if len(p.Beats) >= 2 && len(p.Downbeats) >= 2 {
		return
	}

	starts := []float64{0}
	if len(p.Chords) > 0 {
		for _, segment := range p.Chords {
			starts = append(starts, segment.Start)
		}
	} else {
		for _, line := range lines {
			starts = append(starts, line.Start)
		}
	}
	sort.Float64s(starts)
	grid := starts[:0]
	for _, start := range starts {
		if start < 0 || start >= p.Duration {
			continue
		}
		if len(grid) > 0 && start-grid[len(grid)-1] < 0.2 {
			continue
		}
		grid = append(grid, round(start))
	}
	p.Downbeats = grid
}

// FromProject rebuilds the chart view of a project so a fresh transcription
// can be aligned against the same lyrics and chord anchors.
func FromProject(p *project.Project) *Sheet {
	if p == nil {
		return nil
	}
	hasChords := false
	lines := make([]Line, 0, len(p.Lines))
	for _, line := range p.Lines {
		marks := append([]project.ChordMark(nil), line.Chords...)
		if len(marks) > 0 {
			hasChords = true
		}
		lines = append(lines, Line{Chords: marks, Lyrics: line.Text})
	}
	if !hasChords {
		return nil
	}
	return &Sheet{
		Key:           p.Key,
		Capo:          p.Capo,
		BPM:           p.Tempo,
		TimeSignature: p.TimeSignature,
		Lines:         lines,
	}
}

type span struct {
	start float64
	end   float64
}

func anchors(sheetLines []Line, asr []project.Line, duration float64) []span {
	m, n := len(sheetLines), len(asr)
	dp := make([][]float64, m+1)
	for i := range dp {
		dp[i] = make([]float64, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			best := math.Max(dp[i-1][j], dp[i][j-1])
			if sim := similarity(sheetLines[i-1].Lyrics, asr[j-1].Text); sim >= 0.25 {
				best = math.Max(best, dp[i-1][j-1]+sim)
			}
			dp[i][j] = best
		}
	}

	match := make([]int, m)
	for i := range match {
		match[i] = -1
	}
	for i, j := m, n; i > 0 && j > 0; {
		sim := similarity(sheetLines[i-1].Lyrics, asr[j-1].Text)
		switch {
		case sim >= 0.25 && dp[i][j] == dp[i-1][j-1]+sim:
			match[i-1] = j - 1
			i--
			j--
		case dp[i][j] == dp[i-1][j]:
			i--
		default:
			j--
		}
	}

	out := make([]span, m)
	for i := range out {
		out[i].start = -1
	}
	for i, j := range match {
		if j >= 0 {
			out[i].start = asr[j].Start
			out[i].end = asr[j].End
		}
	}
	for i := range out {
		if out[i].start >= 0 {
			continue
		}
		prev, next := -1, -1
		for p := i - 1; p >= 0; p-- {
			if out[p].start >= 0 {
				prev = p
				break
			}
		}
		for q := i + 1; q < m; q++ {
			if out[q].start >= 0 {
				next = q
				break
			}
		}
		switch {
		case prev >= 0 && next >= 0:
			ratio := float64(i-prev) / float64(next-prev)
			out[i].start = out[prev].start + (out[next].start-out[prev].start)*ratio
		case prev >= 0:
			base := math.Max(out[prev].start, out[prev].end)
			out[i].start = base + (duration-base)*float64(i-prev)/float64(m-prev)
		case next >= 0:
			out[i].start = out[next].start * float64(i) / float64(next)
		default:
			out[i].start = duration * float64(i) / float64(m)
		}
	}
	for i := 1; i < m; i++ {
		if out[i].start <= out[i-1].start {
			out[i].start = out[i-1].start + 0.3
		}
	}
	for i := m - 1; i >= 0; i-- {
		if out[i].end > out[i].start {
			if i+1 < m && out[i].end > out[i+1].start {
				out[i].end = out[i+1].start
			}
			continue
		}
		if i+1 < m {
			out[i].end = out[i+1].start
		} else {
			out[i].end = duration
		}
		if out[i].end <= out[i].start {
			out[i].end = math.Min(duration, out[i].start+2)
		}
	}
	return out
}

func similarity(a, b string) float64 {
	left, right := []rune(compact(a)), []rune(compact(b))
	if len(left) < 2 || len(right) < 2 {
		return 0
	}
	grams := make(map[string]bool, len(left))
	for i := 0; i+1 < len(left); i++ {
		grams[string(left[i:i+2])] = true
	}
	hits := 0
	for i := 0; i+1 < len(right); i++ {
		if grams[string(right[i:i+2])] {
			hits++
		}
	}
	return float64(hits) / float64(len(right)-1)
}

func compact(s string) string {
	var out strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(unicode.ToLower(r))
		}
	}
	return out.String()
}

func round(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
