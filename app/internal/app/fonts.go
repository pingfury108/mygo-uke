package app

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/render"
)

var appFontFamily = "PingFang SC"

// registerBundledFont uses the packaged Noto Sans SC when it is present and
// falls back to the system font otherwise.
func registerBundledFont() {
	dir, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		return
	}
	for _, name := range []string{
		"NotoSansSC-Regular.otf",
		"fonts/NotoSansSC-Regular.otf",
		"fonts/NotoSansSC-Regular.ttf",
	} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if err := ui.RegisterFont(data, "Noto Sans SC"); err != nil {
			continue
		}
		appFontFamily = "Noto Sans SC"
		return
	}
}

func applyFontFamily() {
	studioTheme.Font = appFontFamily
	render.SetFontFamily(appFontFamily)
}
