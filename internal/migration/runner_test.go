package migration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sqliteadapter "gthanks/internal/adapter/sqlite"
)

func TestRunCreatesSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite3")

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "../.."))
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(wd) }()

	db, err := sqliteadapter.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := Run(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='query_cache'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected query_cache table, got count=%d", count)
	}
}
