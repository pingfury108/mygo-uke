package export

import (
	"path/filepath"
	"strings"

	"mygo-uke/internal/project"
	"mygo-uke/internal/subtitle"
)

// SRTName returns the subtitle path beside output.
func SRTName(output string) string {
	return strings.TrimSuffix(output, filepath.Ext(output)) + ".srt"
}

// WriteSRT writes the project's sentence lines beside the video. It is a
// no-op when the project has no subtitles yet.
func WriteSRT(p *project.Project, path string) error {
	if p == nil || len(p.Lines) == 0 {
		return nil
	}
	cues := make([]subtitle.Cue, 0, len(p.Lines))
	for _, line := range p.Lines {
		cues = append(cues, subtitle.Cue{
			Start: line.Start,
			End:   line.End,
			Text:  line.Text,
		})
	}
	return subtitle.Write(path, cues)
}
