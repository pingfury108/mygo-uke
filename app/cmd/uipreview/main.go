// uipreview renders the studio view without opening a window.
//
//	go run ./cmd/uipreview -out /private/tmp/mygo-uke.png
package main

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"

	"mygo-uke/internal/app"
)

func main() {
	out := flag.String("out", "ui-preview.png", "output PNG path")
	width := flag.Int("width", 1440, "window width in DIPs")
	height := flag.Int("height", 900, "window height in DIPs")
	orientation := flag.Int("orientation", 0, "0 vertical, 1 horizontal")
	flag.Parse()

	img := app.RenderFrame(*width, *height, *orientation)
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	fmt.Println(*out)
}
