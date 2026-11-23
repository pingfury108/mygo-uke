package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/sheet"
)

func (a *app) openNewProject() {
	a.cancelSheet()
	a.sheetSeq++
	a.sheetMode = sheetForNew
	a.sheetErr = ""
	a.sheetNote = ""
	a.sheetTab = 0
	a.sheetPaste = ""
	a.newVideo = ""
	a.newChart = nil
	a.newChartOrigin = chartOrigin{}
	a.modal = modalNewProject
}

func (a *app) newProjectModal(c *ui.Context) {
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(580).Gap(12).Children(func() {
			modalTitle(c, "新建工程", "一次选择视频和曲谱；字幕、逐字和弦与小节一起生成")
			a.newVideoCard(c)
			sectionLabel(c, "曲谱", "可选但推荐")
			segment(c, []string{"图片曲谱", "文本曲谱"}, &a.sheetTab)
			if a.sheetTab == 0 {
				a.sheetImagePane(c)
			} else {
				a.sheetTextPane(c)
			}
			if a.sheetRunning {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 10, 8, 10).
					Radius(8).Background(pal.card).Children(func() {
					dot(c, pal.accent)
					text(c, "正在识别曲谱图片，通常需要几秒到半分钟…", 10, pal.muted)
				})
			}
			if a.sheetNote != "" {
				text(c, a.sheetNote, 9.5, pal.good)
			}
			if a.sheetErr != "" {
				ui.Row(c).Padding(8, 10, 8, 10).Radius(8).Background(pal.warnBG).
					AlignItems(ui.Center).Children(func() {
					dot(c, pal.warn)
					text(c, a.sheetErr, 9.5, pal.warn)
				})
			}
			hint := "不导入曲谱也可以创建，但不会有和弦、BPM 和小节。"
			if a.pendingChartReady() {
				hint = "曲谱会和视频时间轴自动对齐；创建后仍可逐字修改和弦。"
			}
			if a.asrConfig.APIKey == "" {
				hint += " 未配置火山 ASR：曲谱行会按视频时长平均分布。"
			}
			text(c, hint, 9, pal.faint)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				ghostButton(c, "取消", a.closeSheet)
				primaryButton(c, "创建工程", a.createNewProject)
			})
		})
	})
	if !open {
		a.closeSheet()
	}
}

func (a *app) newVideoCard(c *ui.Context) {
	card := ui.ButtonBase(c).FillWidth().Height(66).Padding(10, 13, 10, 13).
		Radius(10).Background(pal.card).Border(1, pal.lineStrong).
		Label("选择视频").FocusRing(false)
	if card.Hovered() {
		card.Background(pal.sel)
	}
	card.OnClick(a.pickNewVideo)
	card.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Box(c).Size(38, 38).Radius(8).Background(pal.bg2).Center().Children(func() {
				text(c, "▶", 15, pal.accent)
			})
			ui.Column(c).Grow(1).Gap(3).Children(func() {
				if a.newVideo == "" {
					label(c, "选择弹唱视频", 12, pal.text, 600)
					text(c, "MP4 / MOV / M4A / WAV · 也可以直接拖进窗口", 9, pal.muted)
				} else {
					label(c, filepath.Base(a.newVideo), 12, pal.text, 600).SingleLine()
					text(c, "点击更换视频", 9, pal.accent)
				}
			})
		})
	})
}

func (a *app) pickNewVideo() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win,
			Title:  "选择弹唱视频",
			Filters: []mygo.FileFilter{{
				Name:       "视频或音频",
				Extensions: []string{"mov", "mp4", "m4v", "mkv", "mp3", "m4a", "wav", "flac"},
			}},
		})
		if err != nil {
			mygo.RunOnMain(func() {
				a.sheetErr = err.Error()
				a.invalidate()
			})
			return
		}
		if len(paths) == 0 {
			return
		}
		mygo.RunOnMain(func() {
			a.newVideo = paths[0]
			a.invalidate()
		})
	}()
}

func (a *app) createNewProject() {
	if a.jobRunning {
		return
	}
	if a.sheetRunning {
		a.sheetErr = "曲谱还在识别，请稍候"
		return
	}
	video := strings.TrimSpace(a.newVideo)
	if video == "" {
		a.sheetErr = "先选择视频"
		return
	}
	chart, origin, err := a.pendingChart()
	if err != nil {
		a.sheetErr = err.Error()
		return
	}
	a.sheetErr = ""
	a.startJob(video, true, chart, origin)
}

func (a *app) pendingChartReady() bool {
	if a.sheetTab == 0 {
		return a.newChart != nil
	}
	return strings.TrimSpace(a.sheetPaste) != "" || a.newChart != nil
}

func (a *app) pendingChart() (*sheet.Sheet, chartOrigin, error) {
	if a.sheetTab == 0 {
		if a.newChart != nil {
			return a.newChart, a.newChartOrigin, nil
		}
		if a.newChartOrigin.Name != "" {
			return nil, chartOrigin{}, fmt.Errorf("曲谱图片还没有解析成功，请重新选择")
		}
		return nil, chartOrigin{}, nil
	}
	text := strings.TrimSpace(a.sheetPaste)
	if text == "" {
		if a.newChart != nil {
			return a.newChart, a.newChartOrigin, nil
		}
		return nil, chartOrigin{}, nil
	}
	parsed, err := sheet.ParseText(text)
	if err != nil {
		return nil, chartOrigin{}, err
	}
	return parsed, chartOrigin{Name: "粘贴文本", Text: text}, nil
}

func isVideoFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mov", ".mp4", ".m4v", ".mkv", ".mp3", ".m4a", ".wav", ".flac":
		return true
	}
	return false
}
