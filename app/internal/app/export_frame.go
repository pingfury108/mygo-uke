package app

import (
	"image"

	"github.com/egoist/mygo/ui"
)

// RenderExportFrame draws one vertical video frame: the decoded video frame,
// the current chord chip and one subtitle line. It is the same view code the
// encoder will use, and never opens a window.
func RenderExportFrame(width, height int, background *ui.Bitmap) image.Image {
	view := func(c *ui.Context) {
		ui.Box(c).Size(float32(width), float32(height)).Background(pal.bg).Children(func() {
			if background != nil {
				ui.Image(c, background).
					Absolute().Left(0).Top(0).Size(float32(width), float32(height))
			}
			ui.Box(c).Absolute().Left(72).Top(96).Size(132, 132).Radius(26).
				Background(ui.RGBA(255, 181, 71, 0.92)).Center().Children(func() {
				label(c, "C", 52, ui.Hex("#241a08"), 700)
			})
			label(c, "晚风把蝉声吹进窗台", 44, ui.RGB(255, 255, 255), 700).
				TextAlign(ui.Center).Width(float32(width - 144)).
				Absolute().Left(72).Bottom(float32(height - 176))
		})
	}
	tester := ui.NewTester(view, width, height)
	tester.Frame()
	return tester.Image()
}
