package app

import (
	"context"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/asr"
	"mygo-uke/internal/project"
	"mygo-uke/internal/score"
	"mygo-uke/internal/sheet"
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
	modalSheet
	modalNewProject
)

// app is the state every view reads and changes. It lives on the main
// thread; background jobs must come back through mygo.RunOnMain.
type app struct {
	win *mygo.Window

	proj        *project.Project
	projPath    string
	projects    []projectItem
	selected    int
	tab         int
	phoneTab    int
	phoneBar    int
	phoneScroll ui.ScrollState

	orientation int
	playing     bool
	muted       bool
	playhead    float64
	lastTick    time.Time
	player      *ui.VideoPlayer
	playerErr   error

	selectedLine int

	inspectorScroll [3]ui.ScrollState
	revealLine      int
	revealBar       int
	chordCursor     int

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
	jobChart   *sheet.Sheet
	jobOrigin  chartOrigin
	analysisAt time.Time

	asrConfig     asr.Config
	settings      settings
	settingsDraft settings
	settingsErr   string

	projectIndex   int
	projectTitle   string
	projectErr     string
	projectConfirm bool

	sheetRunning  bool
	sheetErr      string
	sheetNote     string
	sheetPaste    string
	sheetTab      int
	sheetMode     int
	sheetSeq      int
	sheetCancel   context.CancelFunc
	sheetDispatch func(func())
	stagedChart   chartOrigin
	stagedPreview ui.ImageSource
	chartThumb    ui.ImageSource
	chartThumbKey string

	newVideo       string
	newChart       *sheet.Sheet
	newChartOrigin chartOrigin
}

// Run starts the native MyGo window.
func Run() error {
	registerBundledFont()
	applyFontFamily()
	a := newApp()
	mygo.App.WhenReady(func() {
		a.loadLibrary()
		options := mygo.WindowOptions{
			Title:                "MyGo Uke",
			StateKey:             "main",
			TitleBarStyle:        mygo.TitleBarHiddenInset,
			TrafficLightPosition: &mygo.Point{X: 20, Y: 18},
			BackgroundColor:      "#0F1013",
			Content:              ui.View(a.view),
		}
		if runtime.GOOS != "ios" {
			options.Width = 1440
			options.Height = 900
			options.MinWidth = 1100
			options.MinHeight = 700
		}
		a.win = mygo.NewWindow(options)
		a.win.OnFileDrop(func(e *mygo.FileDropEvent) {
			if len(e.Paths) == 0 {
				return
			}
			video, chart := "", ""
			for _, path := range e.Paths {
				switch {
				case isVideoFile(path) && video == "":
					video = path
				case isSheetImage(path) && chart == "":
					chart = path
				}
			}
			if video != "" {
				if a.modal != modalNewProject {
					a.openNewProject()
				}
				a.newVideo = video
				if chart != "" {
					a.stageSheetImage(chart)
				}
				return
			}
			if chart != "" {
				if a.modal != modalNewProject {
					a.openSheetModal()
				}
				a.stageSheetImage(chart)
				return
			}
		})
		a.win.OnClose(func(*mygo.CloseEvent) {
			a.closePlayer()
			a.cancelExport()
			a.cancelSheet()
		})
	})
	return mygo.App.Run()
}

func newApp() *app {
	p := demoProject()
	s := loadSettings()
	return &app{
		proj:          p,
		projects:      demoProjects(),
		playhead:      42.35,
		selectedLine:  7,
		chordChoice:   "C",
		phoneBar:      -1,
		lineIndex:     -1,
		revealLine:    -1,
		revealBar:     -1,
		chordCursor:   -1,
		exportSRT:     false,
		asrConfig:     s.asrConfig(),
		settings:      s,
		sheetDispatch: mygo.RunOnMain,
	}
}

const (
	sheetForCurrent = iota
	sheetForNew
)

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func (a *app) view(c *ui.Context) {
	c.SetTheme(studioTheme)
	a.handleKeys(c)
	a.tick(c)

	if phoneShell(c) {
		a.phone(c)
	} else {
		a.workspace(c)
	}

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
	case modalSheet:
		a.sheetModal(c)
	case modalNewProject:
		a.newProjectModal(c)
	}
}

// workspace is the desktop and iPad landscape shell: projects, preview and
// the inspector side by side.
func (a *app) workspace(c *ui.Context) {
	ui.Column(c).Fill().Background(pal.bg).Children(func() {
		a.titlebar(c)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.rail(c)
			a.stage(c)
			a.inspector(c)
		})
		a.statusbar(c)
	})
}

func (a *app) handleKeys(c *ui.Context) {
	if a.modal != modalNone {
		if c.Shortcut(0, ui.KeyEscape) {
			if a.modal == modalSheet || a.modal == modalNewProject {
				a.closeSheet()
			} else {
				a.modal = modalNone
			}
		}
		if (a.modal == modalSheet || a.modal == modalNewProject) &&
			!c.FocusTakesText() && c.Shortcut(ui.Cmd, ui.KeyV) {
			a.pasteSheetClipboard()
		}
		return
	}
	if !c.FocusTakesText() {
		if c.Shortcut(ui.Cmd, ui.KeyV) {
			a.openSheetModal()
			a.pasteSheetClipboard()
		}
		if c.Shortcut(0, ui.KeySpace) || c.Shortcut(0, ui.KeyK) {
			a.togglePlay()
		}
		if c.Shortcut(0, ui.KeyJ) {
			a.seek(-10)
		}
		if c.Shortcut(0, ui.KeyL) {
			a.seek(10)
		}
		if c.Shortcut(0, ui.KeyLeft) {
			a.seek(-5)
		}
		if c.Shortcut(0, ui.KeyRight) {
			a.seek(5)
		}
		if c.Shortcut(0, ui.KeyM) {
			a.setMuted(!a.muted)
		}
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
		if a.playhead >= line.Start-0.01 && a.playhead < line.End {
			if a.selectedLine != i {
				a.selectedLine = i
				if !a.playing {
					a.revealLine = i
				}
			}
			return
		}
	}
}

func (a *app) lineAt(t float64) (project.Line, int, bool) {
	if len(a.proj.Lines) == 0 {
		return project.Line{}, -1, false
	}
	for i, line := range a.proj.Lines {
		if t >= line.Start && t < line.End {
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

func (a *app) currentChord() (string, bool) {
	if a.proj == nil {
		return "", false
	}
	if segment, _, ok := score.ChordAt(a.proj.Chords, a.playhead); ok {
		return segment.Label, true
	}
	return "", false
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
