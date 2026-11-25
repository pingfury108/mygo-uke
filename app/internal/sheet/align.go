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
	Chords    []project.ChordMark
	Lyrics    string
	CharTimes []float64
	Start     float64
	End       float64
}

// Align matches chart lines to the ASR transcript and gives each line a span.
func Align(s *Sheet, asr []project.Line, duration float64) []TimedLine {
	if len(s.Lines) == 0 || duration <= 0 {
		return nil
	}
	found := anchors(s.Lines, asr, duration)
	out := make([]TimedLine, 0, len(s.Lines))
	for i, line := range s.Lines {
		if found[i].start < 0 {
			continue
		}
		start := clamp(found[i].start, 0, duration)
		end := clamp(found[i].end, 0, duration)
		if end <= start {
			end = math.Min(duration, start+0.5)
		}
		out = append(out, TimedLine{
			Chords:    line.Chords,
			Lyrics:    line.Lyrics,
			CharTimes: found[i].chars,
			Start:     round(start),
			End:       round(end),
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
		times := (project.Line{
			Start: line.Start, End: line.End, Text: line.Lyrics, CharTimes: line.CharTimes,
		}).RuneTimes()
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
	if p == nil || s == nil {
		return
	}
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
			Start:     line.Start,
			End:       line.End,
			Text:      line.Lyrics,
			CharTimes: append([]float64(nil), line.CharTimes...),
			Conf:      1,
			Chords:    append([]project.ChordMark(nil), line.Chords...),
		})
	}
	if len(lines) > 0 {
		p.Lines = lines
		p.LinesEdited = true
		p.Warnings = nil
	}
	p.ChartLines = make([]project.ChartLine, 0, len(s.Lines))
	for _, line := range s.Lines {
		p.ChartLines = append(p.ChartLines, project.ChartLine{
			Lyrics: line.Lyrics,
			Chords: append([]project.ChordMark(nil), line.Chords...),
		})
	}
	p.ChartSnapshot = project.ChartSnapshotVersion
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
	if p == nil || !p.HasChartChords() {
		return nil
	}
	if len(p.ChartLines) > 0 {
		lines := make([]Line, 0, len(p.ChartLines))
		for _, line := range p.ChartLines {
			lines = append(lines, Line{
				Lyrics: line.Lyrics,
				Chords: chordList(append([]project.ChordMark(nil), line.Chords...)),
			})
		}
		return &Sheet{
			Key:           p.Key,
			Capo:          p.Capo,
			BPM:           p.Tempo,
			TimeSignature: p.TimeSignature,
			Lines:         lines,
		}
	}
	type item struct {
		start float64
		line  Line
	}
	items := make([]item, 0, len(p.Lines))
	for _, line := range p.Lines {
		marks := append([]project.ChordMark(nil), line.Chords...)
		items = append(items, item{
			start: line.Start,
			line:  Line{Chords: marks, Lyrics: line.Text},
		})
	}

	segments := append([]score.ChordSegment(nil), p.Chords...)
	sort.SliceStable(segments, func(i, j int) bool { return segments[i].Start < segments[j].Start })
	var group []score.ChordSegment
	flush := func() {
		if len(group) == 0 {
			return
		}
		marks := make([]project.ChordMark, 0, len(group))
		for _, segment := range group {
			marks = append(marks, project.ChordMark{At: -1, Label: segment.Label})
		}
		items = append(items, item{
			start: group[0].Start,
			line:  Line{Chords: chordList(marks)},
		})
		group = nil
	}
	for _, segment := range segments {
		if segment.Source != "chart" {
			continue
		}
		if outsideLines(segment, p.Lines) {
			if len(group) > 0 && segment.Start > group[len(group)-1].End+0.01 {
				flush()
			}
			group = append(group, segment)
			continue
		}
		flush()
	}
	flush()

	sort.SliceStable(items, func(i, j int) bool { return items[i].start < items[j].start })
	lines := make([]Line, 0, len(items))
	for _, current := range items {
		lines = append(lines, current.line)
	}
	return &Sheet{
		Key:           p.Key,
		Capo:          p.Capo,
		BPM:           p.Tempo,
		TimeSignature: p.TimeSignature,
		Lines:         lines,
	}
}

func outsideLines(segment score.ChordSegment, lines []project.Line) bool {
	for _, line := range lines {
		if segment.Start < line.End && segment.End > line.Start {
			return false
		}
	}
	return true
}

type span struct {
	start float64
	end   float64
	chars []float64
}

