// Package project owns project.json: the user's edits, the analysis
// result, and the cached timing data that the UI and renderer share.
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"mygo-uke/internal/score"
)

const Version = 1

// Entry is one project found in the local library.
type Entry struct {
	Path     string
	Project  *Project
	Modified time.Time
}

// Project is the single source of truth for one song.
type Project struct {
	Version       int                  `json:"version"`
	Title         string               `json:"title"`
	Source        string               `json:"source"`
	SourceHash    string               `json:"sourceHash"`
	Duration      float64              `json:"duration"`
	Tempo         float64              `json:"tempo"`
	TimeSignature [2]int               `json:"timeSignature"`
	Key           string               `json:"key"`
	Capo          int                  `json:"capo"`
	Tuning        string               `json:"tuning"`
	Beats         []float64            `json:"beats"`
	Downbeats     []float64            `json:"downbeats"`
	Chords        []score.ChordSegment `json:"chords"`
	Bars          []score.Bar          `json:"bars"`
	Lines         []Line               `json:"lines"`
	LinesEdited   bool                 `json:"linesEdited,omitempty"`
	ChartImage    string               `json:"chartImage,omitempty"`
	ChartName     string               `json:"chartName,omitempty"`
	ChartText     string               `json:"chartText,omitempty"`
	ChartAt       string               `json:"chartAt,omitempty"`
	Warnings      []string             `json:"warnings,omitempty"`
	Pattern       string               `json:"pattern"`
	Style         Style                `json:"style"`
}

// Line is one sentence of lyrics placed on the song timeline.
type Line struct {
	Start       float64     `json:"start"`
	End         float64     `json:"end"`
	Text        string      `json:"text"`
	CharTimes   []float64   `json:"charTimes,omitempty"`
	Conf        float64     `json:"conf,omitempty"`
	NeedsReview bool        `json:"needsReview,omitempty"`
	Chords      []ChordMark `json:"chords,omitempty"`
}

// ChordMark is the point in a lyric line where a chord starts. The chord
// lasts until the next mark (or the end of the last line), so several
// characters can sit under one chord.
type ChordMark struct {
	At     int    `json:"at"`
	Label  string `json:"label"`
	Approx bool   `json:"approx,omitempty"`
}

// Style is the subset of overlay styling the P0 editor exposes.
type Style struct {
	ChordSize float64 `json:"chordSize"`
	SubSize   float64 `json:"subSize"`
	SubtitleY float64 `json:"subtitleY,omitempty"`
	Accent    string  `json:"accent"`
	ShowSafe  bool    `json:"showSafe"`
}

// New returns a project with defaults filled in.
func New(title string) *Project {
	return &Project{
		Version:       Version,
		Title:         title,
		TimeSignature: [2]int{4, 4},
		Tuning:        "GCEA",
		Pattern:       "folk-basic",
		Style: Style{
			ChordSize: 64,
			SubSize:   60,
			SubtitleY: 78,
			Accent:    "#F3B544",
		},
	}
}

// RebuildBars refreshes bars from the cached downbeats and chords.
func (p *Project) RebuildBars() {
	p.Chords = score.SnapToBeats(p.Beats, p.Chords, 0.15)
	p.Downbeats = score.StabilizeDownbeats(p.Beats, p.Downbeats, p.Duration)
	p.Bars = score.BuildBars(p.Downbeats, p.Chords, p.Duration)
}

// ReviewCount counts bars and lines that still need the user's attention.
func (p *Project) ReviewCount() int {
	n := score.ReviewCount(p.Bars)
	for _, line := range p.Lines {
		if line.NeedsReview {
			n++
		}
	}
	return n
}

// SetLine updates the line at index, or appends when index is out of range,
// then keeps the list in timeline order. It returns the line's new index.
func (p *Project) SetLine(index int, line Line) int {
	if index >= 0 && index < len(p.Lines) {
		p.Lines[index] = line
	} else {
		p.Lines = append(p.Lines, line)
	}
	sort.SliceStable(p.Lines, func(i, j int) bool {
		return p.Lines[i].Start < p.Lines[j].Start
	})
	for i, item := range p.Lines {
		if item.Start == line.Start && item.End == line.End && item.Text == line.Text {
			return i
		}
	}
	return 0
}

