package app

import (
	"fmt"

	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
)

type projectItem struct {
	title string
	meta  string
	state string
	chord string
	tone  int
}

func demoProjects() []projectItem {
	return []projectItem{
		{title: "夏夜来信", meta: "03:18 · C 调", state: "编辑中", chord: "C", tone: 0},
		{title: "晴天", meta: "04:29 · G 调", state: "已导出", chord: "G", tone: 1},
		{title: "小情歌", meta: "03:52 · F 调", state: "待校对", chord: "Am", tone: 2},
	}
}

func demoProject() *project.Project {
	p := project.New("夏夜来信")
	p.Source = "~/Movies/夏夜来信.mov"
	p.SourceHash = "sha256:demo"
	p.Duration = 198
	p.Tempo = 68
	p.Key = "C"
	p.Capo = 0

	start := 0.52
	barDur := 3.3
	progression := []string{"C", "G", "Am", "Em", "F", "C", "Dm", "G", "C", "G", "Am", "Em"}
	for i := 0; start+float64(i)*barDur < p.Duration; i++ {
		d := start + float64(i)*barDur
		p.Beats = append(p.Beats, d, d+barDur/4, d+barDur/2, d+barDur*3/4)
		p.Downbeats = append(p.Downbeats, d)
		conf := 0.9
		switch i {
		case 4, 18, 32:
			conf = 0.52
		}
		label := progression[i%len(progression)]
		p.Chords = append(p.Chords, score.ChordSegment{
			Start: d,
			End:   d + barDur,
			Label: label,
			Conf:  conf,
		})
	}
	p.RebuildBars()

	p.Lines = []project.Line{
		{Start: 12.0, End: 16.4, Text: "路灯把影子拉得很长"},
		{Start: 16.4, End: 20.8, Text: "我抱着琴坐在旧台阶上"},
		{Start: 20.8, End: 25.2, Text: "风从巷口轻轻经过"},
		{Start: 25.2, End: 29.6, Text: "像谁没有说完的晚安"},
		{Start: 29.6, End: 34.0, Text: "你说夏天会记得吗"},
		{Start: 34.0, End: 38.2, Text: "那些没寄出的回答"},
		{Start: 38.2, End: 42.0, Text: "晚风把蝉声吹进窗台"},
		{Start: 42.0, End: 45.8, Text: "我把心事弹成了海"},
		{Start: 45.8, End: 50.1, Text: "你说月色刚好 别急着离开"},
		{Start: 50.1, End: 54.4, Text: "这一封夏夜的信 还没拆"},
		{Start: 54.4, End: 58.8, Text: "灯影落在琴弦上"},
		{Start: 58.8, End: 63.2, Text: "我把副歌又唱了一遍"},
	}
	return p
}

func timeText(seconds float64) string { return score.TimeText(seconds) }

func metaLine(p *project.Project) string {
	return fmt.Sprintf("%s · %.0f BPM · %d/%d · %s 调 · Capo %d",
		timeText(p.Duration), p.Tempo, p.TimeSignature[0], p.TimeSignature[1], p.Key, p.Capo)
}
