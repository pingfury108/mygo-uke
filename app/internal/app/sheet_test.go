package app

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/project"
)

func TestSheetModalParsesPastedText(t *testing.T) {
	t.Setenv("MYGO_UKE_SETTINGS", filepath.Join(t.TempDir(), "settings.json"))
	a := newApp()
	a.settings.SheetBaseURL = "https://example.com/v1"
	a.settings.SheetAPIKey = "test"
	a.settings.SheetModel = "vision"
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

func TestSheetImagePaneOffersPasteButton(t *testing.T) {
	t.Setenv("MYGO_UKE_SETTINGS", filepath.Join(t.TempDir(), "settings.json"))
	a := newApp()
	a.settings.SheetBaseURL = "https://example.com/v1"
	a.settings.SheetAPIKey = "test"
	a.settings.SheetModel = "vision"
	a.projPath = filepath.Join(t.TempDir(), "song", "project.json")
	a.openSheetModal()

	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if _, ok := tt.Find("从剪贴板粘贴"); !ok {
		t.Fatal("chart image pane lost its explicit paste action")
	}
}

func TestOpenSheetModalRestoresStoredImage(t *testing.T) {
	t.Setenv("MYGO_UKE_SETTINGS", filepath.Join(t.TempDir(), "settings.json"))
	a := newApp()
	a.settings.SheetBaseURL = "https://example.com/v1"
	a.settings.SheetAPIKey = "test"
	a.settings.SheetModel = "vision"
	a.proj = project.New("下一秒")
	a.proj.Duration = 8
	a.proj.ChartName = "原图.png"
	a.proj.ChartImage = "chart.png"
	a.projPath = filepath.Join(t.TempDir(), "song", "project.json")
	dir := filepath.Dir(a.projPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chart.png"), testChartPNG(t), 0o600); err != nil {
		t.Fatal(err)
	}

	a.openSheetModal()
	if a.stagedChart.Path == "" || a.stagedPreview == nil {
		t.Fatalf("stored chart not staged: %+v preview = %v", a.stagedChart, a.stagedPreview)
	}
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if _, ok := tt.Find("重新识别"); !ok {
		t.Fatal("reopened sheet modal lost its re-recognition action")
	}
}

func TestStageSheetImageWaitsForConfirmation(t *testing.T) {
	t.Setenv("MYGO_UKE_SETTINGS", filepath.Join(t.TempDir(), "settings.json"))
	a := newApp()
	a.settings.SheetBaseURL = "https://example.com/v1"
	a.settings.SheetAPIKey = "test"
	a.settings.SheetModel = "vision"
	a.openSheetModal()

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 210, B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	a.stageSheetImageData(buf.Bytes())

	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if a.sheetRunning {
		t.Fatal("staging an image started recognition before confirmation")
	}
	if a.stagedChart.Name != "剪贴板图片" || a.stagedPreview == nil {
		t.Fatalf("staged chart = %+v preview = %v", a.stagedChart, a.stagedPreview)
	}
	if _, ok := tt.Find("待识别的曲谱"); !ok {
		t.Fatal("staged image has no preview")
	}
	if _, ok := tt.Find("开始识别"); !ok {
		t.Fatal("staged image has no confirmation action")
	}

	a.openNewProject()
	a.stageSheetImageData(buf.Bytes())
	if _, _, err := a.pendingChart(); err == nil {
		t.Fatal("new project accepted a staged image before recognition")
	}
}

func TestReparseSheetImageSendsFreshRequest(t *testing.T) {
	t.Setenv("MYGO_UKE_SETTINGS", filepath.Join(t.TempDir(), "settings.json"))
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requests.Add(1)
		chords := "F  G"
		if count > 1 {
			chords = "G  Am"
		}
		content := `{"key":"C","capo":0,"lines":[{"chords":"` + chords + `","lyrics":"好想能看到 你嘴角微笑"}]}`
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 0,
			"model":   "vision-test",
			"choices": []any{map[string]any{
				"index":         0,
				"finish_reason": "stop",
				"message":       map[string]any{"role": "assistant", "content": content},
			}},
		})
	}))
	defer server.Close()

	a := newApp()
	a.settings.SheetBaseURL = server.URL
	a.settings.SheetAPIKey = "test"
	a.settings.SheetModel = "vision"
	dispatched := make(chan func(), 1)
	a.sheetDispatch = func(fn func()) { dispatched <- fn }

	a.proj = project.New("下一秒")
	a.proj.Duration = 8
	a.proj.Beats = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8}
	a.proj.Downbeats = []float64{0, 4}
	a.projPath = filepath.Join(t.TempDir(), "song", "project.json")
	a.openSheetModal()
	a.stageSheetImageData(testChartPNG(t))

	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	if err := tt.Click("开始识别"); err != nil {
		t.Fatal(err)
	}
	runSheetCallback(t, dispatched)

	if got := requests.Load(); got != 1 {
		t.Fatalf("requests after first recognition = %d, want 1", got)
	}
	if len(a.proj.Chords) != 2 || a.proj.Chords[0].Label != "F" {
		t.Fatalf("chords after first recognition = %+v", a.proj.Chords)
	}

	tt.Frame()
	if err := tt.Click("重新识别"); err != nil {
		t.Fatal(err)
	}
	runSheetCallback(t, dispatched)

	if got := requests.Load(); got != 2 {
		t.Fatalf("requests after re-recognition = %d, want 2", got)
	}
	if len(a.proj.Chords) != 2 || a.proj.Chords[0].Label != "G" {
		t.Fatalf("chords after re-recognition = %+v", a.proj.Chords)
	}
	if a.stagedChart.Name == "" {
		t.Fatal("re-recognition dropped the staged image")
	}
}

func testChartPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 210, B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func runSheetCallback(t *testing.T, dispatched <-chan func()) {
	t.Helper()
	select {
	case fn := <-dispatched:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("sheet recognition did not finish")
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
