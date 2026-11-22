package render

import "github.com/egoist/mygo/ui"

var fontFamily string

// SetFontFamily makes the export use the app's embedded font. An empty name
// keeps the platform font.
func SetFontFamily(name string) {
	fontFamily = name
}

func withFont(e ui.Element) ui.Element {
	if fontFamily == "" {
		return e
	}
	return e.Font(fontFamily)
}