func anchors(sheetLines []Line, asr []project.Line, duration float64) []span {
	out := make([]span, len(sheetLines))
	for i := range out {
		out[i].start = -1
	}

	chart, asrChars := timelineChars(sheetLines, asr)
	if len(chart) == 0 || len(asrChars) == 0 {
		return spreadSpans(sheetLines, duration)
	}
	pairs := lcsPairs(chart, asrChars)
	if len(pairs) == 0 {
		return spreadSpans(sheetLines, duration)
	}

	type matchInfo struct {
		count      int
		firstStart float64
		lastEnd    float64
	}
	matches := make(map[int]*matchInfo)
	for _, pair := range pairs {
		line := chart[pair.chart].line
		info := matches[line]
		if info == nil {
			info = &matchInfo{firstStart: asrChars[pair.asr].start}
			matches[line] = info
		}
		info.count++
		info.lastEnd = asrChars[pair.asr].end
	}

	matchedLyric := false
	for i, line := range sheetLines {
		total := len([]rune(compact(line.Lyrics)))
		info := matches[i]
		if info == nil || total == 0 {
			continue
		}
		coverage := float64(info.count) / float64(total)
		if coverage < 0.3 {
			continue
		}
		out[i] = span{start: clamp(info.firstStart, 0, duration), end: clamp(info.lastEnd, 0, duration)}
		matchedLyric = true
	}
	if !matchedLyric {
		return spreadSpans(sheetLines, duration)
	}

	fillChordOnlySpans(sheetLines, out, duration)
	fillLeadingSpans(sheetLines, out)
	normalizeSpans(out, duration)
	fillLineTimes(sheetLines, out, chart, pairs, asrChars)
	return out
}

type chartChar struct {
	r    rune
	line int
	pos  int
}

type asrChar struct {
	r     rune
	start float64
	end   float64
}

type charPair struct {
	chart int
	asr   int
}

// timelineChars flattens the chart and ASR text into comparable runes. Each
// ASR rune gets an equal share of its sentence span, which is the finest
// timing the streaming ASR exposes.
func timelineChars(lines []Line, asr []project.Line) ([]chartChar, []asrChar) {
	var chart []chartChar
	for i, line := range lines {
		for pos, r := range []rune(line.Lyrics) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				chart = append(chart, chartChar{r: unicode.ToLower(r), line: i, pos: pos})
			}
		}
	}
	var spoken []asrChar
	for _, line := range asr {
		runes := []rune(line.Text)
		if len(runes) == 0 || line.End <= line.Start {
			continue
		}
		times := line.RuneTimes()
		var timed []struct {
			r     rune
			start float64
		}
		for i, r := range runes {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				timed = append(timed, struct {
					r     rune
					start float64
				}{r: unicode.ToLower(r), start: times[i]})
			}
		}
		for i, item := range timed {
			end := line.End
			if i+1 < len(timed) && timed[i+1].start > item.start {
				end = timed[i+1].start
			}
			spoken = append(spoken, asrChar{r: item.r, start: item.start, end: end})
		}
	}
	return chart, spoken
}

// lcsPairs aligns the full ASR transcript against a prefix of the chart.
// The chart tail is free: a video may only cover part of a repeated chart.
// Skipped chart characters cost a little, so an earlier verse wins over an
// identical later repeat instead of matching the same words at the end.
func lcsPairs(chart []chartChar, asr []asrChar) []charPair {
	n, m := len(chart), len(asr)
	const chartGap = -0.1
	dp := make([][]float64, n+1)
	choice := make([][]byte, n+1)
	for i := range dp {
		dp[i] = make([]float64, m+1)
		choice[i] = make([]byte, m+1)
		dp[i][0] = float64(i) * chartGap
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			best := dp[i-1][j] + chartGap
			choice[i][j] = 0
			if dp[i][j-1] > best {
				best = dp[i][j-1]
				choice[i][j] = 1
			}
			if chart[i-1].r == asr[j-1].r {
				best = dp[i-1][j-1] + 1
				choice[i][j] = 2
			}
			dp[i][j] = best
		}
	}
	end, score := 0, 0.0
	for i := 0; i <= n; i++ {
		if dp[i][m] > score {
			score = dp[i][m]
			end = i
		}
	}
	if score == 0 {
		return nil
	}
	var reversed []charPair
	for i, j := end, m; i > 0 && j > 0; {
		switch choice[i][j] {
		case 2:
			reversed = append(reversed, charPair{chart: i - 1, asr: j - 1})
			i--
			j--
		case 0:
			i--
		default:
			j--
		}
	}
	pairs := make([]charPair, len(reversed))
	for i := range reversed {
		pairs[len(reversed)-1-i] = reversed[i]
	}
	return pairs
}

// fillChordOnlySpans gives instrumental chord rows a span between their
// neighboring lyric lines. Apply still keeps them out of the subtitle list.
func fillChordOnlySpans(lines []Line, out []span, duration float64) {
	for i, line := range lines {
		if out[i].start >= 0 || strings.TrimSpace(line.Lyrics) != "" {
			continue
		}
		prev, next := -1, -1
		for p := i - 1; p >= 0; p-- {
			if out[p].start >= 0 {
				prev = p
				break
			}
		}
		for q := i + 1; q < len(lines); q++ {
			if out[q].start >= 0 {
				next = q
				break
			}
		}
		start, end := 0.0, duration
		if prev >= 0 {
			start = out[prev].end
		}
		if next >= 0 {
			end = out[next].start
		}
		if end <= start {
			continue
		}
		out[i] = span{start: start, end: end}
	}
}

