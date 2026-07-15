package database

import (
	"context"
	"hevy-pp-cli/internal/csvimport"
	"os"
	"path/filepath"
	"testing"
)

func TestImportIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hevy.csv")
	data := "Workout Title,Workout Start,Exercise Title,Set Number,Weight kg,Reps\nPush,2026-01-02 10:00:00,Bench Press,1,80,8\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	parsed, err := csvimport.ParseFile(path, 1024)
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(filepath.Join(dir, "hevy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first, err := store.ImportCSV(context.Background(), path, parsed, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.ImportCSV(context.Background(), path, parsed, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.SetsInserted != 1 || !second.Skipped {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
}
