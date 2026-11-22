package app

import "github.com/egoist/mygo/ui"

type stylePreset struct {
	Name      string
	ChordSize float64
	SubSize   float64
	Accent    string
}

var stylePresets = []stylePreset{
	{Name: "琥珀夜色", ChordSize: 64, SubSize: 60, Accent: "#F3B544"},
	{Name: "朱砂现场", ChordSize: 72, SubSize: 62, Accent: "#B5482E"},
	{Name: "青绿清新", ChordSize: 60, SubSize: 56, Accent: "#1F7A74"},
	{Name: "米白极简", ChordSize: 58, SubSize: 54, Accent: "#F3EEE4"},
}

func (a *app) applyStylePreset(c *ui.Context, index int) {
	if index < 0 || index >= len(stylePresets) {
		return
	}
	preset := stylePresets[index]
	a.proj.Style.ChordSize = preset.ChordSize
	a.proj.Style.SubSize = preset.SubSize
	a.proj.Style.Accent = preset.Accent
	a.saveCurrent()
	a.toast(c, "样式已套用", preset.Name)
}

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
