package app

import "testing"

func TestChorddBaseURLFromEnv(t *testing.T) {
	t.Setenv("MYGO_UKE_CHORDD", "http://192.168.31.251:8790/")

	if got := chorddBaseURL(); got != "http://192.168.31.251:8790/" {
		t.Fatalf("chorddBaseURL() = %q", got)
	}
}

func TestVolcanoConfigFromEnv(t *testing.T) {
	t.Setenv("MYGO_UKE_VOLCANO_API_KEY", "key")
	t.Setenv("MYGO_UKE_VOLCANO_RESOURCE_ID", "resource")
	cfg := volcanoConfig()
	if cfg.APIKey != "key" || cfg.ResourceID != "resource" {
		t.Fatalf("config = %+v", cfg)
	}
}
