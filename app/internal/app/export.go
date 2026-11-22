package app

import (
	"context"
	"errors"
	"fmt"
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
			a.exportETA = 0
			a.exportStopping = false
			a.exportErr = ""
			a.exportPath = path
			a.exportNote = ""
			a.modal = modalExport
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			a.exportCancel = cancel
			a.invalidate()
			go a.runExport(ctx, p, source, path, orientation, writeSRT)
		})
	}()
}

func (a *app) runExport(
	ctx context.Context,
	p *project.Project,
	source, path string,
	orientation int,
	writeSRT bool,
) {
	started := time.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(path)
		}
	}()
	last := -1.0
	err := videxport.Run(
		ctx, p, source, path, orientation,
		func(value float64) {
			if value < 1 && value-last < 0.01 {
				return
			}
			last = value
			eta := time.Duration(0)
			if value > 0.01 && value < 1 {
				eta = time.Duration(float64(time.Since(started)) * (1 - value) / value)
			}
			mygo.RunOnMain(func() {
				a.exportProgress = value
				a.exportETA = eta
				a.invalidate()
			})
		},
	)
	ok = err == nil
	mygo.RunOnMain(func() {
		a.exportRunning = false
		a.exportETA = 0
		a.exportCancel = nil
		a.exportStopping = false
		if err != nil {
			a.exportProgress = 0
			if errors.Is(err, context.Canceled) {
				a.exportErr = "已取消导出"
			} else {
				a.exportErr = err.Error()
			}
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
			} else if len(p.Lines) > 0 {
				a.exportNote = "字幕与曲谱已烧进视频"
			}
		}
		a.invalidate()
	})
}

func (a *app) cancelExport() {
	if !a.exportRunning || a.exportCancel == nil {
		return
	}
	a.exportStopping = true
	a.exportCancel()
}

func (a *app) exportEstimate() string {
	if a.exportRunning && a.exportETA > 0 {
		return "约 " + durationWords(a.exportETA) + " · 实时"
	}
	if a.proj == nil || a.proj.Duration <= 0 {
		return "按导出进度更新"
	}
	factor := 0.7
	if a.orientation == 1 {
		factor = 0.6
	}
	return "约 " + durationWords(time.Duration(a.proj.Duration*factor*float64(time.Second)))
}

func durationWords(d time.Duration) string {
	seconds := int(d.Round(time.Second).Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%d 秒", seconds)
	}
	return fmt.Sprintf("%d 分 %d 秒", seconds/60, seconds%60)
}
