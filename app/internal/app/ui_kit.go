package app

import "github.com/egoist/mygo/ui"

func text(c *ui.Context, s string, size float32, color ui.Color) ui.Element {
	return ui.Text(c, s).FontSize(size).TextColor(color)
}

func mono(c *ui.Context, s string, size float32, color ui.Color) ui.Element {
	return ui.Text(c, s).FontSize(size).TextColor(color).Font("monospace")
}

func label(c *ui.Context, s string, size float32, color ui.Color, weight int) ui.Element {
	return ui.Text(c, s).FontSize(size).TextColor(color).FontWeight(weight)
}

// flatButton is the quiet bordered button used in the title bar and stage
// header.
func flatButton(c *ui.Context, textValue string, onClick func()) ui.Element {
	b := ui.ButtonBase(c).Height(28).PaddingX(11).Radius(7).Center().
		Border(1, pal.lineStrong).Label(textValue).FocusRing(false)
	if b.Hovered() {
		b.Background(pal.sel)
	}
	if onClick != nil {
		b.OnClick(onClick)
	}
	b.Children(func() {
		text(c, textValue, 11, pal.muted)
	})
	return b
}

// ghostButton is the quieter action inside the stage.
func ghostButton(c *ui.Context, textValue string, onClick func()) ui.Element {
	b := ui.ButtonBase(c).Height(26).PaddingX(9).Radius(7).Center().
		Border(1, pal.lineStrong).Label(textValue).FocusRing(false)
	if b.Hovered() {
		b.Background(pal.sel)
	}
	if onClick != nil {
		b.OnClick(onClick)
	}
	b.Children(func() {
		text(c, textValue, 10, pal.muted)
	})
	return b
}

func primaryButton(c *ui.Context, textValue string, onClick func()) ui.Element {
	b := ui.ButtonBase(c).Height(28).PaddingX(12).Radius(7).Center().
		Background(pal.accent).Label(textValue).FocusRing(false)
	if b.Hovered() {
		b.Background(ui.Hex("#F6C260"))
	}
	if onClick != nil {
		b.OnClick(onClick)
	}
	b.Children(func() {
		label(c, textValue, 11, pal.accentInk, 650)
	})
	return b
}

func segment(c *ui.Context, values []string, selected *int) ui.Element {
	row := ui.Row(c).Padding(2).Gap(0).Radius(8).Border(1, pal.lineStrong).
		Background(pal.panel).AlignItems(ui.Center)
	row.Children(func() {
		for i, value := range values {
			index := i
			b := ui.ButtonBase(c).Height(24).PaddingX(10).Radius(6).Center().
				Label(value).FocusRing(false)
			if *selected == index {
				b.Background(pal.sel)
			} else if b.Hovered() {
				b.Background(pal.panel2)
			}
			b.OnClick(func() { *selected = index })
			b.Children(func() {
				color := pal.muted
				weight := 400
				if *selected == index {
					color = pal.text
					weight = 600
				}
				label(c, value, 10, color, weight)
			})
		}
	})
	return row
}

func badge(c *ui.Context, value string, fg, bg ui.Color) ui.Element {
	row := ui.Row(c).Padding(3, 8, 3, 8).Radius(999).Background(bg).
		AlignItems(ui.Center).Gap(5)
	row.Children(func() {
		text(c, value, 9.5, fg)
	})
	return row
}

func dot(c *ui.Context, color ui.Color) ui.Element {
	return ui.Box(c).Size(5, 5).Radius(999).Background(color)
}

func hline(c *ui.Context, color ui.Color) ui.Element {
	return ui.Box(c).Height(1).FillWidth().Background(color)
}

func vline(c *ui.Context, color ui.Color) ui.Element {
	return ui.Box(c).Width(1).FillHeight().Background(color)
}

func sectionLabel(c *ui.Context, title, tail string) {
	ui.Row(c).AlignItems(ui.Center).PaddingY(4).Children(func() {
		label(c, title, 10, pal.muted, 600)
		ui.Box(c).Grow(1)
		if tail != "" {
			mono(c, tail, 9, pal.faint)
		}
	})
}

func iconButton(c *ui.Context, glyph, labelText string, onClick func()) ui.Element {
	b := ui.ButtonBase(c).Size(22, 22).Center().Radius(6).Label(labelText).FocusRing(false)
	if b.Hovered() {
		b.Background(pal.sel)
	}
	if onClick != nil {
		b.OnClick(onClick)
	}
	b.Children(func() {
		text(c, glyph, 15, pal.muted)
	})
	return b
}
