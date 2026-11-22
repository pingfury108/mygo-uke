// Package subtitle writes sentence-level SRT files.
package subtitle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cue is one subtitle line with seconds-based timing.
type Cue struct {
	Start float64
	End   float64
	Text  string
}

// Format returns an SRT document. Cues with an empty text or a non-positive
// span are skipped, so a half-edited project still exports cleanly.
func Format(cues []Cue) string {
	var out strings.Builder
	n := 0
	for _, cue := range cues {
		text := strings.TrimSpace(cue.Text)
		if text == "" || cue.End <= cue.Start {
			continue
		}
		start := max(0, cue.Start)
		end := max(start+0.001, cue.End)
		n++
		fmt.Fprintf(&out, "%d\n%s --> %s\n%s\n\n",
			n, stamp(start), stamp(end), text)
	}
	return out.String()
}

// Write writes an SRT file, creating its directory when needed.
func Write(path string, cues []Cue) error {
	body := Format(cues)
	if body == "" {
		return os.WriteFile(path, nil, 0o600)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o600)
}

func stamp(seconds float64) string {
	total := int64(seconds*1000 + 0.5)
	ms := total % 1000
	total /= 1000
	s := total % 60
	total /= 60
	m := total % 60
	h := total / 60
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
