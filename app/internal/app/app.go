package app

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/asr"
	"mygo-uke/internal/chordd"
	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
)

type modalKind int

const (
	modalNone modalKind = iota
	modalExport
	modalChords
	modalAnalysis
	modalLine
	modalSettings
	modalProject
)

// app is the state every view reads and changes. It lives on the main
// thread; background jobs must come back through mygo.RunOnMain.
type app struct {
	win *mygo.Window

	proj     *project.Project
	projPath string
	projects []projectItem
	selected int
	tab      int

	orientation int
	playing     bool
	muted       bool
	playhead    float64
	lastTick    time.Time
	player      *ui.VideoPlayer
	playerErr   error

	selectedLine int
	pattern      int

	modal          modalKind
	analysis       float64
	exportRunning  bool
	exportProgress float64
	exportETA      time.Duration
	exportCancel   context.CancelFunc
	exportStopping bool
	exportErr      string
	exportPath     string
	exportNote     string
	exportSRT      bool
	exportFormat   int
	chordBar       int
	chordChoice    string
	lineIndex      int
	lineText       string
	lineStart      float64
	lineEnd        float64

	jobRunning bool
	jobErr     string
	jobStage   string
	jobImport  bool
	jobSource  string

	chorddURL      string
	chorddOnline   bool
	chorddChecked  bool
	chorddAnalyzer string
	asrConfig      asr.Config
	settings       settings
	settingsDraft  settings
	settingsErr    string
	chorddWake     chan struct{}

	projectIndex   int
	projectTitle   string
	projectErr     string
	projectConfirm bool
}

// Run starts the native MyGo window.
func Run() error {
	registerBundledFont()
	applyFontFamily()
	a := newApp()
	mygo.App.WhenReady(func() {
		a.loadLibrary()
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:                "MyGo Uke",
			Width:                1440,
			Height:               900,
			MinWidth:             1100,
			MinHeight:            700,
			StateKey:             "main",
			TitleBarStyle:        mygo.TitleBarHiddenInset,
			TrafficLightPosition: &mygo.Point{X: 20, Y: 18},
			BackgroundColor:      "#0F1013",
			Content:              ui.View(a.view),
		})
		a.win.OnFileDrop(func(e *mygo.FileDropEvent) {
			if len(e.Paths) > 0 {
				a.startAnalysis(e.Paths[0], true)
			}
		})
		a.win.OnClose(func(*mygo.CloseEvent) { a.closePlayer() })
		a.watchChordd()
	})
	return mygo.App.Run()
}

func newApp() *app {
	p := demoProject()
	s := loadSettings()
	return &app{
		proj:         p,
		projects:     demoProjects(),
		playhead:     42.0,
		selectedLine: 6,
		chordChoice:  "C",
		lineIndex:    -1,
		chorddURL:    s.ChorddURL,
		exportSRT:    true,
		asrConfig:    s.asrConfig(),
		settings:     s,
		chorddWake:   make(chan struct{}, 1),
	}
}

func chorddBaseURL() string {
	if url := os.Getenv("MYGO_UKE_CHORDD"); url != "" {
		return url
	}
	return "http://MiWiFi-R1350-srv.local:8790"
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// watchChordd keeps the rail's service indicator honest. Network work stays
// off the main thread; only the state write comes back to it.
func (a *app) watchChordd() {
	go func() {
		for {
			var baseURL string
			mygo.RunOnMain(func() { baseURL = a.chorddURL })
			client := chordd.New(baseURL)
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			health, err := client.Check(ctx)
			cancel()
			mygo.RunOnMain(func() {
				if baseURL != a.chorddURL {
					return
				}
				a.chorddChecked = true
				a.chorddOnline = err == nil
				a.chorddAnalyzer = health.Analyzer
				if a.win != nil {
					a.win.Invalidate()
				}
			})
			select {
			case <-time.After(20 * time.Second):
			case <-a.chorddWake:
			}
		}
	}()
}

func (a *app) refreshChordd() {
	select {
	case a.chorddWake <- struct{}{}:
	default:
	}
	a.invalidate()
}

func (a *app) view(c *ui.Context) {
	c.SetTheme(studioTheme)
	a.handleKeys(c)
	a.tick(c)

	ui.Column(c).Fill().Background(pal.bg).Children(func() {
		a.titlebar(c)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.rail(c)
			a.stage(c)
			a.inspector(c)
		})
		a.statusbar(c)
	})

	switch a.modal {
	case modalExport:
		a.exportModal(c)
	case modalChords:
		a.chordModal(c)
	case modalAnalysis:
		a.analysisModal(c)
	case modalLine:
		a.lineModal(c)
	case modalSettings:
		a.settingsModal(c)
	case modalProject:
		a.projectModal(c)
	}
}

