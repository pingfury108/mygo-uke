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

	if err := tt.Click("和弦"); err != nil {
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
	tt.Frame()
	if _, ok := tt.Find("分析工程"); !ok {
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
	if got := a.exportEstimate(); got != "约 2 分 30 秒" {
		t.Fatalf("vertical estimate = %q", got)
	}
	a.orientation = 1
	if got := a.exportEstimate(); got != "约 1 分 48 秒" {
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

func TestPatternAndConfidence(t *testing.T) {
	if patternIndex("waltz-3") != 3 || patternID(2) != "fingerpick-3" {
		t.Fatalf("pattern mapping = %q / %q", patternID(2), patternID(patternIndex("waltz-3")))
	}
	a := newApp()
	if a.avgConfidence() <= 0 {
		t.Fatal("demo project should expose a real average confidence")
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
