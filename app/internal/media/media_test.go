package media

import "testing"

func TestParseDurationOutput(t *testing.T) {
	if got := parseDuration("12.345\n"); got != 12.345 {
		t.Fatalf("parseDuration = %v", got)
	}
}