func (a *app) handleKeys(c *ui.Context) {
	if a.modal != modalNone {
		if c.Shortcut(0, ui.KeyEscape) {
			a.modal = modalNone
		}
		return
	}
	if c.Shortcut(0, ui.KeySpace) {
		a.togglePlay()
	}
	if c.Shortcut(0, ui.KeyLeft) {
		a.seek(-5)
	}
	if c.Shortcut(0, ui.KeyRight) {
		a.seek(5)
	}
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		a.save()
	}
	if c.Shortcut(ui.Cmd, ui.KeyE) {
		a.modal = modalExport
	}
}

func (a *app) tick(c *ui.Context) {
	if !a.playing {
		a.lastTick = time.Time{}
		return
	}
	if a.playerUsable() {
		a.playhead = a.player.Position().Seconds()
		if a.player.Ended() {
			a.playhead = a.proj.Duration
			a.playing = false
			a.player.Pause()
		}
		if a.playhead >= a.proj.Duration {
			a.playhead = a.proj.Duration
			a.playing = false
			a.player.Pause()
		}
		a.syncSelection()
		c.After(33 * time.Millisecond)
		return
	}
	now := time.Now()
	if !a.lastTick.IsZero() {
		a.playhead += now.Sub(a.lastTick).Seconds()
	}
	a.lastTick = now
	if a.playhead >= a.proj.Duration {
		a.playhead = a.proj.Duration
		a.playing = false
	}
	a.syncSelection()
	c.After(33 * time.Millisecond)
}

func (a *app) togglePlay() {
	if a.playing {
		a.playing = false
		if a.player != nil {
			a.player.Pause()
		}
		return
	}
	if a.playhead >= a.proj.Duration {
		a.playhead = 0
		a.seekPlayer()
	}
	a.playing = true
	a.lastTick = time.Now()
	if a.player != nil && a.player.Err() == nil {
		a.player.Seek(a.position())
		a.player.Play()
	}
}

func (a *app) seek(delta float64) {
	a.playhead += delta
	if a.playhead < 0 {
		a.playhead = 0
	}
	if a.playhead > a.proj.Duration {
		a.playhead = a.proj.Duration
	}
	a.seekPlayer()
	a.syncSelection()
}

func (a *app) syncSelection() {
	for i, line := range a.proj.Lines {
		if a.playhead >= line.Start-0.01 && a.playhead <= line.End+0.01 {
			a.selectedLine = i
			return
		}
	}
}

func (a *app) lineAt(t float64) (project.Line, int, bool) {
	if len(a.proj.Lines) == 0 {
		return project.Line{}, -1, false
	}
	for i, line := range a.proj.Lines {
		if t >= line.Start && t <= line.End {
			return line, i, true
		}
	}
	best := 0
	for i, line := range a.proj.Lines {
		if line.Start <= t {
			best = i
		}
	}
	return a.proj.Lines[best], best, true
}

func (a *app) barAt(t float64) (score.Bar, int, bool) {
	return score.BarAt(a.proj.Bars, t)
}

func (a *app) save() {
	if a.proj == nil || a.projPath == "" {
		return
	}
	if err := project.Save(a.projPath, a.proj); err != nil {
		log.Printf("save project: %v", err)
	}
}

func (a *app) saveCurrent() {
	a.save()
}

func (a *app) toast(c *ui.Context, title, description string) {
	c.AddToast(ui.Toast{Title: title, Description: description, Type: "success", Timeout: 3 * time.Second})
}
