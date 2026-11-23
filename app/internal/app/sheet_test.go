package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/project"
)

func TestSheetModalParsesPastedText(t *testing.T) {
	t.Setenv("MYGO_UKE_SETTINGS", filepath.Join(t.TempDir(), "settings.json"))
	a := newApp()
	a.proj = project.New("下一秒")
	a.proj.Duration = 8
	a.proj.Beats = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8}
	a.proj.Downbeats = []float64{0, 4}
	a.projPath = filepath.Join(t.TempDir(), "song", "project.json")
	a.openSheetModal()
	a.sheetPaste = "F  G  Em7  Am\n好想能看到 你嘴角微笑\nF  G  C\n最好在下一秒"

	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if err := tt.Click("文本曲谱"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("生成字幕与和弦"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if a.sheetErr != "" {
		t.Fatalf("sheet error = %q", a.sheetErr)
	}
	if len(a.proj.Lines) != 2 || len(a.proj.Chords) != 7 {
		t.Fatalf("lines = %d chords = %d", len(a.proj.Lines), len(a.proj.Chords))
	}
	if !a.proj.LinesEdited || a.sheetNote == "" {
		t.Fatalf("linesEdited = %v note = %q", a.proj.LinesEdited, a.sheetNote)
	}
	if a.proj.ChartName != "粘贴文本" || a.proj.ChartText == "" || a.proj.ChartImage != "" {
		t.Fatalf("chart = %+v", a.proj)
	}
	if saved, err := project.Load(a.projPath); err != nil {
		t.Fatal(err)
	} else if len(saved.Lines) != 2 || len(saved.Chords) != 7 {
		t.Fatalf("saved lines = %d chords = %d", len(saved.Lines), len(saved.Chords))
	} else if saved.ChartName != "粘贴文本" || saved.ChartText == "" {
		t.Fatalf("saved chart = %+v", saved)
	}
}

func TestStoreChartArchivesImage(t *testing.T) {
	dir := t.TempDir()
	a := newApp()
	a.proj = project.New("下一秒")
	a.projPath = filepath.Join(dir, "song", "project.json")
	if err := os.MkdirAll(filepath.Dir(a.projPath), 0o700); err != nil {
		t.Fatal(err)
	}

	png := filepath.Join(t.TempDir(), "source.png")
	if err := os.WriteFile(png, []byte("png-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if warning := a.storeChart(chartOrigin{Name: "source.png", Path: png}); warning != "" {
		t.Fatal(warning)
	}
	if a.proj.ChartImage != "chart.png" || a.proj.ChartName != "source.png" {
		t.Fatalf("chart = %+v", a.proj)
	}
	data, err := os.ReadFile(filepath.Join(dir, "song", "chart.png"))
	if err != nil || string(data) != "png-data" {
		t.Fatalf("archived image = %q err = %v", data, err)
	}

	webp := filepath.Join(t.TempDir(), "other.webp")
	if err := os.WriteFile(webp, []byte("webp-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.storeChart(chartOrigin{Name: "other.webp", Path: webp})
	if a.proj.ChartImage != "chart.webp" {
		t.Fatalf("replaced chart = %+v", a.proj)
	}
	if _, err := os.Stat(filepath.Join(dir, "song", "chart.png")); !os.IsNotExist(err) {
		t.Fatalf("old chart should be removed, err = %v", err)
	}

	a.storeChart(chartOrigin{Name: "粘贴文本", Text: "F\n好想能看到 你嘴角微笑"})
	if a.proj.ChartImage != "" || a.proj.ChartText == "" {
		t.Fatalf("text chart = %+v", a.proj)
	}
	if _, err := os.Stat(filepath.Join(dir, "song", "chart.webp")); !os.IsNotExist(err) {
		t.Fatalf("image should be removed for text chart, err = %v", err)
	}
}

func TestDetectSheetImage(t *testing.T) {
	for _, name := range []string{"chart.png", "chart.JPG", "chart.webp"} {
		if !isSheetImage(name) {
			t.Errorf("%q should be a sheet image", name)
		}
	}
	for _, name := range []string{"song.mp4", "song.m4a", "chart.pdf"} {
		if isSheetImage(name) {
			t.Errorf("%q should not be a sheet image", name)
		}
	}
}

func TestChartEntrySeparatesProjectAndChart(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if _, ok := tt.Find("+ 新建工程"); !ok {
		t.Fatal("new project entry missing")
	}
	if _, ok := tt.Find("+ 添加曲谱"); !ok {
		t.Fatal("chart entry missing")
	}

	a.proj.ChartName = "下一秒.png"
	tt.Frame()
	if _, ok := tt.Find("曲谱 · 下一秒.png"); !ok {
		t.Fatal("current chart entry missing")
	}
}
