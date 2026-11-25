package app

import "github.com/egoist/mygo/ui"

// dialog shows content in the desktop's centered modal, or in a bottom sheet
// on a phone. fraction is the tallest the sheet may be, as a part of the
// window's height.
func (a *app) dialog(c *ui.Context, open *bool, want, fraction float32, content func()) {
	if narrowDialog(c) {
		a.phoneSheet(c, open, fraction, content)
		return
	}
	ui.Modal(c, open, func() {
		ui.Column(c).Width(dialogWidth(c, want)).Gap(12).Children(content)
	})
}

func (a *app) phoneSheet(c *ui.Context, open *bool, fraction float32, content func()) {
	if open == nil || !*open {
		return
	}
	_, h := c.Size()
	if fraction <= 0 || fraction > 1 {
		fraction = 0.9
	}
	_, _, bottom, _ := c.SafeArea()
	height := h * fraction
	limit := float32(560)
	if fraction >= 0.8 {
		limit = 760
	}
	if height > limit {
		height = limit
	}

	ui.Overlay(c, func() {
		back := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			Background(ui.RGBA(0, 0, 0, 0.52)).Modal()
		if back.Clicked() || back.OverlayShortcut(0, ui.KeyEscape) {
			*open = false
		}
		back.Children(func() {
			panel := ui.Column(c).Absolute().Left(0).Right(0).Bottom(0).Height(height).
				Background(pal.panel).Radius(18, 18, 0, 0).
				Padding(14, 16, 14+bottom, 16).Gap(10).Role(ui.RoleDialog)
			panel.Shadow(0, -12, 34, 0, ui.RGBA(0, 0, 0, 0.42))
			panel.Clicked()
			panel.Children(func() {
				ui.Box(c).Width(36).Height(4).Radius(2).AlignSelf(ui.Center).
					Background(pal.lineStrong)
				ui.Scroll(c).Grow(1).Basis(0).Children(func() {
					ui.Column(c).Gap(12).Children(content)
				})
			})
		})
	})
}
