package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultSettingsFromEnv(t *testing.T) {
	t.Setenv("MYGO_UKE_VOLCANO_API_KEY", "key")
	t.Setenv("MYGO_UKE_VOLCANO_RESOURCE_ID", "resource")
	t.Setenv("MYGO_UKE_VOLCANO_MODEL", "bigmodel")
	t.Setenv("MYGO_UKE_SHEET_BASE_URL", "https://llm.example.test/v1")
	t.Setenv("MYGO_UKE_SHEET_API_KEY", "sheet-key")
	t.Setenv("MYGO_UKE_SHEET_MODEL", "vision-model")
	cfg := defaultSettings().asrConfig()
	if cfg.APIKey != "key" || cfg.ResourceID != "resource" {
		t.Fatalf("config = %+v", cfg)
	}
	if cfg.Model != "bigmodel" {
		t.Fatalf("model = %q", cfg.Model)
	}
	if cfg.EndWindowSize != asrSingingEndWindowMS {
		t.Fatalf("end window = %d, want %d", cfg.EndWindowSize, asrSingingEndWindowMS)
	}
	sheetCfg := defaultSettings().sheetConfig()
	if sheetCfg.BaseURL != "https://llm.example.test/v1" ||
		sheetCfg.APIKey != "sheet-key" || sheetCfg.Model != "vision-model" {
		t.Fatalf("sheet config = %+v", sheetCfg)
	}
}

func TestASREndWindowOverride(t *testing.T) {
	t.Setenv("MYGO_UKE_ASR_END_WINDOW", "2600")
	if got := defaultSettings().asrConfig().EndWindowSize; got != 2600 {
		t.Fatalf("end window = %d, want 2600", got)
	}
}

func TestSheetSettingsFallBackToOpenAIEnv(t *testing.T) {
	t.Setenv("MYGO_UKE_SHEET_BASE_URL", "")
	t.Setenv("MYGO_UKE_SHEET_API_KEY", "")
	t.Setenv("MYGO_UKE_SHEET_MODEL", "vision-model")
	t.Setenv("OPENAI_BASE_URL", "https://openai.example.test/v1")
	t.Setenv("OPENAI_API_KEY", "openai-key")

	cfg := defaultSettings().sheetConfig()
	if cfg.BaseURL != "https://openai.example.test/v1" || cfg.APIKey != "openai-key" ||
		cfg.Model != "vision-model" {
		t.Fatalf("sheet config = %+v", cfg)
	}
}

func TestSaveAndLoadSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	t.Setenv("MYGO_UKE_SETTINGS", path)
	for _, name := range []string{
		"MYGO_UKE_VOLCANO_API_KEY",
		"VOLCANO_API_KEY",
		"MYGO_UKE_VOLCANO_RESOURCE_ID",
		"MYGO_UKE_VOLCANO_WS_URL",
		"MYGO_UKE_VOLCANO_MODEL",
		"MYGO_UKE_SHEET_BASE_URL",
		"OPENAI_BASE_URL",
		"MYGO_UKE_SHEET_API_KEY",
		"OPENAI_API_KEY",
		"MYGO_UKE_SHEET_MODEL",
	} {
		t.Setenv(name, "")
	}

	saved := settings{
		VolcanoAPIKey:     "test-key",
		VolcanoResourceID: "test-resource",
		VolcanoWSURL:      "wss://example.test/asr",
		VolcanoModel:      "bigmodel",
		SheetBaseURL:      "https://llm.example.test/v1",
		SheetAPIKey:       "sheet-key",
		SheetModel:        "vision-model",
	}
	if err := saved.save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("settings mode = %o", info.Mode().Perm())
	}

	loaded := loadSettings()
	if loaded != saved {
		t.Fatalf("loaded = %+v, want %+v", loaded, saved)
	}
}

func TestEnvOverridesSavedSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	t.Setenv("MYGO_UKE_SETTINGS", path)
	t.Setenv("MYGO_UKE_VOLCANO_API_KEY", "")
	t.Setenv("VOLCANO_API_KEY", "")
	t.Setenv("MYGO_UKE_VOLCANO_RESOURCE_ID", "")
	t.Setenv("MYGO_UKE_VOLCANO_WS_URL", "")
	t.Setenv("MYGO_UKE_VOLCANO_MODEL", "")
	t.Setenv("MYGO_UKE_SHEET_BASE_URL", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("MYGO_UKE_SHEET_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("MYGO_UKE_SHEET_MODEL", "")
	if err := (settings{
		VolcanoAPIKey: "saved",
		SheetBaseURL:  "https://saved.test/v1",
		SheetAPIKey:   "saved-sheet",
		SheetModel:    "saved-model",
	}).save(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MYGO_UKE_VOLCANO_API_KEY", "env-key")
	t.Setenv("MYGO_UKE_SHEET_BASE_URL", "https://env.test/v1")
	t.Setenv("MYGO_UKE_SHEET_API_KEY", "env-sheet")
	t.Setenv("MYGO_UKE_SHEET_MODEL", "env-model")
	loaded := loadSettings()
	if loaded.VolcanoAPIKey != "env-key" {
		t.Fatalf("loaded = %+v", loaded)
	}
	sheetCfg := loaded.sheetConfig()
	if sheetCfg.BaseURL != "https://env.test/v1" || sheetCfg.APIKey != "env-sheet" ||
		sheetCfg.Model != "env-model" {
		t.Fatalf("sheet config = %+v", sheetCfg)
	}
}
