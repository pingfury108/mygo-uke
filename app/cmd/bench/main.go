// Command bench measures offscreen frame cost without opening a window.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/app"
)

func main() {
	frames := flag.Int("n", 100, "number of frames")
	width := flag.Int("width", 1080, "frame width")
	height := flag.Int("height", 1920, "frame height")
	orientation := flag.Int("orientation", 0, "0 vertical, 1 horizontal")
	mode := flag.String("mode", "export", "export or ui")
	flag.Parse()

	background := image.NewRGBA(image.Rect(0, 0, *width, *height))
	draw.Draw(
		background,
		background.Bounds(),
		&image.Uniform{C: color.RGBA{R: 44, G: 38, B: 30, A: 255}},
		image.Point{},
		draw.Src,
	)
	bitmap := ui.NewBitmap(background)

	render := func() image.Image {
		if *mode == "ui" {
			return app.RenderFrame(*width, *height, *orientation)
		}
		return app.RenderExportFrame(*width, *height, bitmap)
	}
	_ = render()
	start := time.Now()
	for range *frames {
		_ = render()
	}
	elapsed := time.Since(start)
	average := elapsed / time.Duration(*frames)
	fmt.Printf(
		"mode=%s frames=%d size=%dx%d average=%s total=%s\n",
		*mode, *frames, *width, *height, average.Round(time.Microsecond),
		elapsed.Round(time.Millisecond),
	)
}
