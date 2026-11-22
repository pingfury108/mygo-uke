package app

import (
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestBundledFontLoads(t *testing.T) {
	data, err := os.ReadFile("../../resources/fonts/NotoSansSC-Regular.otf")
	if err != nil {
		t.Fatal(err)
	}
	if err := ui.RegisterFont(data, "Noto Sans SC Test"); err != nil {
		t.Fatal(err)
	}
}
