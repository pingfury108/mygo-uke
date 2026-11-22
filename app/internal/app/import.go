package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/chordd"
	"mygo-uke/internal/pipeline"
	"mygo-uke/internal/project"
)

func (a *app) loadLibrary() {
	entries, err := project.List(project.Root())
	if err != nil || len(entries) == 0 {
		return
	}
	items := make([]projectItem, 0, len(entries))
	for i, entry := range entries {
		items = append(items, a.projectItem(entry.Project, entry.Path, i))
	}
	a.projects = items
	a.selected = 0
	a.proj = entries[0].Project
	a.projPath = entries[0].Path
	a.resetCursor()
	a.openPlayerFor(a.proj.Source)
}

func (a *app) projectItem(p *project.Project, path string, tone int) projectItem {
	chord := "C"
	if len(p.Chords) > 0 {
		chord = p.Chords[0].Label
	}
	return projectItem{
		title: p.Title,
		meta:  fmt.Sprintf("%s · %s 调", timeText(p.Duration), p.Key),
		state: "编辑中",
		chord: chord,
		tone:  tone % 3,
		path:  path,
		data:  p,
	}
}

func (a *app) startImport() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win,
			Title:  "导入弹唱视频",
			Filters: []mygo.FileFilter{{
				Name:       "视频或音频",
				Extensions: []string{"mov", "mp4", "m4v", "mkv", "mp3", "m4a", "wav", "flac"},
			}},
		})
		if err != nil {
			mygo.RunOnMain(func() {
				a.jobErr = err.Error()
				a.jobStage = "失败"
				a.modal = modalAnalysis
				a.invalidate()
			})
			return
		}
		if len(paths) == 0 {
			return
		}
		mygo.RunOnMain(func() { a.startAnalysis(paths[0], true) })
	}()
}

func (a *app) startReanalyze(c *ui.Context) {
	if a.jobRunning {
		return
	}
	if a.proj == nil || a.proj.Source == "" {
		a.toast(c, "无法重新分析", "这个工程没有源视频")
		return
	}
	source := expandHome(a.proj.Source)
	if _, err := os.Stat(source); err != nil {
		a.toast(c, "无法重新分析", "示例工程没有源文件，请先导入视频")
		return
	}
	a.startAnalysis(source, false)
}

func (a *app) startAnalysis(source string, importing bool) {
	if a.jobRunning {
		return
	}
	a.jobRunning = true
	a.jobErr = ""
	a.jobStage = "准备中"
	a.jobImport = importing
	a.jobSource = source
	a.analysis = 0
	a.modal = modalAnalysis
	a.invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		var transcriber pipeline.Transcriber
		if a.asrConfig.APIKey != "" {
			transcriber = pipeline.ASRTranscriber{Config: a.asrConfig}
		}
		built, path, err := pipeline.Import(
			ctx,
			source,
			chordd.New(a.chorddURL),
			pipeline.FFmpeg{},
			transcriber,
			project.Root(),
			func(progress pipeline.Progress) {
				mygo.RunOnMain(func() {
					a.analysis = progress.Value
					a.jobStage = progress.Stage
					a.invalidate()
				})
			},
		)
		mygo.RunOnMain(func() {
			a.jobRunning = false
			if err != nil {
				a.jobErr = err.Error()
				a.jobStage = "失败"
				a.analysis = 0
				a.invalidate()
				return
			}
			if !importing && a.proj != nil {
				built = project.Merge(a.proj, built)
				if a.projPath != "" {
					if saveErr := project.Save(a.projPath, built); saveErr != nil {
						a.jobErr = "保存重新分析结果失败：" + saveErr.Error()
						a.jobStage = "失败"
						a.analysis = 0
						a.invalidate()
						return
					}
					if path != a.projPath {
						_ = project.Delete(path)
					}
					path = a.projPath
				}
			}
			a.applyImported(built, path, importing)
			a.analysis = 1
			a.analysisAt = time.Now()
			a.jobStage = "完成"
			if len(built.Warnings) > 0 {
				a.jobStage = "完成 · 字幕未识别"
			}
			a.invalidate()
		})
	}()
}

func (a *app) applyImported(p *project.Project, path string, importing bool) {
	a.proj = p
	a.projPath = path
	a.resetCursor()
	a.openPlayerFor(p.Source)
	item := a.projectItem(p, path, 0)
	if importing || len(a.projects) == 0 {
		a.projects = append([]projectItem{item}, a.projects...)
		a.selected = 0
		return
	}
	index := a.selected
	if index < 0 || index >= len(a.projects) {
		index = 0
	}
	a.projects[index] = item
	a.selected = index
}

func (a *app) resetCursor() {
	a.playing = false
	a.playhead = 0
	if len(a.proj.Downbeats) > 0 {
		a.playhead = a.proj.Downbeats[0]
	}
	a.selectedLine = 0
	a.revealLine = 0
	a.revealBar = -1
	a.chordCursor = -1
	a.resetInspectorScroll()
	a.pattern = patternIndex(a.proj.Pattern)
	if len(a.proj.Chords) > 0 {
		a.chordChoice = a.proj.Chords[0].Label
	}
}

func (a *app) resetInspectorScroll() {
	for i := range a.inspectorScroll {
		a.inspectorScroll[i].X = 0
		a.inspectorScroll[i].Y = 0
	}
}

func (a *app) invalidate() {
	if a.win != nil {
		a.win.Invalidate()
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
