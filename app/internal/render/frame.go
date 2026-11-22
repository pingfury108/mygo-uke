// Package render draws the video overlay shared by preview and export.
package render

import (
	"image"

	"github.com/egoist/mygo/ui"
)

// Frame draws one video frame with the current chord and subtitle.
func Frame(width, height int, background *ui.Bitmap, chord, subtitle string) image.Image {
	chip := float32(height) / 14
	chipFont := chip * 0.39
	subtitleSize := max(float32(28), float32(height)/43)
	view := func(c *ui.Context) {
		ui.Box(c).Size(float32(width), float32(height)).Background(ui.Hex("#0F1013")).Children(func() {
			if background != nil {
				ui.Image(c, background).
					Absolute().Left(0).Top(0).Size(float32(width), float32(height))
			}
			if chord != "" {
				ui.Box(c).Absolute().Left(chip*0.53).Top(chip*0.7).Size(chip, chip).
					Radius(chip * 0.2).Background(ui.RGBA(255, 181, 71, 0.92)).
					Center().Children(func() {
					ui.Text(c, chord).FontSize(chipFont).FontWeight(700).
						TextColor(ui.Hex("#241a08"))
				})
			}
			if subtitle != "" {
				pad := subtitleSize * 1.3
				ui.Box(c).Absolute().Left(pad).Right(pad).Bottom(pad * 2.4).
					Children(func() {
						ui.Text(c, subtitle).FontSize(subtitleSize).FontWeight(700).
							TextColor(ui.RGB(255, 255, 255)).TextAlign(ui.Center).
							Width(float32(width) - pad*2)
					})
			}
		})
	}
	tester := ui.NewTester(view, width, height)
	tester.Frame()
	return tester.Image()
}
