// Command videoprobe opens a video headlessly and reports whether the
// system's player ever produces a picture.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

func main() {
	path := flag.String("path", "", "video path")
	paused := flag.Bool("paused", false, "pause and seek before the first frame")
	seek := flag.Float64("seek", 12, "seconds to seek to with -paused")
	flag.Parse()
	if *path == "" {
		fmt.Fprintln(os.Stderr, "videoprobe: -path is required")
		os.Exit(2)
	}
	runWithLoop(func() int {
		return probe(*path, *paused, *seek)
	})
}

func probe(path string, paused bool, seek float64) int {
	player, err := video.Open(path)
	if err != nil {
		fmt.Println("open:", err)
		return 1
	}
	defer player.Close()
	player.SetMuted(true)
	if paused {
		player.Pause()
		player.Seek(time.Duration(seek * float64(time.Second)))
	}

	tt := ui.NewTester(func(c *ui.Context) {
		ui.Video(c, player).Size(180, 320).Fit(ui.Cover)
	}, 180, 320)
	started := time.Now()
	deadline := started.Add(10 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		err := player.Err()
		if err != nil {
			fmt.Printf("error after %v: %v\n", time.Since(started).Round(time.Millisecond), err)
			return 1
		}
		if player.Ready() {
			w, h := player.Size()
			fmt.Printf("ready=%v size=%dx%d duration=%v position=%v\n",
				player.Ready(), w, h, player.Duration(), player.Position())
			return 0
		}
		time.Sleep(50 * time.Millisecond)
	}
	w, h := player.Size()
	fmt.Printf("not ready after 10s: err=%v size=%dx%d duration=%v buffering=%v\n",
		player.Err(), w, h, player.Duration(), player.Buffering())
	return 1
}
