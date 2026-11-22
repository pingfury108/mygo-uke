package app

func (a *app) setBarChord(index int, value string) bool {
	if a.proj == nil || index < 0 || index >= len(a.proj.Bars) || value == "" {
		return false
	}
	bar := &a.proj.Bars[index]
	if len(bar.Chords) == 0 {
		bar.Chords = []string{value}
	} else {
		bar.Chords[0] = value
	}
	bar.Manual = true
	bar.NeedsReview = false
	a.saveCurrent()
	return true
}
