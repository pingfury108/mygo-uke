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
	if a.player == nil || !a.player.Ready() {
		return
	}
	ui.Video(c, a.player).Fill().Fit(fit)
}
