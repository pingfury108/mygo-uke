package uke

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"C", "C", true},
		{"D#m", "Ebm", true},
		{"C/G", "C", true},
		{"Cmaj9", "Cmaj7", true},
		{"G/B", "G", true},
		{"Am7", "Am7", true},
		{"N.C.", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := Normalize(tt.in)
		if ok != tt.ok {
			t.Fatalf("Normalize(%q) ok = %v, want %v", tt.in, ok, tt.ok)
		}
		if ok && got.Name() != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.in, got.Name(), tt.want)
		}
	}
}

func TestShapeWithCapo(t *testing.T) {
	c, ok := Normalize("D")
	if !ok {
		t.Fatal("D did not normalize")
	}
	s, ok := Shape(c, 2)
	if !ok {
		t.Fatal("D with capo 2 has no shape")
	}
	if s.Name != "C" {
		t.Errorf("D with capo 2 uses shape %q, want C", s.Name)
	}
	if s.Frets != [4]int{0, 0, 0, 3} {
		t.Errorf("D with capo 2 = %v, want C shape", s.Frets)
	}
}

func TestKnownShapes(t *testing.T) {
	tests := map[string][4]int{
		"C":  {0, 0, 0, 3},
		"G":  {0, 2, 3, 2},
		"Am": {2, 0, 0, 0},
		"Em": {0, 4, 3, 2},
		"F":  {2, 0, 1, 0},
		"Dm": {2, 2, 1, 0},
		"G7": {0, 2, 1, 2},
	}
	for label, want := range tests {
		c, ok := Normalize(label)
		if !ok {
			t.Fatalf("%s did not normalize", label)
		}
		s, ok := Shape(c, 0)
		if !ok {
			t.Fatalf("%s has no shape", label)
		}
		if s.Frets != want {
			t.Errorf("%s = %v, want %v", label, s.Frets, want)
		}
	}
}
