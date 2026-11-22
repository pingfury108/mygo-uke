package app

import (
	"testing"

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

	if err := tt.Click("安全区"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.safe {
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
	if _, ok := tt.Find("分析和弦"); !ok {
		t.Fatal("analysis modal did not render")
	}
}

func TestRenderExportFrame(t *testing.T) {
	img := RenderExportFrame(360, 640, nil)
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
