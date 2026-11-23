// Package uke is the only instrument-specific code in mygo-uke: a fixed
// table of standard GCEA ukulele chord shapes, plus normalization from the
// chord labels found in imported charts.
package uke

import (
	"strings"
	"unicode"
)

// Chord is a chord label reduced to a root pitch class and a quality.
// Bass is -1 for a plain chord, and the slash bass otherwise.
type Chord struct {
	Root    int
	Quality string
	Bass    int
	Approx  bool
	Raw     string
}

// Fingering is one way to play a chord. Frets go from the 4th string (G) to
// the 1st (A): -1 is muted, 0 is open, n is fret n.
type Fingering struct {
	Frets  [4]int
	Base   int
	Name   string
	Barre  bool
	Approx bool
}

var pitchNames = [12]string{
	"C", "Db", "D", "Eb", "E", "F", "Gb", "G", "Ab", "A", "Bb", "B",
}

// Name returns the normalized chord label, without a slash bass.
func (c Chord) Name() string { return pitchNames[c.Root] + c.Quality }

// Normalize turns a chart chord label into a ukulele-friendly chord.
// Slash chords keep their bass for display but are played as the root
// chord; extended chords fall back to the closest shape.
func Normalize(label string) (Chord, bool) {
	raw := strings.TrimSpace(label)
	raw = strings.NewReplacer("♯", "#", "♭", "b", "＃", "#").Replace(raw)
	if raw == "" || raw == "-" ||
		strings.EqualFold(raw, "N") || strings.EqualFold(raw, "X") ||
		strings.EqualFold(raw, "N.C.") || strings.EqualFold(raw, "NC") {
		return Chord{}, false
	}
	if i := strings.IndexByte(raw, '/'); i > 0 {
		root, quality, approx, ok := parseChord(raw[:i])
		if !ok {
			return Chord{}, false
		}
		bass := -1
		if b, ok := parseRoot(raw[i+1:]); ok {
			bass = b
		}
		return Chord{Root: root, Quality: quality, Bass: bass, Approx: approx, Raw: raw}, true
	}
	root, quality, approx, ok := parseChord(raw)
	if !ok {
		return Chord{}, false
	}
	return Chord{Root: root, Quality: quality, Bass: -1, Approx: approx, Raw: raw}, true
}

func parseChord(s string) (root int, quality string, approx bool, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", false, false
	}
	root, ok = parseRoot(s)
	if !ok {
		return 0, "", false, false
	}
	i := 1
	for i < len(s) && (s[i] == '#' || s[i] == 'b') {
		i++
	}
	rest := strings.TrimSpace(s[i:])
	rest = strings.TrimSpace(strings.TrimPrefix(rest, ":"))
	rest = stripParentheses(rest)
	quality, approx = normalizeQuality(rest)
	return root, quality, approx, true
}

func parseRoot(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	letter := unicode.ToUpper(rune(s[0]))
	base := -1
	switch letter {
	case 'C':
		base = 0
	case 'D':
		base = 2
	case 'E':
		base = 4
	case 'F':
		base = 5
	case 'G':
		base = 7
	case 'A':
		base = 9
	case 'B':
		base = 11
	default:
		return 0, false
	}
	i := 1
	for i < len(s) && (s[i] == '#' || s[i] == 'b') {
		if s[i] == '#' {
			base++
		} else {
			base--
		}
		i++
	}
	return (base + 120) % 12, true
}

func stripParentheses(s string) string {
	if i := strings.IndexByte(s, '('); i >= 0 {
		return s[:i]
	}
	return s
}

func normalizeQuality(q string) (string, bool) {
	if q == "" {
		return "", false
	}
	if q == "M" || q == "Δ" {
		return "", false
	}
	if q == "Δ7" || q == "M7" {
		return "maj7", false
	}
	lower := strings.ToLower(strings.ReplaceAll(q, " ", ""))
	lower = strings.NewReplacer("△", "maj", "δ", "maj").Replace(lower)
	lower = strings.ReplaceAll(lower, "°", "dim")
	switch lower {
	case "m", "min", "minor", "-":
		return "m", false
	case "maj", "major":
		return "", false
	case "7", "dom7":
		return "7", false
	case "9", "11", "13":
		return "7", true
	case "6":
		return "6", false
	case "m6", "min6":
		return "m6", false
	case "maj7", "major7", "m7maj":
		return "maj7", false
	case "maj9", "maj11", "maj13":
		return "maj7", true
	case "m7", "min7", "minor7", "-7":
		return "m7", false
	case "m9", "m11", "m13", "min9":
		return "m7", true
	case "sus", "sus4":
		return "sus4", false
	case "sus2":
		return "sus2", false
	case "dim", "dim7", "o7", "m7b5", "ø7":
		return "dim", lower != "dim"
	case "aug", "+", "aug7", "+7":
		return "aug", lower != "aug"
	case "add9", "add2":
		return "add9", false
	case "5":
		return "", true
	}
	if strings.HasPrefix(lower, "maj") {
		if hasAny(lower, "7", "9", "11", "13") {
			return "maj7", !hasAny(lower, "maj7")
		}
		return "", true
	}
	if strings.HasPrefix(lower, "min") || strings.HasPrefix(lower, "m") {
		switch {
		case strings.HasPrefix(lower, "m6"):
			return "m6", lower != "m6"
		case strings.HasPrefix(lower, "maj"):
			return "maj7", true
		case strings.HasPrefix(lower, "m7"), strings.HasPrefix(lower, "m9"),
			strings.HasPrefix(lower, "m11"), strings.HasPrefix(lower, "m13"):
			return "m7", lower != "m7"
		default:
			return "m", true
		}
	}
	if strings.HasPrefix(lower, "sus") {
		if strings.Contains(lower, "2") {
			return "sus2", true
		}
		return "sus4", true
	}
	if strings.HasPrefix(lower, "dim") {
		return "dim", true
	}
	if strings.HasPrefix(lower, "aug") {
		return "aug", true
	}
	if strings.HasPrefix(lower, "add") {
		return "add9", true
	}
	return "", true
}

func hasAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// Shape returns the easiest shape for c when played with capo. c stores
// the sounding pitch, so the shape root is the sounding root minus capo.
func Shape(c Chord, capo int) (Fingering, bool) {
	if capo < 0 {
		capo = 0
	}
	shapeRoot := (c.Root - capo + 120) % 12
	frets, used, ok := lookup(shapeRoot, c.Quality)
	if !ok {
		return Fingering{}, false
	}
	s := makeShape(frets)
	s.Name = pitchNames[shapeRoot] + used
	s.Approx = c.Approx || used != c.Quality
	return s, true
}

// Available reports whether the chord can be played in standard GCEA.
func Available(c Chord) bool {
	_, _, ok := lookup(c.Root, c.Quality)
	return ok
}

func makeShape(frets [4]int) Fingering {
	s := Fingering{Frets: frets, Base: 1}
	low := 0
	for _, f := range frets {
		if f > 0 && (low == 0 || f < low) {
			low = f
		}
	}
	if low > 4 {
		s.Base = low
	}
	if frets[0] > 0 && frets[0] == frets[1] && frets[1] == frets[2] && frets[2] == frets[3] {
		s.Barre = true
	}
	return s
}

type shapeRow map[string][4]int

func lookup(root int, quality string) ([4]int, string, bool) {
	for _, q := range fallbacks(quality) {
		if f, ok := shapes[root][q]; ok {
			return f, q, true
		}
	}
	return [4]int{}, "", false
}

func fallbacks(q string) []string {
	switch q {
	case "maj7":
		return []string{"maj7", "", "7"}
	case "m7":
		return []string{"m7", "m", ""}
	case "7":
		return []string{"7", ""}
	case "6":
		return []string{"6", "maj7", ""}
	case "m6":
		return []string{"m6", "m7", "m"}
	case "sus2":
		return []string{"sus2", "sus4", ""}
	case "sus4":
		return []string{"sus4", ""}
	case "dim":
		return []string{"dim", "m"}
	case "aug":
		return []string{"aug", ""}
	case "add9":
		return []string{"add9", "", "maj7"}
	case "m":
		return []string{"m", "m7"}
	default:
		return []string{"", "maj7", "7"}
	}
}

