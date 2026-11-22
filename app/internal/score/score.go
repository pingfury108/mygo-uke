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
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Label    string  `json:"label"`
	Conf     float64 `json:"conf"`
	AltLabel string  `json:"altLabel,omitempty"`
	Approx   bool    `json:"approx,omitempty"`
}

// Bar is one downbeat-delimited measure. Chords holds one label, or two
// when the chord changes late in the bar.
type Bar struct {
	I           int      `json:"i"`
	Start       float64  `json:"start"`
	End         float64  `json:"end"`
	Chords      []string `json:"chords"`
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
