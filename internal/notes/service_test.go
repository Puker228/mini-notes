package notes

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSQLiteNoteStorageCRUD(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "notes.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer func() {
		if err := CloseDB(); err != nil {
			t.Fatalf("CloseDB() error = %v", err)
		}
	}()

	created, err := AddNote("title", "content")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
	if created.ID == 0 {
		t.Fatal("AddNote() returned zero ID")
	}

	if err := CloseDB(); err != nil {
		t.Fatalf("CloseDB() error = %v", err)
	}
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() after close error = %v", err)
	}

	got, err := GetNoteByID(created.ID)
	if err != nil {
		t.Fatalf("GetNoteByID() error = %v", err)
	}
	if got.Title != "title" || got.Content != "content" {
		t.Fatalf("GetNoteByID() = %+v", got)
	}

	updated, err := UpdateNoteByID(created.ID, "updated", "new content")
	if err != nil {
		t.Fatalf("UpdateNoteByID() error = %v", err)
	}
	if updated.Title != "updated" || updated.Content != "new content" {
		t.Fatalf("UpdateNoteByID() = %+v", updated)
	}

	notes, err := ListNotes()
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if len(notes) != 1 || notes[0].ID != created.ID {
		t.Fatalf("ListNotes() = %+v", notes)
	}

	if err := DeleteNoteByID(created.ID); err != nil {
		t.Fatalf("DeleteNoteByID() error = %v", err)
	}

	_, err = GetNoteByID(created.ID)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("GetNoteByID() error = %v, want %v", err, ErrNoteNotFound)
	}
}
