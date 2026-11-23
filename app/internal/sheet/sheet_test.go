package sheet

import (
	"math"
	"testing"

	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
)

func TestDecodeAcceptsFencedJSON(t *testing.T) {
	raw := "```json\n" +
		`{"key":"C","capo":2,"lines":[{"chords":["F","G"],"lyrics":"好想能看到 你嘴角微笑"}]}` +
		"\n```"
	parsed, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Key != "C" || parsed.Capo != 2 || len(parsed.Lines) != 1 {
		t.Fatalf("parsed = %+v", parsed)
	}
	if parsed.Lines[0].Lyrics != "好想能看到 你嘴角微笑" {
		t.Errorf("lyrics = %q", parsed.Lines[0].Lyrics)
	}
}

func TestDecodeAcceptsStringChordsAndKeySuffix(t *testing.T) {
	raw := `{"key":"C调","lines":[{"chords":"F G Em7","lyrics":"好想能看到 你嘴角微笑"}]}`
	parsed, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Key != "C" {
		t.Errorf("key = %q", parsed.Key)
	}
	if len(parsed.Lines[0].Chords) != 3 || parsed.Lines[0].Chords[0].Label != "F" {
		t.Errorf("chords = %v", parsed.Lines[0].Chords)
	}
}

func TestDecodeAcceptsBareLineArray(t *testing.T) {
	raw := `好的，结果如下：[{"chords":["F","G"],"lyrics":"好想能看到 你嘴角微笑"}]`
	parsed, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Lines) != 1 || parsed.Lines[0].Lyrics != "好想能看到 你嘴角微笑" {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestParseTextReadsChordAndLyricRows(t *testing.T) {
	parsed, err := ParseText("[主歌]\nF  G  Em7\n好想能看到 你嘴角微笑\n【副歌】\nF  G  C\n最好在下一秒\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Lines) != 2 {
		t.Fatalf("lines = %d", len(parsed.Lines))
	}
	if len(parsed.Lines[0].Chords) != 3 || parsed.Lines[0].Chords[2].Label != "Em7" {
		t.Errorf("chords = %v", parsed.Lines[0].Chords)
	}
}

func TestParseTextKeepsChordColumns(t *testing.T) {
	parsed, err := ParseText("F   G\n好想能看到 你嘴角微笑\n")
	if err != nil {
		t.Fatal(err)
	}
	marks := parsed.Lines[0].Chords
	if len(marks) != 2 {
		t.Fatalf("marks = %+v", marks)
	}
	if marks[0].At != 0 || marks[0].Label != "F" {
		t.Errorf("mark 0 = %+v", marks[0])
	}
	if marks[1].At != 2 || marks[1].Label != "G" {
		t.Errorf("mark 1 = %+v", marks[1])
	}
}

func TestParseTextAcceptsInlineChords(t *testing.T) {
	parsed, err := ParseText("[F]好想[G]能看到 你嘴角微笑\n")
	if err != nil {
		t.Fatal(err)
	}
	line := parsed.Lines[0]
	if line.Lyrics != "好想能看到 你嘴角微笑" {
		t.Fatalf("lyrics = %q", line.Lyrics)
	}
	if len(line.Chords) != 2 || line.Chords[0].At != 0 || line.Chords[1].At != 2 {
		t.Fatalf("chords = %+v", line.Chords)
	}
}

func TestParseTextReadsTempoMeterKeyAndCapo(t *testing.T) {
	parsed, err := ParseText("BPM 96\n3/4\nKey: C\nCapo 2\nF  G\n好想能看到 你嘴角微笑\n")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.BPM != 96 || parsed.TimeSignature != [2]int{3, 4} {
		t.Fatalf("tempo = %v meter = %v", parsed.BPM, parsed.TimeSignature)
	}
	if parsed.Key != "C" || parsed.Capo != 2 || len(parsed.Lines) != 1 {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestDecodeKeepsChordPositions(t *testing.T) {
	raw := `{"lines":[{"lyrics":"好想能看到","chords":[{"at":2,"label":"G"}]}]}`
	parsed, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	marks := parsed.Lines[0].Chords
	if len(marks) != 1 || marks[0].At != 2 || marks[0].Label != "G" {
		t.Fatalf("marks = %+v", marks)
	}
}

func TestDecodeMapsPrintedChordRowToCharacters(t *testing.T) {
	raw := `{"lines":[{"chords":"F      G   Em7   Am","lyrics":"好想能看到 你嘴角微笑"}]}`
	parsed, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	marks := parsed.Lines[0].Chords
	if len(marks) != 4 {
		t.Fatalf("marks = %+v", marks)
	}
	want := []project.ChordMark{
		{At: 0, Label: "F"},
		{At: 3, Label: "G"},
		{At: 6, Label: "Em7"},
		{At: 9, Label: "Am"},
	}
	for i := range want {
		if marks[i] != want[i] {
			t.Fatalf("mark %d = %+v, want %+v", i, marks[i], want[i])
		}
	}
}

func TestDecodeKeepsChordOnlyRow(t *testing.T) {
	raw := `{"lines":[{"chords":"F   G   C","lyrics":""}]}`
	parsed, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Lines) != 1 || len(parsed.Lines[0].Chords) != 3 {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestAlignUsesASRSpans(t *testing.T) {
	parsed := &Sheet{Lines: []Line{
		{Chords: marks("F", "G", "Em7", "Am"), Lyrics: "好想能看到 你嘴角微笑"},
		{Chords: marks("F", "G", "C"), Lyrics: "最好在下一秒"},
		{Chords: marks("F", "G", "Em7", "A7"), Lyrics: "好想能听到 你轻声歌唱"},
	}}
	asr := []project.Line{
		{Start: 0, End: 3, Text: "好想能看到你嘴角微笑"},
		{Start: 3, End: 5, Text: "最好在下一秒"},
		{Start: 5, End: 8, Text: "好想能听到你轻声歌唱"},
	}

	lines := Align(parsed, asr, 8)

	if len(lines) != 3 {
		t.Fatalf("lines = %d", len(lines))
	}
	if lines[0].Start != 0 || lines[0].End != 3 {
		t.Errorf("line 0 = %v-%v, want 0-3", lines[0].Start, lines[0].End)
	}
	if lines[1].Start != 3 || lines[1].End != 5 {
		t.Errorf("line 1 = %v-%v, want 3-5", lines[1].Start, lines[1].End)
	}
	if lines[2].Lyrics != parsed.Lines[2].Lyrics {
		t.Errorf("line 2 lyrics = %q", lines[2].Lyrics)
	}
}

func TestAlignSpansLinesAcrossUtterancesAndDropsUnsungRepeat(t *testing.T) {
	chart := &Sheet{Lines: []Line{
		{Lyrics: "好想能看到 你嘴角微笑 最好在下一秒 好想能听到 你轻声歌唱"},
		{Lyrics: "最好在下一秒"},
		{Lyrics: "纯白棒球帽 墨绿色衣角 时间静止的美好 默契发生在每个下一秒"},
		{Lyrics: "爱上同一种口味的蛋糕 不约而同哼唱一段曲调"},
		{Lyrics: "喜欢这样看你傻傻的笑 好想能这样 就白头到老 最好从下一秒"},
		{Lyrics: "好想能看到 你嘴角微笑 最好在下一秒 好想能听到 你轻声歌唱"},
		{Lyrics: "最好在下一秒"},
		{Lyrics: "纯白棒球帽 墨绿色衣角 时间静止的美好 默契发生在每个下一秒"},
		{Lyrics: "爱上同一种口味的蛋糕 不约而同哼唱一段曲调"},
		{Lyrics: "喜欢这样看你傻傻的笑 好想能这样 就白头到老 最好从下一秒"},
	}}
	asr := []project.Line{
		{Start: 3.65, End: 16.36, Text: "好想能看到你嘴角微笑，最好在下一秒。"},
		{Start: 16.87, End: 28.19, Text: "好想能听到你轻声歌唱，最好在下一秒。"},
		{Start: 29.69, End: 32.90, Text: "纯白的猫，"},
		{Start: 32.90, End: 36.74, Text: "墨绿色衣角，"},
		{Start: 36.74, End: 41.87, Text: "时间静止的美好。"},
		{Start: 42.45, End: 47.98, Text: "默契发生在每个下一秒。"},
		{Start: 48.67, End: 54.13, Text: "爱上同一种口味的蛋糕。"},
		{Start: 54.82, End: 60.41, Text: "不约而同哼唱一段曲调。"},
		{Start: 60.68, End: 65.88, Text: "喜欢这样看你傻傻的笑。"},
		{Start: 66.73, End: 78.61, Text: "好想能这样就白头到老，最后从下一秒。"},
	}

	lines := Align(chart, asr, 80.63)
	if len(lines) != 5 {
		t.Fatalf("lines = %d, want 5 (the repeated chart tail is not sung)", len(lines))
	}
	want := []struct{ start, end float64 }{
		{3.65, 23.945},
		{23.945, 28.19},
		{29.69, 47.98},
		{48.67, 60.41},
		{60.68, 78.61},
	}
	for i, item := range want {
		if math.Abs(lines[i].Start-item.start) > 0.001 || math.Abs(lines[i].End-item.end) > 0.001 {
			t.Errorf("line %d = %.3f-%.3f, want %.3f-%.3f",
				i, lines[i].Start, lines[i].End, item.start, item.end)
		}
	}
	if got := charTime(t, lines[0], '唱'); math.Abs(got-23.238) > 0.05 {
		t.Errorf("唱 @ %.3f, want ~23.238", got)
	}
	if got := charTime(t, lines[1], '最'); math.Abs(got-23.945) > 0.05 {
		t.Errorf("最 @ %.3f, want ~23.945", got)
	}
}

func charTime(t *testing.T, line TimedLine, target rune) float64 {
	t.Helper()
	for i, r := range []rune(line.Lyrics) {
		if r == target {
			return line.CharTimes[i]
		}
	}
	t.Fatalf("%q not found in %q", target, line.Lyrics)
	return 0
}

func TestChordsSpreadsLineChords(t *testing.T) {
	lines := []TimedLine{
		{Chords: marks("F", "G", "Em7", "Am"), Lyrics: "好想能看到 你嘴角微笑", Start: 0, End: 4},
		{Chords: marks("F", "G", "C"), Lyrics: "最好在下一秒", Start: 4, End: 7},
	}

	chords := Chords(lines, 7)

	if len(chords) != 7 {
		t.Fatalf("chords = %d, want 7", len(chords))
	}
	if chords[0].Label != "F" || chords[3].Label != "Am" {
		t.Errorf("line 0 chords = %v", chords[:4])
	}
	if chords[3].End != 4 {
		t.Errorf("last chord ends at %v, want 4", chords[3].End)
	}
}

func TestChordsUseCharacterPositionsAndLastUntilNextMark(t *testing.T) {
	lines := []TimedLine{{
		Chords: []project.ChordMark{{At: 0, Label: "F"}, {At: 5, Label: "G"}},
		Lyrics: "好想能看到你嘴角微笑",
		Start:  0,
		End:    10,
	}}
	chords := Chords(lines, 10)
	if len(chords) != 2 {
		t.Fatalf("chords = %+v", chords)
	}
	if chords[0].Start != 0 || chords[0].End != 5 || chords[0].Label != "F" {
		t.Errorf("chord 0 = %+v", chords[0])
	}
	if chords[1].Start != 5 || chords[1].End != 10 || chords[1].Label != "G" {
		t.Errorf("chord 1 = %+v", chords[1])
	}
}

func TestAlignWithoutASRSpreadsOverDuration(t *testing.T) {
	parsed := &Sheet{Lines: []Line{{Lyrics: "一"}, {Lyrics: "二"}, {Lyrics: "三"}, {Lyrics: "四"}}}

	lines := Align(parsed, nil, 8)

	if len(lines) != 4 {
		t.Fatalf("lines = %d", len(lines))
	}
	want := []TimedLine{
		{Lyrics: "一", Start: 0, End: 2},
		{Lyrics: "二", Start: 2, End: 4},
		{Lyrics: "三", Start: 4, End: 6},
		{Lyrics: "四", Start: 6, End: 8},
	}
	for i, line := range lines {
		if line.Start != want[i].Start || line.End != want[i].End || line.Lyrics != want[i].Lyrics {
			t.Fatalf("line %d = %+v, want %+v", i, line, want[i])
		}
	}
}

func TestApplyKeepsChordOnlyLinesOutOfSubtitles(t *testing.T) {
	p := project.New("下一秒")
	p.Duration = 10
	p.Beats = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	p.Downbeats = []float64{0, 4, 8}
	p.Lines = []project.Line{{Start: 2, End: 6, Text: "好想能看到 你嘴角微笑"}}
	parsed := &Sheet{Key: "C", Lines: []Line{
		{Chords: marks("F", "G")},
		{Chords: marks("F", "G", "Em7"), Lyrics: "好想能看到 你嘴角微笑"},
		{Chords: marks("C")},
	}}

	Apply(p, parsed)

	if len(p.Lines) != 1 || p.Lines[0].Text != "好想能看到 你嘴角微笑" {
		t.Fatalf("lines = %+v", p.Lines)
	}
	if len(p.Chords) != 6 {
		t.Fatalf("chords = %d, want 6", len(p.Chords))
	}
	if p.Chords[0].Label != "F" || p.Chords[5].Label != "C" {
		t.Errorf("chords = %+v", p.Chords)
	}
}

func TestApplyOnlyReplacesWhatTheChartHas(t *testing.T) {
	p := project.New("下一秒")
	p.Duration = 4
	p.Beats = []float64{0, 1, 2, 3, 4}
	p.Downbeats = []float64{0}
	p.Lines = []project.Line{{Start: 0, End: 4, Text: "原来的字幕"}}
	p.Chords = []score.ChordSegment{{Start: 0, End: 4, Label: "C", Conf: 0.5}}

	Apply(p, &Sheet{Lines: []Line{{Lyrics: "新的字幕"}}})
	if len(p.Lines) != 1 || p.Lines[0].Text != "新的字幕" {
		t.Fatalf("lines = %+v", p.Lines)
	}
	if len(p.Chords) != 1 || p.Chords[0].Label != "C" {
		t.Fatalf("chords = %+v", p.Chords)
	}

	Apply(p, &Sheet{Lines: []Line{{Chords: marks("F"), Lyrics: ""}}})
	if len(p.Lines) != 1 || p.Lines[0].Text != "新的字幕" {
		t.Fatalf("lines after chord-only chart = %+v", p.Lines)
	}
	if len(p.Chords) != 1 || p.Chords[0].Label != "F" {
		t.Fatalf("chords after chord-only chart = %+v", p.Chords)
	}
}

func TestApplyRebuildsManualBars(t *testing.T) {
	p := project.New("下一秒")
	p.Duration = 8
	p.Beats = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8}
	p.Downbeats = []float64{0, 4}
	p.Lines = []project.Line{
		{Start: 0, End: 4, Text: "好想能看到 你嘴角微笑"},
		{Start: 4, End: 8, Text: "最好在下一秒"},
	}
	parsed := &Sheet{Key: "C", Lines: []Line{
		{Chords: marks("F", "G", "Em7", "Am"), Lyrics: "好想能看到 你嘴角微笑"},
		{Chords: marks("F", "G", "C"), Lyrics: "最好在下一秒"},
	}}

	Apply(p, parsed)

	if !p.LinesEdited || len(p.Lines) != 2 || p.Key != "C" {
		t.Fatalf("project = %+v", p)
	}
	if len(p.Chords) != 7 {
		t.Fatalf("chords = %d, want 7", len(p.Chords))
	}
	for _, bar := range p.Bars {
		if len(bar.Chords) == 0 {
			continue
		}
		if !bar.Manual || bar.NeedsReview {
			t.Fatalf("bar %d = %+v, want manual", bar.I, bar)
		}
	}
}

func TestApplyBuildsBeatGridFromChartBPM(t *testing.T) {
	p := project.New("下一秒")
	p.Duration = 4
	p.Lines = []project.Line{{Start: 0, End: 4, Text: "好想能看到"}}
	parsed := &Sheet{
		BPM:           120,
		TimeSignature: [2]int{3, 4},
		Lines: []Line{{
			Chords: marks("F", "G"),
			Lyrics: "好想能看到",
		}},
	}

	Apply(p, parsed)

	if p.Tempo != 120 || p.TimeSignature != [2]int{3, 4} {
		t.Fatalf("tempo = %v meter = %v", p.Tempo, p.TimeSignature)
	}
	if len(p.Beats) != 9 || p.Beats[1] != 0.5 {
		t.Fatalf("beats = %v", p.Beats)
	}
	if len(p.Downbeats) != 3 || p.Downbeats[1] != 1.5 || p.Downbeats[2] != 3 {
		t.Fatalf("downbeats = %v", p.Downbeats)
	}
}

func TestApplyFallsBackToChordStartsWithoutBPM(t *testing.T) {
	p := project.New("下一秒")
	p.Duration = 8
	p.Lines = []project.Line{
		{Start: 0, End: 4, Text: "好想能看到 你嘴角微笑"},
		{Start: 4, End: 8, Text: "最好在下一秒"},
	}
	parsed := &Sheet{Lines: []Line{
		{Chords: marks("F", "G"), Lyrics: "好想能看到 你嘴角微笑"},
		{Chords: marks("C"), Lyrics: "最好在下一秒"},
	}}

	Apply(p, parsed)

	if len(p.Downbeats) < 2 || p.Downbeats[0] != 0 {
		t.Fatalf("downbeats = %v", p.Downbeats)
	}
	if len(p.Bars) < 2 {
		t.Fatalf("bars = %d", len(p.Bars))
	}
}

func marks(labels ...string) []project.ChordMark {
	out := make([]project.ChordMark, 0, len(labels))
	for _, label := range labels {
		out = append(out, project.ChordMark{At: -1, Label: label})
	}
	return out
}
