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

// RenderPhoneFrame renders one phone screen without opening a window. It is
// what uipreview uses while the mobile shell is being tuned.
func RenderPhoneFrame(width, height, tab int) image.Image {
	a := newApp()
	a.phoneTab = tab
	tester := ui.NewTester(a.view, width, height)
	tester.Frame()
	return tester.Image()
}

// RenderPhoneModal renders a phone screen with one of the desktop dialogs
// open, to check that the adaptive width still fits.
func RenderPhoneModal(width, height, modal int) image.Image {
	a := newApp()
	a.phoneTab = phoneProjectTab
	a.modal = modalKind(modal)
	tester := ui.NewTester(a.view, width, height)
	tester.Frame()
	return tester.Image()
}
