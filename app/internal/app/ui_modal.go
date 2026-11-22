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
	title := "导出成片"
	detail := "字幕与曲谱直接烧进画面，不经过第二套渲染器"
	switch {
	case a.exportErr != "":
		title = "导出失败"
		if a.exportErr == "已取消导出" {
			title = "导出已取消"
		}
		detail = a.exportErr
	case a.exportRunning:
		title = "正在导出"
		if a.exportStopping {
			title = "正在取消"
		}
		detail = "H.264 · AAC · 逐帧叠加 · 硬件编码优先"
	case a.exportProgress >= 1:
		title = "导出完成"
		detail = a.exportPath
	}
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(460).Gap(12).Children(func() {
			modalTitle(c, title, detail)
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				text(c, "画幅", 10, pal.muted).Width(58)
				segment(c, []string{"竖屏 1080×1920", "横屏 1920×1080"}, &a.orientation)
			})
			exportRow(c, "视频", "H.264 · GPU 优先，CPU 回退 · faststart")
			exportRow(c, "音频", "AAC 192k · 原视频音轨")
			exportRow(c, "文件名", exportName(a.proj.Title, a.orientation))
			exportRow(c, "预计耗时", a.exportEstimate())
			exportRow(c, "字幕", "已烧进画面 · 逐句")
			if a.exportRunning || a.exportProgress > 0 {
				value := "不额外写入"
				if a.exportSRT {
					value = "逐句 · 与视频同目录"
				}
				exportRow(c, "附加 SRT", value)
			} else {
				ui.Row(c).Padding(8, 10, 8, 10).Radius(7).Background(pal.card).
					AlignItems(ui.Center).Children(func() {
					text(c, "附加 SRT", 10, pal.muted).Width(58)
					mono(c, "可选 · 不改动已烧录画面", 9.5, pal.text)
					ui.Box(c).Grow(1)
					ui.Switch(c, &a.exportSRT)
				})
				if a.exportSRT && len(a.proj.Lines) == 0 {
					text(c, "工程还没有字幕，这次不会生成 SRT", 9, pal.faint)
				}
			}
			if a.exportRunning || a.exportProgress > 0 {
				progress := a.exportProgress
				ui.Column(c).Gap(5).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						text(c, "导出进度", 9.5, pal.muted)
						ui.Box(c).Grow(1)
						mono(c, fmt.Sprintf("%.0f%%", progress*100), 9.5, pal.text)
					})
					ui.Box(c).Height(6).FillWidth().Radius(3).Background(pal.line).
						Draw(func(p *ui.Painter, r ui.Rect) {
							p.Fill(
								ui.Rect{X: r.X, Y: r.Y, W: r.W * float32(progress), H: r.H},
								pal.accent, 3,
							)
						})
				})
			}
			if a.exportRunning {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 10, 8, 10).
					Radius(8).Background(pal.warnBG).Children(func() {
					dot(c, pal.accent)
					if a.exportStopping {
						text(c, "正在停止 ffmpeg 并清理未完成的文件…", 10, pal.muted)
					} else {
						text(c, "导出在后台执行，可以关闭这个窗口", 10, pal.muted)
					}
				})
			} else if a.exportErr != "" {
				text(c, a.exportErr, 9.5, pal.warn)
			} else if a.exportProgress >= 1 {
				mono(c, a.exportPath, 9, pal.good).SingleLine()
				if a.exportNote != "" {
					text(c, a.exportNote, 9, pal.muted)
				}
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				switch {
				case a.exportRunning:
					if a.exportStopping {
						text(c, "正在取消…", 9.5, pal.faint)
					} else {
						ghostButton(c, "取消导出", func() { a.cancelExport() })
					}
				case a.exportErr != "":
					ghostButton(c, "关闭", func() { a.modal = modalNone })
					primaryButton(c, "重试", func() { a.startExport(c) })
				case a.exportProgress >= 1:
					ghostButton(c, "关闭", func() { a.modal = modalNone })
					primaryButton(c, "完成", func() { a.modal = modalNone })
				default:
					ghostButton(c, "取消", func() { a.modal = modalNone })
					primaryButton(c, "开始导出", func() { a.startExport(c) })
				}
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
							drawDiagram(p, r, shape, a.overlayAccent())
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
						a.setBarChord(a.chordBar, a.chordChoice)
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
	title := "分析工程"
	detail := "抽取音轨 → 和弦 / 字幕并行 → 小节"
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
			asrDetail := "未配置 MYGO_UKE_VOLCANO_API_KEY · 跳过"
			asrStatus := "跳过"
			asrDone := false
			if a.asrConfig.APIKey != "" {
				asrDetail = "火山流式 ASR · 句级 utterances"
				asrStatus = "并行"
				asrDone = a.analysis >= 1
			}
			stageRow(c, "2", "语音转写", asrDetail, asrDone, asrStatus)
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
