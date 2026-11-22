package subtitle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormat(t *testing.T) {
	got := Format([]Cue{
		{Start: 1.25, End: 3.5, Text: "第一句"},
		{Start: 3.5, End: 4, Text: "  "},
		{Start: 4, End: 4, Text: "空"},
		{Start: 3661.001, End: 3662.002, Text: "第二句"},
	})
	want := "1\n00:00:01,250 --> 00:00:03,500\n第一句\n\n" +
		"2\n01:01:01,001 --> 01:01:02,002\n第二句\n\n"
	if got != want {
		t.Fatalf("Format() =\n%q\nwant\n%q", got, want)
	}
}

func TestWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "song.srt")
	if err := Write(path, []Cue{{Start: 0, End: 1, Text: "你好"}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "1\n00:00:00,000 --> 00:00:01,000\n你好\n\n" {
		t.Fatalf("file = %q", data)
	}
}