// fillLeadingSpans keeps lyric lines at the top of the chart from vanishing
// when the ASR missed them. They take the time before the first matched line,
// so a missing first phrase still appears instead of leaving a blank intro.
func fillLeadingSpans(lines []Line, out []span) {
	first := -1
	for i, line := range lines {
		if strings.TrimSpace(line.Lyrics) == "" {
			continue
		}
		if out[i].start >= 0 {
			first = i
			break
		}
	}
	if first <= 0 {
		return
	}
	var missing []int
	total := 0
	for i := 0; i < first; i++ {
		if out[i].start >= 0 || strings.TrimSpace(lines[i].Lyrics) == "" {
			continue
		}
		missing = append(missing, i)
		total += max(1, len([]rune(compact(lines[i].Lyrics))))
	}
	if len(missing) == 0 {
		return
	}
	end := out[first].start
	if end <= 0 {
		return
	}
	cursor := 0.0
	covered := 0
	for _, i := range missing {
		weight := max(1, len([]rune(compact(lines[i].Lyrics))))
		covered += weight
		next := end * float64(covered) / float64(total)
		out[i] = span{start: round(cursor), end: round(next)}
		cursor = next
	}
}

func normalizeSpans(out []span, duration float64) {
	last := -1
	for i := range out {
		if out[i].start < 0 {
			continue
		}
		if last >= 0 && out[i].start < out[last].end {
			out[last].end = out[i].start
		}
		if out[i].end <= out[i].start {
			out[i].end = math.Min(duration, out[i].start+0.2)
		}
		last = i
	}
}

// fillLineTimes maps matched ASR characters back to the printed lyric runes.
// Missing characters (ASR errors) are interpolated, so the karaoke overlay can
// follow the actual phrase boundaries instead of dividing a whole line evenly.
func fillLineTimes(lines []Line, out []span, chart []chartChar, pairs []charPair, asr []asrChar) {
	assigned := make([][]float64, len(lines))
	for i, line := range lines {
		assigned[i] = make([]float64, len([]rune(line.Lyrics)))
		for j := range assigned[i] {
			assigned[i][j] = -1
		}
	}
	for _, pair := range pairs {
		item := chart[pair.chart]
		if item.pos >= 0 && item.pos < len(assigned[item.line]) {
			assigned[item.line][item.pos] = asr[pair.asr].start
		}
	}
	for i, line := range lines {
		if out[i].start < 0 || len(assigned[i]) == 0 {
			continue
		}
		out[i].chars = interpolateTimes(line.Lyrics, out[i], assigned[i])
	}
}

func interpolateTimes(text string, sp span, assigned []float64) []float64 {
	runes := []rune(text)
	times := make([]float64, len(runes))
	for i := range times {
		times[i] = -1
	}
	for i, value := range assigned {
		if i >= len(times) || value < 0 {
			continue
		}
		times[i] = clamp(value, sp.start, sp.end)
	}
	prev, next := -1, -1
	for i := range runes {
		if times[i] >= 0 {
			prev = i
			continue
		}
		if !unicode.IsLetter(runes[i]) && !unicode.IsDigit(runes[i]) {
			if prev >= 0 {
				times[i] = times[prev]
			}
			continue
		}
		next = -1
		for j := i + 1; j < len(runes); j++ {
			if times[j] >= 0 {
				next = j
				break
			}
		}
		switch {
		case prev >= 0 && next >= 0:
			times[i] = times[prev] + (times[next]-times[prev])*float64(i-prev)/float64(next-prev)
		case prev >= 0:
			times[i] = times[prev] + (sp.end-times[prev])/float64(max(1, len(runes)-1-prev))
		case next >= 0:
			times[i] = sp.start + (times[next]-sp.start)*float64(i)/float64(next)
		default:
			times[i] = sp.start
		}
	}
	for i := 1; i < len(times); i++ {
		if times[i] < times[i-1] {
			times[i] = times[i-1]
		}
	}
	if len(times) > 0 && times[0] < 0 {
		times[0] = sp.start
		for i := 1; i < len(times); i++ {
			if times[i] < 0 {
				times[i] = times[i-1]
			}
		}
	}
	return times
}

func spreadSpans(lines []Line, duration float64) []span {
	out := make([]span, len(lines))
	if len(lines) == 0 {
		return out
	}
	step := duration / float64(len(lines))
	for i := range out {
		out[i] = span{start: round(step * float64(i)), end: round(step * float64(i+1))}
	}
	return out
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
