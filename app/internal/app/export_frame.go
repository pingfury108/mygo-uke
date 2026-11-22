package app

import (
	"image"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/render"
)

// RenderExportFrame draws one vertical video frame: the decoded video frame,
// the current chord chip and one subtitle line. It is the same view code the
// encoder will use, and never opens a window.
func RenderExportFrame(width, height int, background *ui.Bitmap) image.Image {
	return render.Frame(width, height, background, "C", "晚风把蝉声吹进窗台")
}
