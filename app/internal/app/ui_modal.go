package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/score"
	"mygo-uke/internal/uke"
)

func modalTitle(c *ui.Context, title, sub string) {
	ui.Column(c).Gap(4).Children(func() {
		label(c, title, 18, pal.text, 650)
		if sub != "" {
			text(c, sub, 10.5, pal.muted)
		}
	})
	ui.Box(c).Height(1).FillWidth().Background(pal.line)
}

func (a *app) exportModal(c *ui.Context) {
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(460).Gap(12).Children(func() {
			modalTitle(c, "导出成片", "字幕与曲谱直接烧进画面，不经过第二套渲染器")
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				text(c, "画幅", 10, pal.muted).Width(58)
				segment(c, []string{"竖屏 1080×1920", "横屏 1920×1080"}, &a.orientation)
			})
			exportRow(c, "视频", "H.264 · CRF 18 · yuv420p · faststart")
			exportRow(c, "音频", "AAC 192k · 原视频音轨")
			exportRow(c, "文件名", exportName(a.proj.Title, a.orientation))
			exportRow(c, "预计耗时", "约 4 分 20 秒")
			if a.exporting {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 10, 8, 10).
					Radius(8).Background(pal.warnBG).Children(func() {
					dot(c, pal.accent)
					text(c, "导出任务已创建，关闭窗口后继续执行", 10, pal.muted)
				})
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				ghostButton(c, "取消", func() { a.modal = modalNone })
				primaryButton(c, "开始导出", func() {
					a.exporting = true
					a.modal = modalNone
					a.toast(c, "已开始导出", exportName(a.proj.Title, a.orientation))
				})
			})
		})
	})
	if !open {
		a.modal = modalNone
	}
}

func exportRow(c *ui.Context, name, value string) {
	ui.Row(c).Padding(8, 10, 8, 10).Radius(7).Background(pal.card).
		AlignItems(ui.Center).Gap(8).Children(func() {
		text(c, name, 10, pal.muted).Width(58)
		mono(c, value, 9.5, pal.text).SingleLine()
	})
}

func exportName(title string, orientation int) string {
	if orientation == 1 {
		return title + "_横屏_1080p.mp4"
	}
	return title + "_竖屏_1080p.mp4"
}

func (a *app) chordModal(c *ui.Context) {
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(480).Gap(12).Children(func() {
			bar := scoreBarAt(a, a.chordBar)
			title := fmt.Sprintf("第 %d 小节", a.chordBar+1)
			modalTitle(c, title+" · 选择和弦", "点一下替换，谱面和时间轴会立刻重排")
			grid := ui.Grid(c).FillWidth().Columns(5).Gap(6)
			grid.Children(func() {
				for _, name := range []string{
					"C", "G", "Am", "Em", "F",
					"Dm", "G7", "Cmaj7", "Am7", "Em7",
					"D7", "Bb", "A", "E7", "C7",
				} {
					chord := name
					on := a.chordChoice == chord
					b := ui.ButtonBase(c).Height(34).Radius(7).Center().Label(chord).FocusRing(false)
					if on {
						b.Background(pal.accent)
					} else {
						b.Background(pal.card).Border(1, pal.line)
					}
					if b.Hovered() && !on {
						b.Background(pal.panel2)
					}
					b.OnClick(func() { a.chordChoice = chord })
					b.Children(func() {
						color := pal.text
						if on {
							color = pal.accentInk
						}
						label(c, chord, 11, color, 650)
					})
				}
			})
			ui.Row(c).Gap(14).AlignItems(ui.Center).Padding(10).Radius(9).
				Background(pal.card).Children(func() {
				preview := ui.Box(c).Size(96, 104)
				preview.Draw(func(p *ui.Painter, r ui.Rect) {
					if chord, ok := uke.Normalize(a.chordChoice); ok {
						if shape, ok := uke.Shape(chord, a.proj.Capo); ok {
							drawDiagram(p, r, shape)
							p.Text(r.X+r.W/2-18, r.Y+r.H-4, shape.Name, 9, pal.muted)
							return
						}
					}
					p.Text(r.X+r.W/2-4, r.Y+r.H/2, "?", 16, pal.faint)
				})
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					label(c, "当前："+firstChord(bar), 10, pal.muted, 500)
					text(c, "标准调弦 GCEA · Capo "+fmt.Sprintf("%d", a.proj.Capo), 9, pal.faint)
					text(c, "按形状调查表，画面同时显示实际调", 9, pal.faint)
				})
			})
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				ghostButton(c, "取消", func() { a.modal = modalNone })
				primaryButton(c, "应用和弦", func() {
					if a.chordBar >= 0 && a.chordBar < len(a.proj.Bars) {
						a.proj.Bars[a.chordBar].Chords = []string{a.chordChoice}
						a.proj.Bars[a.chordBar].NeedsReview = false
						a.toast(c, "和弦已更新", fmt.Sprintf("第 %d 小节 → %s", a.chordBar+1, a.chordChoice))
					}
					a.modal = modalNone
				})
			})
		})
	})
	if !open {
		a.modal = modalNone
	}
}

