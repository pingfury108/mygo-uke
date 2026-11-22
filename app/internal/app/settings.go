package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"mygo-uke/internal/asr"
)

type settings struct {
	ChorddURL         string `json:"chorddUrl"`
	VolcanoAPIKey     string `json:"volcanoApiKey"`
	VolcanoResourceID string `json:"volcanoResourceId"`
	VolcanoWSURL      string `json:"volcanoWsUrl"`
	VolcanoModel      string `json:"volcanoModel"`
}

func settingsPath() string {
	if value := os.Getenv("MYGO_UKE_SETTINGS"); value != "" {
		return value
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "mygo-uke", "settings.json")
}

func defaultSettings() settings {
	key := os.Getenv("MYGO_UKE_VOLCANO_API_KEY")
	if key == "" {
		key = os.Getenv("VOLCANO_API_KEY")
	}
	return settings{
		ChorddURL:         chorddBaseURL(),
		VolcanoAPIKey:     key,
		VolcanoResourceID: envOr("MYGO_UKE_VOLCANO_RESOURCE_ID", ""),
		VolcanoWSURL:      envOr("MYGO_UKE_VOLCANO_WS_URL", ""),
		VolcanoModel:      envOr("MYGO_UKE_VOLCANO_MODEL", ""),
	}
}

func loadSettings() settings {
	s := defaultSettings()
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return s
	}
	var saved settings
	if json.Unmarshal(data, &saved) != nil {
		return s
	}
	if os.Getenv("MYGO_UKE_CHORDD") == "" && saved.ChorddURL != "" {
		s.ChorddURL = saved.ChorddURL
	}
	if os.Getenv("MYGO_UKE_VOLCANO_API_KEY") == "" &&
		os.Getenv("VOLCANO_API_KEY") == "" && saved.VolcanoAPIKey != "" {
		s.VolcanoAPIKey = saved.VolcanoAPIKey
	}
	if os.Getenv("MYGO_UKE_VOLCANO_RESOURCE_ID") == "" && saved.VolcanoResourceID != "" {
		s.VolcanoResourceID = saved.VolcanoResourceID
	}
	if os.Getenv("MYGO_UKE_VOLCANO_WS_URL") == "" && saved.VolcanoWSURL != "" {
		s.VolcanoWSURL = saved.VolcanoWSURL
	}
	if os.Getenv("MYGO_UKE_VOLCANO_MODEL") == "" && saved.VolcanoModel != "" {
		s.VolcanoModel = saved.VolcanoModel
	}
	return s
}

func (s settings) asrConfig() asr.Config {
	return asr.Config{
		APIKey:     s.VolcanoAPIKey,
		WSURL:      s.VolcanoWSURL,
		ResourceID: s.VolcanoResourceID,
		Model:      s.VolcanoModel,
	}
}

func (s settings) save() error {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
