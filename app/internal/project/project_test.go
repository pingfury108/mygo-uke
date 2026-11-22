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
