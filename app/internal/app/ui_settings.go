package app

import (
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *app) openSettings() {
	a.settingsDraft = a.settings
	a.settingsErr = ""
	a.modal = modalSettings
}

func (a *app) settingsModal(c *ui.Context) {
	open := true
	sub := "chordd 与火山 ASR；环境变量存在时优先使用环境变量"
	if a.settingsErr != "" {
		sub = a.settingsErr
	}
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(520).Gap(12).Children(func() {
			modalTitle(c, "设置与 API Key", sub)
			settingsField(c, "chordd 地址", "http://host:8790", &a.settingsDraft.ChorddURL, false)
			settingsField(c, "火山 API Key", "留空则不跑语音转写", &a.settingsDraft.VolcanoAPIKey, true)
			ui.Row(c).Gap(8).Children(func() {
				settingsField(c, "资源 ID", "默认 2.0 小时版", &a.settingsDraft.VolcanoResourceID, false)
				settingsField(c, "模型", "bigmodel", &a.settingsDraft.VolcanoModel, false)
			})
			settingsField(c, "WebSocket", "留空使用官方 v3 端点", &a.settingsDraft.VolcanoWSURL, false)
			mono(c, settingsPath(), 8.5, pal.faint).SingleLine()
			if a.settingsErr != "" {
				text(c, a.settingsErr, 9.5, pal.warn)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				ghostButton(c, "取消", func() { a.modal = modalNone })
				primaryButton(c, "保存设置", func() { a.saveSettings(c) })
			})
		})
	})
	if !open {
		a.modal = modalNone
	}
}

func settingsField(c *ui.Context, name, placeholder string, value *string, password bool) {
	ui.Column(c).Grow(1).Gap(4).Children(func() {
		text(c, name, 9.5, pal.muted)
		input := ui.TextInput(c, value).Placeholder(placeholder).Label(name).FillWidth()
		if password {
			input.Password()
		}
	})
}

func (a *app) saveSettings(c *ui.Context) {
	draft := a.settingsDraft
	draft.ChorddURL = strings.TrimSpace(draft.ChorddURL)
	draft.VolcanoAPIKey = strings.TrimSpace(draft.VolcanoAPIKey)
	draft.VolcanoResourceID = strings.TrimSpace(draft.VolcanoResourceID)
	draft.VolcanoWSURL = strings.TrimSpace(draft.VolcanoWSURL)
	draft.VolcanoModel = strings.TrimSpace(draft.VolcanoModel)
	a.settingsDraft = draft

	parsed, err := url.Parse(draft.ChorddURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		a.settingsErr = "chordd 地址要以 http:// 或 https:// 开头"
		return
	}
	if err := draft.save(); err != nil {
		a.settingsErr = "无法保存设置：" + err.Error()
		return
	}

	a.settings = draft
	a.chorddURL = draft.ChorddURL
	a.asrConfig = draft.asrConfig()
	a.settingsErr = ""
	a.chorddChecked = false
	a.chorddOnline = false
	a.chorddAnalyzer = ""
	a.modal = modalNone
	a.refreshChordd()
	a.toast(c, "设置已保存", "下次导入或重新分析时生效")
}
