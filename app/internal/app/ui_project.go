package app

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/project"
)

func (a *app) openProjectModal(index int) {
	if index < 0 || index >= len(a.projects) {
		return
	}
	a.selectProject(index)
	a.projectIndex = index
	a.projectTitle = a.projects[index].title
	a.projectErr = ""
	a.projectConfirm = false
	a.modal = modalProject
}

func (a *app) projectModal(c *ui.Context) {
	open := true
	item := projectItem{}
	if a.projectIndex >= 0 && a.projectIndex < len(a.projects) {
		item = a.projects[a.projectIndex]
	}
	detail := "重命名只改工程标题，不改字幕、和弦和时间轴"
	if a.projectErr != "" {
		detail = a.projectErr
	}
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(480).Gap(12).Children(func() {
			modalTitle(c, "工程设置", detail)
			ui.Column(c).Gap(4).Children(func() {
				text(c, "工程名称", 9.5, pal.muted)
				ui.TextInput(c, &a.projectTitle).Label("工程名称").
					Placeholder("输入工程名称").FillWidth().AutoFocus()
			})
			if item.path != "" {
				mono(c, item.path, 8.5, pal.faint).SingleLine()
			}
			if a.proj != nil && a.proj.ChartName != "" {
				path := a.chartPath()
				ui.Row(c).Padding(8, 10, 8, 10).Radius(7).Background(pal.card).
					AlignItems(ui.Center).Gap(8).Children(func() {
					text(c, "曲谱", 10, pal.muted).Width(42)
					ui.Column(c).Grow(1).Children(func() {
						mono(c, a.proj.ChartName, 9.5, pal.text).SingleLine()
					})
					if path != "" {
						ghostButton(c, "预览", func() { _ = mygo.Shell.PreviewItem(path) })
					}
					ghostButton(c, "更换", func() {
						a.modal = modalNone
						a.openSheetModal()
					})
				})
			}
			if a.projectConfirm {
				ui.Row(c).Padding(8, 10, 8, 10).Radius(8).Background(pal.warnBG).
					AlignItems(ui.Center).Children(func() {
					dot(c, pal.warn)
					text(c, "再点一次删除，工程目录会一起移除", 9.5, pal.warn)
				})
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if item.path != "" {
					ghostButton(c, "删除", func() { a.deleteProject(c) })
				}
				ghostButton(c, "取消", func() { a.modal = modalNone })
				primaryButton(c, "保存名称", func() { a.saveProjectTitle(c) })
			})
		})
	})
	if !open {
		a.modal = modalNone
	}
}

func (a *app) saveProjectTitle(c *ui.Context) {
	title := strings.TrimSpace(a.projectTitle)
	if title == "" {
		a.projectErr = "工程名称不能为空"
		return
	}
	if a.projectIndex < 0 || a.projectIndex >= len(a.projects) {
		a.modal = modalNone
		return
	}
	item := &a.projects[a.projectIndex]
	item.title = title
	if item.data != nil {
		item.data.Title = title
		if item.path != "" {
			if err := project.Save(item.path, item.data); err != nil {
				a.projectErr = "保存工程失败：" + err.Error()
				return
			}
		}
	}
	a.projectErr = ""
	a.modal = modalNone
	a.toast(c, "工程已重命名", title)
}

func (a *app) deleteProject(c *ui.Context) {
	if !a.projectConfirm {
		a.projectConfirm = true
		return
	}
	if a.projectIndex < 0 || a.projectIndex >= len(a.projects) {
		a.modal = modalNone
		return
	}
	item := a.projects[a.projectIndex]
	if item.path != "" {
		if err := project.Delete(item.path); err != nil {
			a.projectErr = "删除工程失败：" + err.Error()
			return
		}
	}
	a.projects = append(a.projects[:a.projectIndex], a.projects[a.projectIndex+1:]...)
	a.projectConfirm = false
	a.modal = modalNone
	if len(a.projects) == 0 {
		a.projects = demoProjects()
		a.selectProject(0)
	} else {
		next := a.projectIndex
		if next >= len(a.projects) {
			next = len(a.projects) - 1
		}
		a.selectProject(next)
	}
	a.toast(c, "工程已删除", item.title)
}