// DeleteLine removes one line and reports whether it existed.
func (p *Project) DeleteLine(index int) bool {
	if index < 0 || index >= len(p.Lines) {
		return false
	}
	p.Lines = append(p.Lines[:index], p.Lines[index+1:]...)
	return true
}

// Save writes the project atomically, so a crash cannot leave half a file.
func Save(path string, p *Project) error {
	if p == nil {
		return errors.New("project is nil")
	}
	if p.Version == 0 {
		p.Version = Version
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load reads a project.json.
func Load(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.Version == 0 {
		p.Version = Version
	}
	if p.Tuning == "" {
		p.Tuning = "GCEA"
	}
	if p.Pattern == "" {
		p.Pattern = "folk-basic"
	}
	if p.Style.ChordSize <= 0 {
		p.Style.ChordSize = 64
	}
	if p.Style.SubSize <= 0 {
		p.Style.SubSize = 60
	}
	if p.Style.SubtitleY == 0 {
		p.Style.SubtitleY = 78
	}
	if p.Style.Accent == "" {
		p.Style.Accent = "#F3B544"
	}
	return &p, nil
}

// Merge keeps the user's edits on top of a fresh analysis. Chords the user
// picked survive by overlapping bar; subtitle lines survive as a set once
// the user edited or deleted any of them.
func Merge(edited, built *Project) *Project {
	if edited == nil || built == nil {
		return built
	}
	built.Title = edited.Title
	built.Capo = edited.Capo
	built.Pattern = edited.Pattern
	built.Style = edited.Style
	built.ChartImage = edited.ChartImage
	built.ChartName = edited.ChartName
	built.ChartText = edited.ChartText
	built.ChartAt = edited.ChartAt
	if edited.LinesEdited {
		built.Lines = append([]Line(nil), edited.Lines...)
		built.LinesEdited = true
	}
	hasChart := false
	for _, segment := range edited.Chords {
		if segment.Source == "chart" {
			hasChart = true
			break
		}
	}
	for _, line := range edited.Lines {
		if len(line.Chords) > 0 {
			hasChart = true
			break
		}
	}
	if hasChart {
		built.Chords = nil
		for _, segment := range edited.Chords {
			if segment.Source == "chart" {
				built.Chords = append(built.Chords, segment)
			}
		}
		built.RebuildChartChords()
	}
	for i := range built.Bars {
		for _, bar := range edited.Bars {
			if !bar.Manual || !overlaps(bar, built.Bars[i]) {
				continue
			}
			built.Bars[i].Chords = append([]string(nil), bar.Chords...)
			built.Bars[i].Manual = true
			built.Bars[i].NeedsReview = false
			built.Bars[i].AltLabel = ""
			built.Bars[i].Approx = false
			break
		}
	}
	return built
}

func overlaps(a, b score.Bar) bool {
	return a.Start < b.End && b.Start < a.End
}

// Delete removes one project directory. It only accepts a project.json path,
// so a bad path cannot take an unrelated directory with it.
func Delete(path string) error {
	if filepath.Base(path) != "project.json" {
		return errors.New("project path must end in project.json")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Dir(path))
}

// Root is the local project library, overridable for tests and portable use.
func Root() string {
	if value := os.Getenv("MYGO_UKE_PROJECTS"); value != "" {
		return value
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "mygo-uke", "projects")
}

// NewDir returns a fresh project directory under root.
func NewDir(root, title string) string {
	slug := slugify(title)
	if slug == "" {
		slug = "project"
	}
	return filepath.Join(root, fmt.Sprintf("%s-%d", slug, time.Now().UnixNano()))
}

// List returns projects newest first. A missing library is empty, not an error.
func List(root string) ([]Entry, error) {
	dirs, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		path := filepath.Join(root, dir.Name(), "project.json")
		loaded, err := Load(path)
		if err != nil {
			continue
		}
		info, err := dir.Info()
		if err != nil {
			continue
		}
		entries = append(entries, Entry{Path: path, Project: loaded, Modified: info.ModTime()})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Modified.After(entries[j].Modified)
	})
	return entries, nil
}

func slugify(title string) string {
	var out strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			out.WriteRune(r)
			lastDash = false
		case !lastDash:
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}
