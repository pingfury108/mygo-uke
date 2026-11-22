package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChorddBaseURLFromEnv(t *testing.T) {
	t.Setenv("MYGO_UKE_CHORDD", "http://192.168.31.251:8790/")

	if got := chorddBaseURL(); got != "http://192.168.31.251:8790/" {
		t.Fatalf("chorddBaseURL() = %q", got)
	}
}

func TestDefaultSettingsFromEnv(t *testing.T) {
	t.Setenv("MYGO_UKE_VOLCANO_API_KEY", "key")
	t.Setenv("MYGO_UKE_VOLCANO_RESOURCE_ID", "resource")
	t.Setenv("MYGO_UKE_VOLCANO_MODEL", "bigmodel")
	cfg := defaultSettings().asrConfig()
	if cfg.APIKey != "key" || cfg.ResourceID != "resource" {
		t.Fatalf("config = %+v", cfg)
	}
	if cfg.Model != "bigmodel" {
		t.Fatalf("model = %q", cfg.Model)
	}
}

func TestSaveAndLoadSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	t.Setenv("MYGO_UKE_SETTINGS", path)
	for _, name := range []string{
		"MYGO_UKE_CHORDD",
		"MYGO_UKE_VOLCANO_API_KEY",
		"VOLCANO_API_KEY",
		"MYGO_UKE_VOLCANO_RESOURCE_ID",
		"MYGO_UKE_VOLCANO_WS_URL",
		"MYGO_UKE_VOLCANO_MODEL",
	} {
		t.Setenv(name, "")
	}

	saved := settings{
		ChorddURL:         "http://127.0.0.1:8790",
		VolcanoAPIKey:     "test-key",
		VolcanoResourceID: "test-resource",
		VolcanoWSURL:      "wss://example.test/asr",
		VolcanoModel:      "bigmodel",
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
	t.Setenv("MYGO_UKE_CHORDD", "")
	t.Setenv("MYGO_UKE_VOLCANO_API_KEY", "")
	t.Setenv("VOLCANO_API_KEY", "")
	t.Setenv("MYGO_UKE_VOLCANO_RESOURCE_ID", "")
	t.Setenv("MYGO_UKE_VOLCANO_WS_URL", "")
	t.Setenv("MYGO_UKE_VOLCANO_MODEL", "")
	if err := (settings{ChorddURL: "http://saved.test", VolcanoAPIKey: "saved"}).save(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MYGO_UKE_CHORDD", "http://env.test")
	t.Setenv("MYGO_UKE_VOLCANO_API_KEY", "env-key")
	loaded := loadSettings()
	if loaded.ChorddURL != "http://env.test" || loaded.VolcanoAPIKey != "env-key" {
		t.Fatalf("loaded = %+v", loaded)
	}
}
