package project

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadAndList(t *testing.T) {
	root := t.TempDir()
	first := New("夏夜来信")
	first.Duration = 198
	firstPath := filepath.Join(NewDir(root, first.Title), "project.json")
	if err := Save(firstPath, first); err != nil {
		t.Fatal(err)
	}
	second := New("晴天")
	secondPath := filepath.Join(NewDir(root, second.Title), "project.json")
	if err := Save(secondPath, second); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Dir(secondPath), future, future); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Title != "夏夜来信" || loaded.Duration != 198 {
		t.Fatalf("loaded = %+v", loaded)
	}

	entries, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d", len(entries))
	}
	if entries[0].Project.Title != "晴天" {
		t.Fatalf("newest = %q", entries[0].Project.Title)
	}
}

func TestListMissingLibraryIsEmpty(t *testing.T) {
	entries, err := List(filepath.Join(t.TempDir(), "missing"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("entries = %v, err = %v", entries, err)
	}
}

func TestSetLineUpsertsAndSorts(t *testing.T) {
	p := New("字幕")
	p.Lines = []Line{
		{Start: 5, End: 8, Text: "第二句"},
		{Start: 10, End: 12, Text: "第三句"},
	}
	if got := p.SetLine(1, Line{Start: 1, End: 2, Text: "第一句"}); got != 0 {
		t.Fatalf("updated index = %d", got)
	}
	if len(p.Lines) != 2 || p.Lines[0].Text != "第一句" || p.Lines[1].Text != "第二句" {
		t.Fatalf("lines = %#v", p.Lines)
	}
	if got := p.SetLine(9, Line{Start: 13, End: 14, Text: "第四句"}); got != 2 {
		t.Fatalf("appended index = %d", got)
	}
	if !p.DeleteLine(1) || len(p.Lines) != 2 || p.Lines[1].Text != "第四句" {
		t.Fatalf("delete = %#v", p.Lines)
	}
	if p.DeleteLine(9) {
		t.Fatal("out-of-range delete should fail")
	}
}
