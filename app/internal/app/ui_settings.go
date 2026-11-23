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
	sub := "火山 ASR 与曲谱模型；环境变量存在时优先使用环境变量"
	if a.settingsErr != "" {
		sub = a.settingsErr
	}
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(520).Gap(12).Children(func() {
			modalTitle(c, "设置", sub)
			settingsField(c, "火山 API Key", "留空则不跑语音转写", &a.settingsDraft.VolcanoAPIKey, true)
			ui.Row(c).Gap(8).Children(func() {
				settingsField(c, "资源 ID", "默认 2.0 小时版", &a.settingsDraft.VolcanoResourceID, false)
				settingsField(c, "模型", "bigmodel", &a.settingsDraft.VolcanoModel, false)
			})
			settingsField(c, "WebSocket", "留空使用官方 v3 端点", &a.settingsDraft.VolcanoWSURL, false)
			sectionLabel(c, "曲谱识别 · OpenAI 兼容", "")
			settingsField(c, "Base URL", "留空使用 OpenAI 官方端点", &a.settingsDraft.SheetBaseURL, false)
			ui.Row(c).Gap(8).Children(func() {
				settingsField(c, "API Key", "用于解析曲谱图片", &a.settingsDraft.SheetAPIKey, true)
				settingsField(c, "模型", "支持图片输入的模型", &a.settingsDraft.SheetModel, false)
			})
			text(c, "图片识别需要支持视觉输入的模型；纯文本模型只能用粘贴文本曲谱。", 9, pal.faint)
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
	draft.VolcanoAPIKey = strings.TrimSpace(draft.VolcanoAPIKey)
	draft.VolcanoResourceID = strings.TrimSpace(draft.VolcanoResourceID)
	draft.VolcanoWSURL = strings.TrimSpace(draft.VolcanoWSURL)
	draft.VolcanoModel = strings.TrimSpace(draft.VolcanoModel)
	draft.SheetBaseURL = strings.TrimSpace(draft.SheetBaseURL)
	draft.SheetAPIKey = strings.TrimSpace(draft.SheetAPIKey)
	draft.SheetModel = strings.TrimSpace(draft.SheetModel)
	a.settingsDraft = draft

	if draft.SheetBaseURL != "" {
		parsed, err := url.Parse(draft.SheetBaseURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			a.settingsErr = "曲谱 Base URL 要以 http:// 或 https:// 开头"
			return
		}
	}
	if err := draft.save(); err != nil {
		a.settingsErr = "无法保存设置：" + err.Error()
		return
	}

	a.settings = draft
	a.asrConfig = draft.asrConfig()
	a.settingsErr = ""
	a.modal = modalNone
	a.toast(c, "设置已保存", "下次创建工程或重新转写时生效")
}
