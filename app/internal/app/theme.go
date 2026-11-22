package app

import "github.com/egoist/mygo/ui"

// palette holds the A · studio tokens from designs/mygo-uke-ui/styles.css.
type palette struct {
	bg         ui.Color
	bg2        ui.Color
	panel      ui.Color
	panel2     ui.Color
	card       ui.Color
	sel        ui.Color
	text       ui.Color
	muted      ui.Color
	faint      ui.Color
	line       ui.Color
	lineStrong ui.Color
	accent     ui.Color
	accentInk  ui.Color
	accentLine ui.Color
	warn       ui.Color
	warnBG     ui.Color
	good       ui.Color
	mat        ui.Color
	stage      ui.Color
	frameLine  ui.Color
	wave       ui.Color
}

var pal = palette{
	bg:         ui.Hex("#0F1013"),
	bg2:        ui.Hex("#141519"),
	panel:      ui.Hex("#181A1F"),
	panel2:     ui.Hex("#1C1E24"),
	card:       ui.Hex("#1E2026"),
	sel:        ui.Hex("#25242A"),
	text:       ui.Hex("#F2EDE4"),
	muted:      ui.Hex("#96928A"),
	faint:      ui.Hex("#6D6A64"),
	line:       ui.RGBA(255, 255, 255, 0.075),
	lineStrong: ui.RGBA(255, 255, 255, 0.14),
	accent:     ui.Hex("#F3B544"),
	accentInk:  ui.Hex("#1D1507"),
	accentLine: ui.RGBA(243, 181, 68, 0.42),
	warn:       ui.Hex("#F0B36A"),
	warnBG:     ui.RGBA(243, 181, 68, 0.10),
	good:       ui.Hex("#79C39A"),
	mat:        ui.Hex("#0B0C0E"),
	stage:      ui.Hex("#101114"),
	frameLine:  ui.RGBA(255, 255, 255, 0.12),
	wave:       ui.Hex("#45423C"),
}

var studioTheme = newStudioTheme()

func newStudioTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Background = pal.bg
	t.Surface = pal.card
	t.SurfaceHover = pal.sel
	t.SurfacePressed = pal.panel2
	t.Border = pal.lineStrong
	t.Text = pal.text
	t.TextMuted = pal.muted
	t.Accent = pal.accent
	t.AccentHover = ui.Hex("#F6C260")
	t.AccentPressed = ui.Hex("#E3A63A")
	t.AccentText = pal.accentInk
	t.Danger = ui.Hex("#E07A63")
	t.Warning = pal.warn
	t.Success = pal.good
	t.Selection = ui.RGBA(243, 181, 68, 0.28)
	t.Focus = pal.accent
	t.Radius = 8
	t.Spacing = 4
	t.FontSize = 13
	t.Font = "PingFang SC"
	return t
}
