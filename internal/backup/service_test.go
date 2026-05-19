package backup

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func TestServiceSaveCreatesZipWithDatabaseUploadsAndManifest(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "notes.db")
	backupDir := filepath.Join(root, "backups")
	uploadsDir := filepath.Join(root, "uploads")

	if err := os.MkdirAll(filepath.Join(uploadsDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploadsDir, "nested", "photo.txt"), []byte("image-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})

	if _, err := db.Exec("CREATE TABLE notes (id INTEGER PRIMARY KEY, title TEXT); INSERT INTO notes (title) VALUES ('first')"); err != nil {
		t.Fatal(err)
	}

	service := NewService(db, backupDir, uploadsDir)
	if err := service.Save(t.Context()); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one backup file, got %d", len(entries))
	}
	if !strings.HasSuffix(entries[0].Name(), ".zip") {
		t.Fatalf("expected zip backup, got %q", entries[0].Name())
	}

	archive, err := zip.OpenReader(filepath.Join(backupDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()

	files := map[string]*zip.File{}
	for _, file := range archive.File {
		files[file.Name] = file
	}

	for _, name := range []string{"db.sqlite", "uploads/", "uploads/nested/photo.txt", "manifest.json"} {
		if files[name] == nil {
			t.Fatalf("expected %q in backup archive", name)
		}
	}

	manifestFile, err := files["manifest.json"].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer manifestFile.Close()

	var manifest Manifest
	if err := json.NewDecoder(manifestFile).Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.DBFile != "db.sqlite" {
		t.Fatalf("expected db file in manifest, got %q", manifest.DBFile)
	}
	if manifest.UploadsDir != "uploads" {
		t.Fatalf("expected uploads dir in manifest, got %q", manifest.UploadsDir)
	}
}
