package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
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
	if _, ok := tt.Find("重新分析"); !ok {
		t.Fatal("analysis modal did not render")
	}
}
