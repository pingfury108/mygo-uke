// Package score turns raw chord segments and downbeats into bars. It has no
// UI dependency, so the rules that drive the timeline are unit-testable.
package score

import (
	"fmt"
	"math"
	"sort"
)

// ChordSegment is one chord heard over time, in seconds.
type ChordSegment struct {
	Start      float64  `json:"start"`
	End        float64  `json:"end"`
	Label      string   `json:"label"`
	Conf       float64  `json:"conf"`
	AltLabel   string   `json:"altLabel,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	Approx     bool     `json:"approx,omitempty"`
}

// Bar is one downbeat-delimited measure. Chords holds one label, or two
// when the chord changes late in the bar.
type Bar struct {
	I           int      `json:"i"`
	Start       float64  `json:"start"`
	End         float64  `json:"end"`
	Chords      []string `json:"chords"`
	Candidates  []string `json:"candidates,omitempty"`
	Conf        float64  `json:"conf"`
	NeedsReview bool     `json:"needsReview,omitempty"`
	Manual      bool     `json:"manual,omitempty"`
	AltLabel    string   `json:"altLabel,omitempty"`
	Approx      bool     `json:"approx,omitempty"`
}

// BuildBars slices the song at downbeats and votes, by overlap, for the
// chord of each bar. A chord that starts after beat three and lasts at
// least one beat becomes the bar's second chord.
func BuildBars(downbeats []float64, chords []ChordSegment, duration float64) []Bar {
	db := cleanTimes(downbeats, duration)
	if len(db) == 0 {
		if duration <= 0 {
			return nil
		}
		db = []float64{0}
	}
	segs := cleanSegments(chords, duration)
	bars := make([]Bar, 0, len(db))
	for i, start := range db {
		end := duration
		if i+1 < len(db) {
			end = db[i+1]
		}
		if end <= start {
			continue
		}
		bar := voteBar(i, start, end, segs)
		bars = append(bars, bar)
	}
	for i := range bars {
		bars[i].I = i
	}
	return bars
}

// StabilizeDownbeats repairs a downbeat track that collapsed into
// near-beat clusters, which happens on intros with no clear pulse. It only
// rebuilds the grid when most downbeats already agree on one bar length and
// enough of the track disagrees with it; a healthy track is returned as is.
func StabilizeDownbeats(beats, downbeats []float64, duration float64) []float64 {
	if len(beats) < 8 || len(downbeats) < 4 {
		return downbeats
	}
	idx := make([]int, 0, len(downbeats))
	for _, downbeat := range downbeats {
		i := nearestBeat(beats, downbeat)
		if len(idx) == 0 || idx[len(idx)-1] != i {
			idx = append(idx, i)
		}
	}
	if len(idx) < 4 {
		return downbeats
	}

	histogram := map[int]int{}
	for i := 1; i < len(idx); i++ {
		if gap := idx[i] - idx[i-1]; gap >= 2 && gap <= 8 {
			histogram[gap]++
		}
	}
	barBeats, votes := 0, 0
	for gap, count := range histogram {
		if count > votes || (count == votes && gap < barBeats) {
			barBeats, votes = gap, count
		}
	}
	total := len(idx) - 1
	if barBeats == 0 || votes*2 < total {
		return downbeats
	}
	abnormal := 0
	for i := 1; i < len(idx); i++ {
		if idx[i]-idx[i-1] != barBeats {
			abnormal++
		}
	}
	if abnormal*4 <= total {
		return downbeats
	}

	phase, hits := 0, -1
	for candidate := 0; candidate < barBeats && candidate < len(beats); candidate++ {
		count := 0
		for i := candidate; i < len(beats); i += barBeats {
			for _, downbeat := range downbeats {
				if math.Abs(beats[i]-downbeat) <= 0.12 {
					count++
					break
				}
			}
		}
		if count > hits {
			phase, hits = candidate, count
		}
	}

	grid := make([]float64, 0, len(beats)/barBeats+1)
	for i := phase; i < len(beats); i += barBeats {
		grid = append(grid, beats[i])
	}
	if len(grid) < 3 {
		return downbeats
	}
	if duration > 0 {
		gaps := make([]float64, 0, len(beats)-1)
		for i := 1; i < len(beats); i++ {
			if gap := beats[i] - beats[i-1]; gap > 0.05 {
				gaps = append(gaps, gap)
			}
		}
		if len(gaps) > 0 {
			sort.Float64s(gaps)
			expected := duration / (float64(barBeats) * gaps[len(gaps)/2])
			if expected > 0 && math.Abs(float64(len(grid))-expected) > expected*0.4 {
				return downbeats
			}
		}
	}
	return grid
}

// SnapToBeats moves chord edges that sit close to a beat onto that beat.
// Edges farther than tolerance keep their original timing.
func SnapToBeats(beats []float64, chords []ChordSegment, tolerance float64) []ChordSegment {
	if len(beats) < 2 || tolerance <= 0 {
		return chords
	}
	out := make([]ChordSegment, len(chords))
	copy(out, chords)
	for i := range out {
		start := snapBeat(beats, out[i].Start, tolerance)
		end := snapBeat(beats, out[i].End, tolerance)
		if end > start {
			out[i].Start = start
			out[i].End = end
		}
	}
	return out
}

func snapBeat(beats []float64, t, tolerance float64) float64 {
	best, distance := t, tolerance
	for _, beat := range beats {
		if diff := math.Abs(beat - t); diff < distance {
			best, distance = beat, diff
		}
	}
	return best
}

func nearestBeat(beats []float64, t float64) int {
	best := 0
	for i := 1; i < len(beats); i++ {
		if math.Abs(beats[i]-t) < math.Abs(beats[best]-t) {
			best = i
		}
	}
	return best
}

// BarAt returns the bar holding t. Before the first downbeat it returns
// the first bar; after the last it returns the last.
func BarAt(bars []Bar, t float64) (Bar, int, bool) {
	if len(bars) == 0 {
		return Bar{}, -1, false
	}
	if t <= bars[0].Start {
		return bars[0], 0, true
	}
	for i := range bars {
		if t < bars[i].End {
			return bars[i], i, true
		}
	}
	last := len(bars) - 1
	return bars[last], last, true
}

// BarIndexAt is BarAt without copying the bar.
func BarIndexAt(bars []Bar, t float64) int {
	_, i, ok := BarAt(bars, t)
	if !ok {
		return -1
	}
	return i
}

// ReviewCount counts bars the user should look at.
func ReviewCount(bars []Bar) int {
	n := 0
	for _, b := range bars {
		if b.NeedsReview {
			n++
		}
	}
	return n
}

// Ruler returns up to n bars around the bar holding t, for the transport
// ruler. It keeps the result in timeline order.
func Ruler(bars []Bar, t float64, n int) []Bar {
	if n <= 0 || len(bars) == 0 {
		return nil
	}
	center := BarIndexAt(bars, t)
	if center < 0 {
		center = 0
	}
	start := center - n/2
	if start < 0 {
		start = 0
	}
	if start+n > len(bars) {
		start = len(bars) - n
	}
	if start < 0 {
		start = 0
	}
	end := start + n
	if end > len(bars) {
		end = len(bars)
	}
	return bars[start:end]
}

// TimeText formats seconds as mm:ss.
func TimeText(seconds float64) string {
	if seconds < 0 || math.IsNaN(seconds) {
		seconds = 0
	}
	s := int(math.Round(seconds))
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

func voteBar(i int, start, end float64, segs []ChordSegment) Bar {
	bar := Bar{I: i, Start: start, End: end, Conf: 1}
	span := end - start
	beat := span / 4
	if beat <= 0 {
		beat = span
	}
	weights := map[string]float64{}
	confSum := map[string]float64{}
	candWeight := map[string]float64{}
	alt := map[string]string{}
	approx := map[string]bool{}
	firstStart := map[string]float64{}
	lastEnd := map[string]float64{}
	for _, s := range segs {
		overlap := math.Min(end, s.End) - math.Max(start, s.Start)
		if overlap <= 0 {
			continue
		}
		weights[s.Label] += overlap
		confSum[s.Label] += overlap * s.Conf
		for _, candidate := range s.Candidates {
			if candidate != "" && candidate != s.Label {
				candWeight[candidate] += overlap
			}
		}
		if s.AltLabel != "" && alt[s.Label] == "" {
			alt[s.Label] = s.AltLabel
		}
		approx[s.Label] = approx[s.Label] || s.Approx
		if v, ok := firstStart[s.Label]; !ok || s.Start < v {
			firstStart[s.Label] = s.Start
		}
		if s.End > lastEnd[s.Label] {
			lastEnd[s.Label] = s.End
		}
	}
	if len(weights) == 0 {
		bar.Chords = nil
		bar.Conf = 0
		bar.NeedsReview = true
		return bar
	}
	primary, primaryWeight := "", 0.0
	for label, w := range weights {
		if w > primaryWeight || (w == primaryWeight && label < primary) {
			primary, primaryWeight = label, w
		}
	}
	bar.Chords = []string{primary}
	bar.Conf = confSum[primary] / primaryWeight
	bar.AltLabel = alt[primary]
	bar.Approx = approx[primary]

	// A late change that lasts at least a beat is a real second chord.
	for label, w := range weights {
		if label == primary || w < beat {
			continue
		}
		if firstStart[label] >= start+3*beat-1e-6 && lastEnd[label]-firstStart[label] >= beat-1e-6 {
			bar.Chords = append(bar.Chords, label)
			conf := confSum[label] / w
			if conf < bar.Conf {
				bar.Conf = conf
			}
			if bar.AltLabel == "" {
				bar.AltLabel = alt[label]
			}
			bar.Approx = bar.Approx || approx[label]
		}
	}
	if len(bar.Chords) > 1 {
		sort.SliceStable(bar.Chords[1:], func(a, b int) bool {
			return firstStart[bar.Chords[a+1]] < firstStart[bar.Chords[b+1]]
		})
	}
	type candidate struct {
		label  string
		weight float64
	}
	var candidates []candidate
	for label, weight := range candWeight {
		if weight < beat*0.5 {
			continue
		}
		used := false
		for _, chord := range bar.Chords {
			if chord == label {
				used = true
				break
			}
		}
		if !used {
			candidates = append(candidates, candidate{label: label, weight: weight})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].weight > candidates[j].weight
	})
	for i := 0; i < len(candidates) && i < 3; i++ {
		bar.Candidates = append(bar.Candidates, candidates[i].label)
	}
	if bar.Conf < 0.6 {
		bar.NeedsReview = true
	}
	return bar
}

func cleanTimes(in []float64, duration float64) []float64 {
	out := make([]float64, 0, len(in))
	for _, t := range in {
		if math.IsNaN(t) || math.IsInf(t, 0) {
			continue
		}
		if t < 0 {
			t = 0
		}
		if duration > 0 && t >= duration {
			continue
		}
		out = append(out, t)
	}
	sort.Float64s(out)
	if len(out) == 0 {
		return nil
	}
	dedup := out[:1]
	for _, t := range out[1:] {
		if t-dedup[len(dedup)-1] > 0.02 {
			dedup = append(dedup, t)
		}
	}
	return dedup
}

func cleanSegments(in []ChordSegment, duration float64) []ChordSegment {
	out := make([]ChordSegment, 0, len(in))
	for _, s := range in {
		if s.Label == "" || math.IsNaN(s.Start) || math.IsNaN(s.End) {
			continue
		}
		if s.Start < 0 {
			s.Start = 0
		}
		if duration > 0 && s.Start > duration {
			continue
		}
		if s.End <= s.Start {
			continue
		}
		if duration > 0 && s.End > duration {
			s.End = duration
		}
		if s.Conf <= 0 {
			s.Conf = 0.5
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}
