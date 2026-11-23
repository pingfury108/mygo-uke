package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
)

func TestStudioInteractions(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()

	if _, ok := tt.Find("逐句字幕"); !ok {
		t.Fatal("subtitle inspector did not render")
	}

	if err := tt.Click("小节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.tab != 1 {
		t.Fatalf("tab = %d, want 1", a.tab)
	}
	if _, ok := tt.Find("常用和弦"); !ok {
		t.Fatal("chord panel did not render")
	}

	if err := tt.Click("样式"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.tab != 2 {
		t.Fatalf("tab = %d, want 2", a.tab)
	}
	if _, ok := tt.Find("叠加层"); !ok {
		t.Fatal("style panel did not render")
	}
	if err := tt.Click("朱砂现场"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.proj.Style.Accent != "#B5482E" {
		t.Fatalf("preset accent = %q", a.proj.Style.Accent)
	}

	if err := tt.Click("安全区"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.proj.Style.ShowSafe {
		t.Fatal("safe area did not toggle")
	}

	if err := tt.Click("横屏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.orientation != 1 {
		t.Fatalf("orientation = %d, want 1", a.orientation)
	}
	if _, ok := tt.Find("当前和弦"); !ok {
		t.Fatal("horizontal preview did not render")
	}

	if err := tt.Click("导出成片"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.modal != modalExport {
		t.Fatalf("modal = %v, want export", a.modal)
	}
	if _, ok := tt.Find("开始导出"); !ok {
		t.Fatal("export modal did not render")
	}
	if _, ok := tt.Find("已烧进画面 · 逐字"); !ok {
		t.Fatal("export modal did not explain burned-in subtitles")
	}
	if a.exportSRT {
		t.Fatal("SRT should be opt-in")
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.modal != modalNone {
		t.Fatalf("modal = %v, want none", a.modal)
	}

	if err := tt.Click("字幕"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("添加"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.modal != modalLine || a.lineIndex != -1 {
		t.Fatalf("modal = %v, lineIndex = %d", a.modal, a.lineIndex)
	}
	if _, ok := tt.Find("保存字幕"); !ok {
		t.Fatal("new line modal did not render")
	}
}

func TestTimelineDragScrubs(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	rect, ok := tt.Find("时间轴")
	if !ok {
		t.Fatal("timeline not found")
	}
	y := rect.Y + rect.H/2
	tt.Press(rect.X+rect.W*0.2, y)
	tt.Move(rect.X+rect.W*0.7, y)
	tt.Release(rect.X+rect.W*0.7, y)
	want := a.proj.Duration * 0.7
	if diff := a.playhead - want; diff < -2 || diff > 2 {
		t.Fatalf("playhead = %.2f, want %.2f", a.playhead, want)
	}
}

func TestPlaybackShortcuts(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()

	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if !a.playing {
		t.Fatal("space did not start playback")
	}
	tt.Key(0, ui.KeyK)
	tt.Frame()
	if a.playing {
		t.Fatal("K did not pause playback")
	}

	a.playhead = 20
	tt.Key(0, ui.KeyJ)
	tt.Frame()
	if a.playhead != 10 {
		t.Fatalf("J seek = %.1f, want 10", a.playhead)
	}
	tt.Key(0, ui.KeyL)
	tt.Frame()
	if a.playhead != 20 {
		t.Fatalf("L seek = %.1f, want 20", a.playhead)
	}

	muted := a.muted
	tt.Key(0, ui.KeyM)
	tt.Frame()
	if a.muted == muted {
		t.Fatal("M did not toggle mute")
	}
}

func TestInspectorToolbarStaysFixed(t *testing.T) {
	a := newApp()
	for i := 0; i < 30; i++ {
		start := 80 + float64(i)*3
		a.proj.Lines = append(a.proj.Lines, project.Line{
			Start: start,
			End:   start + 2.8,
			Text:  "长视频字幕",
		})
	}
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()

	before, ok := tt.Find("编辑选中")
	if !ok {
		t.Fatal("subtitle edit action missing")
	}
	a.inspectorScroll[0].Y = 600
	tt.Frame()
	after, ok := tt.Find("编辑选中")
	if !ok {
		t.Fatal("subtitle edit action scrolled away")
	}
	if before.Y != after.Y {
		t.Fatalf("subtitle toolbar moved from %.1f to %.1f", before.Y, after.Y)
	}
	if err := tt.Click("编辑选中"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.modal != modalLine {
		t.Fatalf("modal = %v, want line editor", a.modal)
	}
}

func TestInspectorRevealsSelectedLine(t *testing.T) {
	a := newApp()
	a.proj.Lines = nil
	for i := 0; i < 50; i++ {
		start := float64(i) * 3
		a.proj.Lines = append(a.proj.Lines, project.Line{
			Start: start,
			End:   start + 2.8,
			Text:  "远处理字幕",
		})
	}
	a.playhead = 147
	a.selectedLine = 0
	a.syncSelection()
	if a.selectedLine != 49 || a.revealLine != 49 {
		t.Fatalf("selectedLine = %d, revealLine = %d", a.selectedLine, a.revealLine)
	}

	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	tt.Frame()
	if a.inspectorScroll[0].Y <= 0 {
		t.Fatalf("subtitle list did not scroll to selected line: Y = %.1f", a.inspectorScroll[0].Y)
	}
}

func TestSubtitleClickSeeks(t *testing.T) {
	a := newApp()
	a.playing = true
	line := a.proj.Lines[2]
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if err := tt.Click(line.Text); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.playhead != line.Start {
		t.Fatalf("playhead = %.1f, want %.1f", a.playhead, line.Start)
	}
	if a.playing {
		t.Fatal("clicking a subtitle did not pause playback")
	}
}

func TestSubtitleChordChipOpensBarEditor(t *testing.T) {
	a := newApp()
	a.proj.Lines[0].Chords = []project.ChordMark{
		{At: 0, Label: "Bbmaj7"},
		{At: 4, Label: "Cmaj7"},
	}
	a.selectedLine = 3
	a.playhead = 0
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if _, ok := tt.Find("Cmaj7"); !ok {
		t.Fatal("subtitle line chord chip missing")
	}
	if err := tt.Click("Cmaj7"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.modal != modalChords {
		t.Fatalf("modal = %v, want chord editor", a.modal)
	}
	if a.chordBar < 0 || a.chordChoice != "Cmaj7" {
		t.Fatalf("chordBar = %d, choice = %q", a.chordBar, a.chordChoice)
	}
	if a.selectedLine != 0 {
		t.Fatalf("selectedLine = %d, want 0", a.selectedLine)
	}
	if a.playhead < a.proj.Lines[0].Start+0.5 {
		t.Fatalf("playhead = %.2f, chip should seek past the line start", a.playhead)
	}
}

func TestSubtitleChordChipSitsAboveItsLyricSpan(t *testing.T) {
	a := newApp()
	a.proj.Lines[0].Text = "好想能看到 你嘴角微笑"
	a.proj.Lines[0].Chords = []project.ChordMark{
		{At: 0, Label: "F"},
		{At: 3, Label: "Em7"},
	}
	a.selectedLine = 0
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()

	chip, ok := tt.Find("Em7")
	if !ok {
		t.Fatal("subtitle chord chip missing")
	}
	lyric, ok := tt.Find("看到 你嘴角微笑")
	if !ok {
		t.Fatal("second lyric span missing")
	}
	if chip.X < lyric.X-1 || chip.X > lyric.X+1 {
		t.Fatalf("chip X = %.1f, lyric X = %.1f", chip.X, lyric.X)
	}
	if chip.Y+chip.H > lyric.Y {
		t.Fatalf("chip bottom = %.1f, lyric top = %.1f", chip.Y+chip.H, lyric.Y)
	}
}

func TestSubtitleChordChipWrapsWithItsLongSpan(t *testing.T) {
	a := newApp()
	a.proj.Lines[0].Text = "纯白棒球帽 墨绿色衣角 时间静止的美好 默契发生在每个下一秒"
	a.proj.Lines[0].Chords = []project.ChordMark{
		{At: 0, Label: "F0"},
		{At: 3, Label: "G0"},
		{At: 6, Label: "Em0"},
		{At: 9, Label: "A0"},
		{At: 12, Label: "F1"},
		{At: 15, Label: "G1"},
		{At: 18, Label: "C1"},
		{At: 20, Label: "F2"},
		{At: 29, Label: "Em1"},
	}
	a.selectedLine = 0
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()

	first, ok := tt.Find("F0")
	if !ok {
		t.Fatal("first chord chip missing")
	}
	chip, ok := tt.Find("F2")
	if !ok {
		t.Fatal("wrapped chord chip missing")
	}
	lyric, ok := tt.Find("默契发生在每个下一")
	if !ok {
		t.Fatal("wrapped lyric span missing")
	}
	if chip.X < lyric.X-1 || chip.X > lyric.X+1 {
		t.Fatalf("chip X = %.1f, lyric X = %.1f", chip.X, lyric.X)
	}
	if chip.Y+chip.H > lyric.Y {
		t.Fatalf("chip bottom = %.1f, lyric top = %.1f", chip.Y+chip.H, lyric.Y)
	}
	if chip.Y <= first.Y {
		t.Fatalf("chip Y = %.1f, first chip Y = %.1f", chip.Y, first.Y)
	}
}

func TestChordToolbarStaysFixed(t *testing.T) {
	a := newApp()
	a.tab = 1
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()

	before, ok := tt.Find("常用和弦")
	if !ok {
		t.Fatal("chord palette missing")
	}
	a.inspectorScroll[1].Y = 600
	tt.Frame()
	after, ok := tt.Find("常用和弦")
	if !ok {
		t.Fatal("chord palette scrolled away")
	}
	if before.Y != after.Y {
		t.Fatalf("chord palette moved from %.1f to %.1f", before.Y, after.Y)
	}

	a.playhead = a.proj.Bars[len(a.proj.Bars)-1].Start
	a.chordCursor = -1
	tt.Frame()
	tt.Frame()
	if a.inspectorScroll[1].Y <= 0 {
		t.Fatalf("bar list did not scroll to current bar: Y = %.1f", a.inspectorScroll[1].Y)
	}
}

func TestChordPaletteTooltipExplainsChord(t *testing.T) {
	a := newApp()
	a.tab = 1
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()

	r, ok := tt.Find("Cmaj7")
	if !ok {
		t.Fatal("chord palette button missing")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(700 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("Cmaj7 大七和弦 · C E G B") {
		t.Fatalf("chord tooltip missing: %v", tt.Texts())
	}
}

func TestChordRowSeeksWithoutFooter(t *testing.T) {
	a := newApp()
	a.tab = 1
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if _, ok := tt.Find("编辑当前小节"); ok {
		t.Fatal("chord footer should be removed")
	}

	bar := a.proj.Bars[2]
	if err := tt.Click(timeText(bar.Start)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.playhead != bar.Start {
		t.Fatalf("playhead = %.1f, want %.1f", a.playhead, bar.Start)
	}
	if a.playing {
		t.Fatal("clicking a bar did not pause playback")
	}
	if a.modal != modalNone {
		t.Fatal("single click opened the chord editor")
	}
}

func TestStudioAtMinimumSize(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if _, ok := tt.Find("夏夜来信"); !ok {
		t.Fatal("project title missing at minimum size")
	}
}

func TestProjectSwitch(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if err := tt.Click("晴天"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.selected != 1 || a.proj.Title != "晴天" {
		t.Fatalf("selected = %d, title = %q", a.selected, a.proj.Title)
	}
}

func TestModalsRender(t *testing.T) {
	a := newApp()
	a.modal = modalChords
	a.chordBar = 12
	a.chordChoice = "G"
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if _, ok := tt.Find("应用和弦"); !ok {
		t.Fatal("chord modal did not render")
	}

	a.modal = modalAnalysis
	a.analysis = 0.5
	a.jobRunning = true
	tt.Frame()
	if _, ok := tt.Find("正在生成"); !ok {
		t.Fatal("analysis modal did not render")
	}

	a.modal = modalLine
	a.lineIndex = 0
	a.lineText = "改过的字幕"
	a.lineStart, a.lineEnd = 42, 45
	tt.Frame()
	if _, ok := tt.Find("保存字幕"); !ok {
		t.Fatal("line modal did not render")
	}

	a.settingsDraft = a.settings
	a.modal = modalSettings
	tt.Frame()
	if _, ok := tt.Find("保存设置"); !ok {
		t.Fatal("settings modal did not render")
	}
}

func TestNewProjectModalRendersOneStepImport(t *testing.T) {
	a := newApp()
	a.openNewProject()
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if _, ok := tt.Find("新建工程"); !ok {
		t.Fatal("new project modal did not render")
	}
	if _, ok := tt.Find("选择弹唱视频"); !ok {
		t.Fatal("video picker missing")
	}
	if _, ok := tt.Find("创建工程"); !ok {
		t.Fatal("create action missing")
	}
	if err := tt.Click("文本曲谱"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("解析曲谱"); !ok {
		t.Fatal("text chart parser missing")
	}
}

func TestChordModalRendersCandidates(t *testing.T) {
	a := newApp()
	a.proj.Bars[0].Candidates = []string{"Gmaj7", "Bm"}
	a.chordBar = 0
	a.chordChoice = "G"
	a.modal = modalChords
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	for _, name := range []string{"识别候选", "Gmaj7", "Bm"} {
		if _, ok := tt.Find(name); !ok {
			t.Fatalf("candidate %q did not render", name)
		}
	}
}

func TestSettingsModalSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	t.Setenv("MYGO_UKE_SETTINGS", path)
	a := newApp()
	a.openSettings()
	a.settingsDraft.VolcanoAPIKey = "test-key"
	a.settingsDraft.VolcanoResourceID = "test-resource"

	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if err := tt.Click("保存设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.modal != modalNone {
		t.Fatalf("modal = %v, want none", a.modal)
	}
	if a.settings.VolcanoAPIKey != "test-key" || a.asrConfig.APIKey != "test-key" {
		t.Fatalf("settings = %+v asr = %+v", a.settings, a.asrConfig)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestExportEstimate(t *testing.T) {
	a := newApp()
	a.proj.Duration = 60
	if got := a.exportEstimate(); got != "约 42 秒" {
		t.Fatalf("vertical estimate = %q", got)
	}
	a.orientation = 1
	if got := a.exportEstimate(); got != "约 36 秒" {
		t.Fatalf("horizontal estimate = %q", got)
	}
	if got := durationWords(90 * time.Second); got != "1 分 30 秒" {
		t.Fatalf("durationWords = %q", got)
	}
}

func TestProjectRename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "song", "project.json")
	p := project.New("旧名字")
	if err := project.Save(path, p); err != nil {
		t.Fatal(err)
	}
	a := newApp()
	a.applyImported(p, path, true)
	a.openProjectModal(0)
	a.projectTitle = "新名字"

	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if err := tt.Click("保存名称"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	loaded, err := project.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Title != "新名字" || a.projects[0].title != "新名字" {
		t.Fatalf("title = %q / %q", loaded.Title, a.projects[0].title)
	}
}

func TestProjectDeleteNeedsConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "song", "project.json")
	p := project.New("待删除")
	if err := project.Save(path, p); err != nil {
		t.Fatal(err)
	}
	a := newApp()
	a.applyImported(p, path, true)
	a.openProjectModal(0)

	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, err := os.Stat(path); err != nil {
		t.Fatal("project was deleted before confirmation")
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("project still exists: %v", err)
	}
}

func TestRenderExportFrame(t *testing.T) {
	img := RenderExportFrame(360, 640, nil, 0)
	if got := img.Bounds().Size(); got.X != 360 || got.Y != 640 {
		t.Fatalf("export frame size = %v", got)
	}
}

func TestRenderImportedProjectWithoutLines(t *testing.T) {
	p := project.New("导入测试")
	p.Duration = 12
	p.Tempo = 100
	p.Key = "G"
	p.Downbeats = []float64{0, 3, 6, 9}
	p.Chords = []score.ChordSegment{
		{Start: 0, End: 3, Label: "G", Conf: 0.9},
		{Start: 3, End: 6, Label: "C", Conf: 0.8},
	}
	p.RebuildBars()

	a := newApp()
	a.proj = p
	a.projects = append([]projectItem{a.projectItem(p, "/tmp/project.json", 0)}, a.projects...)
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if _, ok := tt.Find("导入测试"); !ok {
		t.Fatal("imported project title did not render")
	}
	if _, ok := tt.Find("0 句"); !ok {
		t.Fatal("empty subtitle list did not render")
	}
}

func TestSaveUsesProjectPath(t *testing.T) {
	p := demoProject()
	path := filepath.Join(t.TempDir(), "project.json")
	if err := project.Save(path, p); err != nil {
		t.Fatal(err)
	}
	a := newApp()
	a.proj = p
	a.projPath = path
	p.Lines[0].Text = "改过的句子"
	a.save()

	loaded, err := project.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Lines[0].Text != "改过的句子" {
		t.Fatalf("saved text = %q", loaded.Lines[0].Text)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestAverageConfidence(t *testing.T) {
	a := newApp()
	if a.avgConfidence() <= 0 {
		t.Fatal("demo project should expose a real average confidence")
	}
}

func TestMetaLineHidesUnknownTempoAndDefaultMeter(t *testing.T) {
	p := project.New("测试")
	p.Duration = 81
	p.Key = "C"
	if got := metaLine(p); got != "01:21 · C 调 · Capo 0" {
		t.Fatalf("meta = %q", got)
	}
	p.Tempo = 96
	if got := metaLine(p); got != "01:21 · 96 BPM · 4/4 · C 调 · Capo 0" {
		t.Fatalf("meta with BPM = %q", got)
	}
	p.Tempo = 0
	p.TimeSignature = [2]int{3, 4}
	if got := metaLine(p); got != "01:21 · 3/4 · C 调 · Capo 0" {
		t.Fatalf("meta with 3/4 = %q", got)
	}
}

func TestApplyImportedPrependsProject(t *testing.T) {
	a := newApp()
	p := project.New("新歌")
	a.applyImported(p, "/tmp/new/project.json", true)

	if a.proj != p || a.selected != 0 || a.projects[0].title != "新歌" {
		t.Fatalf("selected=%d title=%q", a.selected, a.projects[0].title)
	}
	if len(a.projects) != len(demoProjects())+1 {
		t.Fatalf("projects = %d", len(a.projects))
	}
}
