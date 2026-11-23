package app

import (
	"os"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

// openPlayerFor loads the project's source into the system video player.
// It runs on the main thread, as every player method does.
func (a *app) openPlayerFor(source string) {
	a.closePlayer()
	a.playerErr = nil
	if source == "" {
		return
	}
	path := expandHome(source)
	if _, err := os.Stat(path); err != nil {
		a.playerErr = err
		return
	}
	player, err := video.Open(path)
	if err != nil {
		a.playerErr = err
		return
	}
	a.player = player
	player.SetMuted(a.muted)
	player.Pause()
	player.Seek(a.position())
}

func (a *app) closePlayer() {
	if a.player != nil {
		a.player.Close()
		a.player = nil
	}
}

func (a *app) seekPlayer() {
	if a.player != nil {
		a.player.Seek(a.position())
	}
}

// seekTo pauses and moves the video to t, keeping the subtitle and bar
// selections in step.
func (a *app) seekTo(t float64) {
	a.playhead = t
	a.playing = false
	if a.player != nil {
		a.player.Pause()
	}
	a.seekPlayer()
	a.syncSelection()
}

func (a *app) position() time.Duration {
	return time.Duration(a.playhead * float64(time.Second))
}

func (a *app) setMuted(muted bool) {
	a.muted = muted
	if a.player != nil {
		a.player.SetMuted(muted)
	}
}

func (a *app) playerUsable() bool {
	return a.player != nil && a.player.Ready() && a.player.Err() == nil
}

func (a *app) videoLayer(c *ui.Context, fit ui.Fit) {
	if a.player == nil {
		return
	}
	// Mount the Video even before the first frame: the element is what
	// asks the engine for pictures, and the engine becomes Ready only once
	// one is taken. Gating this on Ready would never show the video.
	ui.Video(c, a.player).Fill().Fit(fit)
}

func (a *app) videoStatus() string {
	if a.playerErr != nil {
		return "视频无法打开"
	}
	if a.player == nil {
		return "没有源视频"
	}
	if err := a.player.Err(); err != nil {
		return "视频无法播放"
	}
	if !a.player.Ready() {
		return "正在加载视频…"
	}
	return ""
}
