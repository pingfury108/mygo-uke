package app

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPhoneShellSwitchesScreens(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 402, 874)
	tt.Frame()

	if a.phoneTab != phoneEditTab {
		t.Fatalf("phone tab = %d, want edit", a.phoneTab)
	}
	if _, ok := tt.Find("编辑当前句"); ok {
		t.Fatal("edit screen should not show the lyric action bar")
	}

	if err := tt.Click("歌词"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.phoneTab != phoneLyricsTab {
		t.Fatalf("phone tab = %d, want lyrics", a.phoneTab)
	}
	if _, ok := tt.Find("编辑当前句"); !ok {
		t.Fatal("lyrics screen did not render its action bar")
	}

	if err := tt.Click("和弦"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.phoneTab != phoneChordsTab {
		t.Fatalf("phone tab = %d, want chords", a.phoneTab)
	}
	if _, ok := tt.Find("按字对齐"); !ok {
		t.Fatal("chords screen did not render the per-character panel")
	}

	if err := tt.Click("工程"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.phoneTab != phoneProjectTab {
		t.Fatalf("phone tab = %d, want project", a.phoneTab)
	}
	if _, ok := tt.Find("新建工程"); !ok {
		t.Fatal("project screen did not render the new project card")
	}
}

func TestPhoneEditControlsAndLyricSeek(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 402, 874)
	tt.Frame()

	if err := tt.Click("播放"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.playing {
		t.Fatal("play button did not start playback")
	}
	if err := tt.Click("播放"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.playing {
		t.Fatal("play button did not pause playback")
	}

	if err := tt.Click("静音"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.muted {
		t.Fatal("mute button did not mute the preview")
	}

	if err := tt.Click("歌词"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("我把心事弹成了海"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if math.Abs(a.playhead-42.0) > 0.6 {
		t.Fatalf("playhead = %.2f, want the lyric start", a.playhead)
	}
	if a.selectedLine != 7 {
		t.Fatalf("selected line = %d, want 7", a.selectedLine)
	}
}

func TestPhoneChordsSeekAndOpenEditor(t *testing.T) {
	a := newApp()
	a.phoneTab = phoneChordsTab
	tt := ui.NewTester(a.view, 402, 874)
	tt.Frame()

	if err := tt.Click("第 13 小节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.playhead != a.proj.Bars[12].Start {
		t.Fatalf("playhead = %.2f, want bar 13 start %.2f", a.playhead, a.proj.Bars[12].Start)
	}
}

func TestPhoneProjectExportSheet(t *testing.T) {
	a := newApp()
	a.phoneTab = phoneProjectTab
	tt := ui.NewTester(a.view, 402, 874)
	tt.Frame()

	if err := tt.Click("导出成片"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.modal != modalExport {
		t.Fatalf("modal = %v, want export", a.modal)
	}
	if _, ok := tt.Find("开始导出"); !ok {
		t.Fatal("export sheet did not render")
	}
}

func TestPhoneDialogsUseBottomSheets(t *testing.T) {
	cases := []struct {
		name  string
		modal modalKind
		want  string
	}{
		{"export", modalExport, "开始导出"},
		{"chords", modalChords, "应用和弦"},
		{"analysis", modalAnalysis, "总进度"},
		{"line", modalLine, "保存字幕"},
		{"settings", modalSettings, "保存设置"},
		{"project", modalProject, "保存名称"},
		{"sheet", modalSheet, "导入曲谱"},
		{"new project", modalNewProject, "创建工程"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp()
			a.modal = tc.modal
			tt := ui.NewTester(a.view, 402, 874)
			tt.Frame()
			if _, ok := tt.Find(tc.want); !ok {
				t.Fatalf("phone dialog %q did not render %q", tc.name, tc.want)
			}
		})
	}
}

func TestIPadPortraitUsesPhoneShell(t *testing.T) {
	for _, size := range [][2]int{{834, 1194}, {1024, 1366}} {
		a := newApp()
		tt := ui.NewTester(a.view, size[0], size[1])
		tt.Frame()

		if _, ok := tt.Find("+ 新建工程"); ok {
			t.Fatalf("iPad portrait %dx%d should not show the landscape project rail", size[0], size[1])
		}
		if _, ok := tt.Find("和弦"); !ok {
			t.Fatalf("iPad portrait %dx%d lost the phone tab bar", size[0], size[1])
		}
	}
}

func TestWorkspaceAtIPadWidth(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1194, 834)
	tt.Frame()

	if _, ok := tt.Find("逐句字幕"); !ok {
		t.Fatal("iPad workspace lost its subtitle inspector")
	}
	if _, ok := tt.Find("+ 新建工程"); !ok {
		t.Fatal("iPad workspace lost its project rail")
	}
}