var shapes = map[int]shapeRow{
	0: { // C
		"":     {0, 0, 0, 3},
		"m":    {0, 3, 3, 3},
		"7":    {0, 0, 0, 1},
		"maj7": {0, 0, 0, 2},
		"m7":   {3, 3, 3, 3},
		"sus4": {0, 0, 1, 3},
		"sus2": {0, 2, 3, 3},
		"dim":  {2, 3, 2, 3},
		"aug":  {1, 0, 0, 3},
		"6":    {0, 0, 0, 0},
		"m6":   {2, 3, 3, 3},
		"add9": {0, 2, 0, 3},
	},
	1: { // Db
		"":     {1, 1, 1, 4},
		"m":    {1, 4, 4, 4},
		"7":    {1, 1, 1, 2},
		"maj7": {1, 1, 1, 3},
		"m7":   {4, 4, 4, 4},
		"sus4": {1, 1, 2, 4},
		"sus2": {1, 3, 1, 4},
		"dim":  {3, 4, 3, 4},
		"aug":  {2, 1, 1, 4},
		"6":    {1, 1, 1, 1},
		"m6":   {3, 4, 4, 4},
		"add9": {1, 3, 1, 4},
	},
	2: { // D
		"":     {2, 2, 2, 0},
		"m":    {2, 2, 1, 0},
		"7":    {2, 0, 2, 0},
		"maj7": {2, 2, 2, 4},
		"m7":   {2, 2, 1, 3},
		"sus4": {0, 2, 3, 0},
		"sus2": {2, 2, 0, 0},
		"dim":  {1, 2, 1, 2},
		"aug":  {3, 2, 2, 0},
		"6":    {2, 2, 2, 2},
		"m6":   {2, 2, 1, 0},
		"add9": {2, 2, 0, 0},
	},
	3: { // Eb
		"":     {3, 3, 3, 1},
		"m":    {3, 3, 2, 1},
		"7":    {3, 3, 3, 4},
		"maj7": {3, 3, 3, 5},
		"m7":   {3, 3, 2, 4},
		"sus4": {1, 3, 4, 1},
		"dim":  {2, 3, 2, 3},
		"aug":  {4, 3, 3, 1},
		"6":    {3, 3, 3, 3},
		"m6":   {3, 3, 2, 1},
	},
	4: { // E
		"":     {4, 4, 4, 2},
		"m":    {0, 4, 3, 2},
		"7":    {1, 2, 0, 2},
		"maj7": {1, 3, 0, 2},
		"m7":   {0, 2, 0, 2},
		"sus4": {2, 4, 0, 2},
		"dim":  {0, 1, 0, 1},
		"aug":  {1, 0, 0, 3},
		"6":    {1, 1, 0, 2},
		"m6":   {0, 1, 0, 2},
	},
	5: { // F
		"":     {2, 0, 1, 0},
		"m":    {1, 0, 1, 3},
		"7":    {2, 3, 1, 3},
		"maj7": {2, 4, 1, 3},
		"m7":   {1, 3, 1, 3},
		"sus4": {3, 0, 1, 1},
		"sus2": {3, 0, 1, 3},
		"dim":  {1, 2, 1, 2},
		"aug":  {2, 1, 1, 4},
		"6":    {2, 2, 1, 3},
		"m6":   {1, 3, 1, 3},
		"add9": {0, 0, 1, 3},
	},
	6: { // Gb
		"":     {3, 1, 2, 1},
		"m":    {2, 1, 2, 0},
		"7":    {3, 4, 2, 4},
		"maj7": {3, 5, 2, 4},
		"m7":   {2, 4, 2, 4},
		"sus4": {4, 1, 2, 2},
		"dim":  {2, 3, 2, 3},
		"aug":  {3, 2, 2, 0},
		"6":    {3, 3, 2, 4},
		"m6":   {2, 4, 2, 4},
	},
	7: { // G
		"":     {0, 2, 3, 2},
		"m":    {0, 2, 3, 1},
		"7":    {0, 2, 1, 2},
		"maj7": {0, 2, 2, 2},
		"m7":   {0, 2, 1, 1},
		"sus4": {0, 2, 3, 3},
		"sus2": {0, 2, 3, 0},
		"dim":  {0, 1, 0, 1},
		"aug":  {4, 3, 3, 1},
		"6":    {0, 2, 0, 2},
		"m6":   {0, 2, 3, 1},
	},
	8: { // Ab
		"":     {1, 3, 4, 3},
		"m":    {1, 3, 4, 2},
		"7":    {1, 3, 2, 3},
		"maj7": {1, 3, 3, 3},
		"m7":   {1, 3, 2, 2},
		"sus4": {1, 3, 4, 4},
		"dim":  {1, 2, 1, 2},
		"aug":  {1, 0, 0, 3},
		"6":    {1, 3, 1, 3},
		"m6":   {1, 3, 4, 2},
	},
	9: { // A
		"":     {2, 1, 0, 0},
		"m":    {2, 0, 0, 0},
		"7":    {0, 1, 0, 0},
		"maj7": {1, 1, 0, 0},
		"m7":   {0, 0, 0, 0},
		"sus4": {2, 2, 0, 0},
		"sus2": {2, 4, 0, 2},
		"dim":  {2, 3, 2, 3},
		"aug":  {2, 1, 1, 4},
		"6":    {2, 1, 2, 0},
		"m6":   {2, 0, 2, 0},
		"add9": {2, 1, 0, 2},
	},
	10: { // Bb
		"":     {3, 2, 1, 1},
		"m":    {3, 1, 1, 1},
		"7":    {1, 2, 1, 1},
		"maj7": {3, 2, 1, 0},
		"m7":   {1, 1, 1, 1},
		"sus4": {3, 3, 1, 1},
		"sus2": {3, 3, 1, 1},
		"dim":  {3, 4, 3, 4},
		"aug":  {3, 2, 2, 0},
		"6":    {3, 2, 3, 1},
		"m6":   {3, 1, 3, 1},
	},
	11: { // B
		"":     {4, 3, 2, 2},
		"m":    {4, 2, 2, 2},
		"7":    {2, 3, 2, 2},
		"maj7": {3, 3, 2, 2},
		"m7":   {2, 2, 2, 2},
		"sus4": {4, 4, 2, 2},
		"sus2": {4, 4, 2, 2},
		"dim":  {1, 2, 1, 2},
		"aug":  {4, 3, 3, 1},
		"6":    {4, 3, 4, 2},
		"m6":   {4, 2, 4, 2},
		"add9": {4, 3, 2, 2},
	},
}
