package app

import (
	"image"

	"github.com/egoist/mygo/ui"
)

// RenderFrame draws one frame offscreen, for design review and headless
// checks. It never opens a window.
func RenderFrame(width, height, orientation int) image.Image {
	a := newApp()
	a.orientation = orientation
	tester := ui.NewTester(a.view, width, height)
	tester.Frame()
	return tester.Image()
}
