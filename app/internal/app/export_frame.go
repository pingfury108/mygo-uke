package app

import (
	"image"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/render"
)

// RenderExportFrame draws one video frame with the real overlay layout. It is
// the same view code the encoder uses, and never opens a window.
func RenderExportFrame(width, height int, background *ui.Bitmap, orientation int) image.Image {
	p := demoProject()
	videoWidth, videoHeight := width, height
	if orientation == 1 {
		videoWidth, videoHeight = render.HorizontalVideoWidth(height), height
	}
	return render.Frame(render.Spec{
		Width:       width,
		Height:      height,
		VideoWidth:  videoWidth,
		VideoHeight: videoHeight,
		Background:  background,
		Project:     p,
		Time:        p.Lines[0].Start,
		Orientation: orientation,
	})
}
