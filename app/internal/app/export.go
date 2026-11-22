package app

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	videxport "mygo-uke/internal/export"
	"mygo-uke/internal/project"
)

func (a *app) startExport(c *ui.Context) {
	if a.exportRunning {
		return
	}
	if a.proj == nil || a.proj.Source == "" {
		a.toast(c, "无法导出", "这个工程没有源视频")
		return
	}
	source := expandHome(a.proj.Source)
	if _, err := os.Stat(source); err != nil {
		a.toast(c, "无法导出", "示例工程没有源文件，请先导入视频")
		return
	}
	name := exportName(a.proj.Title, a.orientation)
	orientation := a.orientation
	writeSRT := a.exportSRT
	p := a.proj

	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      a.win,
			Title:       "导出成片",
			DefaultPath: name,
			Filters:     []mygo.FileFilter{{Name: "MP4 视频", Extensions: []string{"mp4"}}},
		})
		if err != nil {
			mygo.RunOnMain(func() {
				a.exportErr = err.Error()
				a.modal = modalExport
				a.invalidate()
			})
			return
		}
		if path == "" {
			return
		}
		if !strings.HasSuffix(strings.ToLower(path), ".mp4") {
			path += ".mp4"
		}
		mygo.RunOnMain(func() {
			a.exportRunning = true
			a.exportProgress = 0
			a.exportErr = ""
			a.exportPath = path
			a.exportNote = ""
			a.modal = modalExport
			a.invalidate()
			go a.runExport(p, source, path, orientation, writeSRT)
		})
	}()
}

func (a *app) runExport(
	p *project.Project,
	source, path string,
	orientation int,
	writeSRT bool,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	last := -1.0
	err := videxport.Run(
		ctx, p, source, path, orientation,
		func(value float64) {
			if value < 1 && value-last < 0.01 {
				return
			}
			last = value
			mygo.RunOnMain(func() {
				a.exportProgress = value
				a.invalidate()
			})
		},
	)
	mygo.RunOnMain(func() {
		a.exportRunning = false
		if err != nil {
			a.exportErr = err.Error()
		} else {
			a.exportProgress = 1
			if writeSRT {
				srtPath := videxport.SRTName(path)
				if srtErr := videxport.WriteSRT(p, srtPath); srtErr != nil {
					a.exportNote = "视频已导出；SRT 写入失败：" + srtErr.Error()
				} else if len(p.Lines) > 0 {
					a.exportNote = "SRT 已写入 " + srtPath
				} else {
					a.exportNote = "工程无字幕，未生成 SRT"
				}
			}
		}
		a.invalidate()
	})
}
