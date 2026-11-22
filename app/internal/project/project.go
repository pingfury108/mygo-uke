// Package project owns project.json: the user's edits, the analysis
// result, and the cached timing data that the UI and renderer share.
package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"mygo-uke/internal/score"
)

const Version = 1

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
	Pattern       string               `json:"pattern"`
	Style         Style                `json:"style"`
}

// Line is one sentence of lyrics, with the time span ASR gave it.
type Line struct {
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Text        string  `json:"text"`
	Conf        float64 `json:"conf,omitempty"`
	NeedsReview bool    `json:"needsReview,omitempty"`
}

// Style is the subset of overlay styling the P0 editor exposes.
type Style struct {
	ChordSize float64 `json:"chordSize"`
	SubSize   float64 `json:"subSize"`
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
			Accent:    "#F3B544",
		},
	}
}

// RebuildBars refreshes bars from the cached downbeats and chords.
func (p *Project) RebuildBars() {
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
	return &p, nil
}
