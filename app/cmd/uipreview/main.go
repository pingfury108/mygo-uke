// uipreview renders the studio view without opening a window. -tab picks a
// phone screen (0 edit, 1 lyrics, 2 chords, 3 project) and -modal opens one of
// the dialogs on top.
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
	phoneTab := flag.Int("tab", -1, "phone screen: 0 edit, 1 lyrics, 2 chords, 3 project")
	phoneModal := flag.Int("modal", -1, "phone modal kind: 1 export, 5 settings, 8 new project")
	flag.Parse()

	img := app.RenderFrame(*width, *height, *orientation)
	if *phoneTab >= 0 {
		img = app.RenderPhoneFrame(*width, *height, *phoneTab)
	}
	if *phoneModal >= 0 {
		img = app.RenderPhoneModal(*width, *height, *phoneModal)
	}
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