func scoreBarAt(a *app, i int) score.Bar {
	if i >= 0 && i < len(a.proj.Bars) {
		return a.proj.Bars[i]
	}
	return score.Bar{}
}

func (a *app) analysisModal(c *ui.Context) {
	open := true
	title := "分析和弦"
	detail := "抽取音轨 → beat_this / BTC → 小节"
	switch {
	case a.jobErr != "":
		title = "分析失败"
		detail = a.jobErr
	case a.jobRunning:
		title = "正在分析"
		detail = a.jobStage
	case a.analysis >= 1:
		title = "分析完成"
		detail = "工程已保存，可以逐句校对"
	}
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(460).Gap(12).Children(func() {
			modalTitle(c, title, detail)
			stageRow(c, "1", "抽取音轨", "ffmpeg · 128k mono mp3", a.analysis >= 0.16, "")
			stageRow(c, "2", "语音转写", "火山 ASR 待接入 · 可先手动填词", false, "待接入")
			stageRow(c, "3", "和弦分析", "BTC large-voca + beat_this", a.analysis >= 0.88, "")
			stageRow(c, "4", "小节对齐", fmt.Sprintf("%d bars", len(a.proj.Bars)), a.analysis >= 1, "")
			progress := a.analysis
			ui.Column(c).Gap(5).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					text(c, "总进度", 9.5, pal.muted)
					ui.Box(c).Grow(1)
					mono(c, fmt.Sprintf("%.0f%%", progress*100), 9.5, pal.text)
				})
				ui.Box(c).Height(6).FillWidth().Radius(3).Background(pal.line).
					Draw(func(p *ui.Painter, r ui.Rect) {
						p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * float32(progress), H: r.H}, pal.accent, 3)
					})
			})
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				switch {
				case a.jobRunning:
					text(c, "分析中，先别关窗口…", 9.5, pal.faint)
				case a.jobErr != "":
					ghostButton(c, "关闭", func() { a.modal = modalNone })
					primaryButton(c, "重试", func() { a.startAnalysis(a.jobSource, a.jobImport) })
				default:
					ghostButton(c, "关闭", func() { a.modal = modalNone })
					primaryButton(c, "完成", func() { a.modal = modalNone })
				}
			})
		})
	})
	if !open {
		a.modal = modalNone
	}
}

func stageRow(c *ui.Context, index, name, detail string, done bool, status string) {
	ui.Row(c).Gap(9).AlignItems(ui.Center).Children(func() {
		box := ui.Box(c).Size(20, 20).Radius(999).Center()
		if done {
			box.Background(pal.warnBG)
		} else {
			box.Background(pal.card).Border(1, pal.line)
		}
		box.Children(func() {
			if done {
				text(c, "✓", 10, pal.accent)
			} else {
				mono(c, index, 9, pal.faint)
			}
		})
		ui.Column(c).Gap(1).Children(func() {
			text(c, name, 10.5, pal.text)
			mono(c, detail, 8.5, pal.faint)
		})
		ui.Box(c).Grow(1)
		if done {
			text(c, "完成", 9, pal.good)
		} else if status != "" {
			text(c, status, 9, pal.faint)
		} else {
			text(c, "等待中", 9, pal.faint)
		}
	})
}
