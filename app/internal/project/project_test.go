package project

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mygo-uke/internal/score"
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

func TestMergeKeepsEdits(t *testing.T) {
	edited := New("我的歌")
	edited.Capo = 3
	edited.Pattern = "waltz-3"
	edited.Style.Accent = "#B5482E"
	edited.LinesEdited = true
	edited.Lines = []Line{{Start: 0, End: 3, Text: "改过的字幕"}}
	edited.ChartImage = "chart.png"
	edited.ChartName = "曲谱.png"
	edited.ChartAt = "2026-11-23T10:00:00+08:00"
	edited.Bars = []score.Bar{
		{Start: 0, End: 3, Chords: []string{"G"}, Manual: true},
		{Start: 3, End: 6, Chords: []string{"Em"}},
	}

	built := New("自动标题")
	built.Tempo = 120
	built.Lines = []Line{{Start: 0, End: 3, Text: "ASR 原句"}}
	built.Bars = []score.Bar{
		{Start: 0, End: 3, Chords: []string{"C"}},
		{Start: 3, End: 6, Chords: []string{"Am"}},
	}

	merged := Merge(edited, built)
	if merged.Title != "我的歌" || merged.Capo != 3 || merged.Pattern != "waltz-3" {
		t.Fatalf("merged = %+v", merged)
	}
	if merged.Style.Accent != "#B5482E" {
		t.Fatalf("accent = %q", merged.Style.Accent)
	}
	if merged.ChartImage != "chart.png" || merged.ChartName != "曲谱.png" ||
		merged.ChartAt != "2026-11-23T10:00:00+08:00" {
		t.Fatalf("chart = %+v", merged)
	}
	if len(merged.Lines) != 1 || merged.Lines[0].Text != "改过的字幕" {
		t.Fatalf("lines = %+v", merged.Lines)
	}
	if merged.Bars[0].Chords[0] != "G" || !merged.Bars[0].Manual {
		t.Fatalf("manual bar = %+v", merged.Bars[0])
	}
	if merged.Bars[1].Chords[0] != "Am" {
		t.Fatalf("fresh bar = %+v", merged.Bars[1])
	}
}

func TestMergeUsesFreshLinesWithoutEdits(t *testing.T) {
	edited := New("歌")
	edited.Lines = []Line{{Start: 0, End: 3, Text: "旧 ASR"}}
	built := New("歌")
	built.Lines = []Line{{Start: 0, End: 3, Text: "新 ASR"}}

	merged := Merge(edited, built)
	if merged.Lines[0].Text != "新 ASR" {
		t.Fatalf("lines = %+v", merged.Lines)
	}
}

func TestDeleteRejectsForeignPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keep.txt")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(path); err == nil {
		t.Fatal("Delete accepted a non-project path")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFillsStyleDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.json")
	data := []byte(`{"version":1,"title":"旧工程","duration":10}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Style.ChordSize != 64 || loaded.Style.SubSize != 60 ||
		loaded.Style.SubtitleY != 78 || loaded.Style.Accent != "#F3B544" {
		t.Fatalf("style = %+v", loaded.Style)
	}
}
