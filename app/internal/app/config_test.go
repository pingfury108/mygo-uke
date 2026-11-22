package app

import "testing"

func TestChorddBaseURLFromEnv(t *testing.T) {
	t.Setenv("MYGO_UKE_CHORDD", "http://192.168.31.251:8790/")

	if got := chorddBaseURL(); got != "http://192.168.31.251:8790/" {
		t.Fatalf("chorddBaseURL() = %q", got)
	}
}
