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

	"mygo-uke/internal/pipeline"
	"mygo-uke/internal/project"
	"mygo-uke/internal/sheet"
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
	meta := fmt.Sprintf("%s · %s 调", timeText(p.Duration), p.Key)
	if p.ChartName != "" {
		meta += " · 有谱"
	}
	return projectItem{
		title: p.Title,
		meta:  meta,
		state: "编辑中",
		chord: chord,
		tone:  tone % 3,
		path:  path,
		data:  p,
	}
}

func (a *app) startImport() {
	a.openNewProject()
}

func (a *app) startReanalyze(c *ui.Context) {
	if a.jobRunning {
		return
	}
	if a.proj == nil || a.proj.Source == "" {
		a.toast(c, "无法重新转写", "这个工程没有源视频")
		return
	}
	if a.asrConfig.APIKey == "" {
		a.toast(c, "无法重新转写", "先在设置里配置火山 ASR Key")
		return
	}
	source := expandHome(a.proj.Source)
	if _, err := os.Stat(source); err != nil {
		a.toast(c, "无法重新转写", "示例工程没有源文件，请先导入视频")
		return
	}
	chart, restore := a.chartForReanalyze()
	a.startJob(source, false, chart, chartOrigin{}, restore)
}

// chartForReanalyze prefers the stored chart snapshot. Projects created
// before chartLines existed only kept the lyric lines that matched the old
// transcript, so the opening line of the chart may be missing; re-parsing the
// saved text or image rebuilds it before the new transcript is aligned.
func (a *app) chartForReanalyze() (*sheet.Sheet, chartOrigin) {
	chart := sheet.FromProject(a.proj)
	if a.proj == nil || a.chartSnapshotComplete() {
		return chart, chartOrigin{}
	}
	if text := strings.TrimSpace(a.proj.ChartText); text != "" {
		if parsed, err := sheet.ParseText(text); err == nil && len(parsed.Lines) > 0 {
			return parsed, chartOrigin{}
		}
	}
	if path := a.chartPath(); path != "" {
		return chart, chartOrigin{Name: a.proj.ChartName, Path: path}
	}
	return chart, chartOrigin{}
}

func (a *app) chartSnapshotComplete() bool {
	if a.proj == nil || len(a.proj.ChartLines) == 0 {
		return false
	}
	return a.proj.ChartSnapshot >= project.ChartSnapshotVersion
}

func (a *app) startJob(
	source string,
	importing bool,
	chart *sheet.Sheet,
	origin chartOrigin,
	restore chartOrigin,
) {
	if a.jobRunning {
		return
	}
	a.jobRunning = true
	a.jobErr = ""
	a.jobStage = "准备中"
	a.jobImport = importing
	a.jobSource = source
	a.jobChart = chart
	a.jobOrigin = origin
	a.jobRestore = restore
	a.analysis = 0
	a.modal = modalAnalysis
	a.invalidate()

	sheetCfg := a.settings.sheetConfig()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		var transcriber pipeline.Transcriber
		if a.asrConfig.APIKey != "" {
			transcriber = pipeline.ASRTranscriber{Config: a.asrConfig}
		}
		restoreErr := ""
		if !importing && chartOriginReady(restore) {
			mygo.RunOnMain(func() {
				a.jobStage = "恢复曲谱"
				a.invalidate()
			})
			parsed, err := parseChartOrigin(ctx, restore, sheetCfg)
			switch {
			case err != nil:
				restoreErr = err.Error()
			case parsed != nil && len(parsed.Lines) > 0:
				chart = parsed
				mygo.RunOnMain(func() {
					a.jobChart = parsed
					a.invalidate()
				})
			}
		}
		built, path, err := pipeline.Import(
			ctx,
			source,
			pipeline.FFmpeg{},
			transcriber,
			chart,
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
				built = mergeAfterReanalyze(a.proj, built)
				if restoreErr != "" {
					// Keep the failed snapshot eligible so a later run retries
					// the stored chart instead of accepting the partial lines.
					built.ChartSnapshot = 0
					built.Warnings = append(built.Warnings, "曲谱快照恢复失败："+restoreErr)
				}
				if a.projPath != "" {
					if saveErr := project.Save(a.projPath, built); saveErr != nil {
						a.jobErr = "保存重新转写结果失败：" + saveErr.Error()
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
			if importing && chart != nil {
				warning := a.storeChartFor(built, path, origin)
				if warning != "" {
					built.Warnings = append(built.Warnings, warning)
				}
				if saveErr := project.Save(path, built); saveErr != nil {
					a.jobErr = "保存曲谱失败：" + saveErr.Error()
					a.jobStage = "失败"
					a.analysis = 0
					a.invalidate()
					return
				}
			}
			a.applyImported(built, path, importing)
			a.analysis = 1
			a.analysisAt = time.Now()
			a.jobStage = "完成"
			if len(built.Warnings) > 0 {
				a.jobStage = "完成 · 请检查提示"
			}
			if importing {
				a.newVideo = ""
				a.newChart = nil
				a.newChartOrigin = chartOrigin{}
				a.sheetPaste = ""
				a.sheetMode = sheetForCurrent
			}
			a.invalidate()
		})
	}()
}

// mergeAfterReanalyze prefers the fresh ASR timing, but keeps the old project
// when transcription failed and the chart only produced fallback spacing.
func mergeAfterReanalyze(edited, built *project.Project) *project.Project {
	if built != nil {
		for _, warning := range built.Warnings {
			if strings.HasPrefix(warning, "字幕识别失败") {
				return project.Merge(edited, built)
			}
		}
	}
	return project.MergeReanalyze(edited, built)
}

func (a *app) retryJob() {
	a.startJob(a.jobSource, a.jobImport, a.jobChart, a.jobOrigin, a.jobRestore)
}

func chartOriginReady(origin chartOrigin) bool {
	return origin.Text != "" || len(origin.Data) > 0 || origin.Path != ""
}

// parseChartOrigin rebuilds a chart from the copy stored in the project.
func parseChartOrigin(ctx context.Context, origin chartOrigin, cfg sheet.Config) (*sheet.Sheet, error) {
	switch {
	case origin.Text != "":
		return sheet.ParseText(origin.Text)
	case len(origin.Data) > 0:
		if !cfg.Ready() {
			return nil, fmt.Errorf("图片识别还没有配置，请到「设置」完成配置")
		}
		client, err := sheet.New(ctx, cfg)
		if err != nil {
			return nil, err
		}
		return client.ParseImageData(ctx, origin.Data, "image/png")
	case origin.Path != "":
		if !cfg.Ready() {
			return nil, fmt.Errorf("图片识别还没有配置，请到「设置」完成配置")
		}
		client, err := sheet.New(ctx, cfg)
		if err != nil {
			return nil, err
		}
		return client.ParseImage(ctx, origin.Path)
	}
	return nil, fmt.Errorf("没有可恢复的曲谱")
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
	a.resetChartThumb()
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
