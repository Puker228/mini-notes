package notes

import (
	"errors"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "notes.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := CloseDB(); err != nil {
			t.Fatalf("CloseDB() error = %v", err)
		}
	})
}

func TestStorageCRUD(t *testing.T) {
	setupTestDB(t)

	created, err := AddNote("title", "content", "")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
	if created.ID == 0 {
		t.Fatal("AddNote() returned zero ID")
	}

	got, err := GetNoteByID(created.ID)
	if err != nil {
		t.Fatalf("GetNoteByID() error = %v", err)
	}
	if got.Title != "title" || got.Content != "content" {
		t.Fatalf("GetNoteByID() = %+v", got)
	}

	updated, err := UpdateNoteByID(created.ID, "updated", "new content", "")
	if err != nil {
		t.Fatalf("UpdateNoteByID() error = %v", err)
	}
	if updated.Title != "updated" || updated.Content != "new content" {
		t.Fatalf("UpdateNoteByID() = %+v", updated)
	}

	result, err := ListNotes(ListParams{})
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if len(result.Notes) != 1 || result.Notes[0].ID != created.ID {
		t.Fatalf("ListNotes() = %+v", result)
	}
}

func TestStoragePersists(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "notes.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	created, err := AddNote("title", "content", "image-data")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	if err := CloseDB(); err != nil {
		t.Fatalf("CloseDB() error = %v", err)
	}
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() after close error = %v", err)
	}
	t.Cleanup(func() {
		if err := CloseDB(); err != nil {
			t.Fatalf("CloseDB() error = %v", err)
		}
	})

	got, err := GetNoteByID(created.ID)
	if err != nil {
		t.Fatalf("GetNoteByID() error = %v", err)
	}
	if got.Title != "title" || got.Content != "content" || got.ImageData != "image-data" {
		t.Fatalf("GetNoteByID() = %+v", got)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("GetNoteByID() timestamps were not populated: %+v", got)
	}
}

func TestListNotesFilters(t *testing.T) {
	setupTestDB(t)

	alpha, err := AddNote("Alpha", "first searchable note", "")
	if err != nil {
		t.Fatalf("AddNote(alpha) error = %v", err)
	}
	beta, err := AddNote("Beta", "second note", "")
	if err != nil {
		t.Fatalf("AddNote(beta) error = %v", err)
	}
	gamma, err := AddNote("Gamma", "third searchable note", "")
	if err != nil {
		t.Fatalf("AddNote(gamma) error = %v", err)
	}

	if err := SoftDeleteNoteByID(beta.ID); err != nil {
		t.Fatalf("SoftDeleteNoteByID() error = %v", err)
	}

	result, err := ListNotes(ListParams{})
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if result.Total != 2 || len(result.Notes) != 2 {
		t.Fatalf("ListNotes() = %+v", result)
	}

	searchResult, err := ListNotes(ListParams{
		Query:    "searchable",
		Sort:     "title",
		Order:    "asc",
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("ListNotes(search) error = %v", err)
	}
	if searchResult.Total != 2 || searchResult.TotalPages != 2 || !searchResult.HasNext {
		t.Fatalf("ListNotes(search) pagination = %+v", searchResult)
	}
	if len(searchResult.Notes) != 1 || searchResult.Notes[0].ID != alpha.ID {
		t.Fatalf("ListNotes(search) first page = %+v, want Alpha", searchResult.Notes)
	}

	nextPage, err := ListNotes(ListParams{
		Query:    "searchable",
		Sort:     "title",
		Order:    "asc",
		Page:     2,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("ListNotes(search page 2) error = %v", err)
	}
	if len(nextPage.Notes) != 1 || nextPage.Notes[0].ID != gamma.ID || !nextPage.HasPrev {
		t.Fatalf("ListNotes(search) second page = %+v, want Gamma with previous page", nextPage)
	}
}

func TestListNotesSearchMatchesSubstring(t *testing.T) {
	setupTestDB(t)

	created, err := AddNote("Database", "sqlite", "")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	result, err := ListNotes(ListParams{Query: "lite"})
	if err != nil {
		t.Fatalf("ListNotes(substring search) error = %v", err)
	}
	if result.Total != 1 || len(result.Notes) != 1 || result.Notes[0].ID != created.ID {
		t.Fatalf("ListNotes(substring search) = %+v, want sqlite note", result)
	}
}

func TestListNotesEncryptedOnly(t *testing.T) {
	setupTestDB(t)

	regular, err := AddNote("Regular", "plain content", "")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
	private, err := AddPrivateNote("Private", "encrypted content", "", "secret")
	if err != nil {
		t.Fatalf("AddPrivateNote() error = %v", err)
	}

	result, err := ListNotes(ListParams{EncryptedOnly: true})
	if err != nil {
		t.Fatalf("ListNotes(encrypted only) error = %v", err)
	}
	if result.Total != 1 || len(result.Notes) != 1 {
		t.Fatalf("ListNotes(encrypted only) = %+v", result)
	}
	if result.Notes[0].ID != private.ID || !result.Notes[0].IsEncrypted {
		t.Fatalf("ListNotes(encrypted only) note = %+v, want private note", result.Notes[0])
	}
	if result.Notes[0].ID == regular.ID {
		t.Fatalf("ListNotes(encrypted only) returned regular note")
	}
}

func TestArchiveRestoreDelete(t *testing.T) {
	setupTestDB(t)

	created, err := AddNote("title", "content", "")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	if err := SoftDeleteNoteByID(created.ID); err != nil {
		t.Fatalf("SoftDeleteNoteByID() error = %v", err)
	}

	_, err = GetNoteByID(created.ID)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("GetNoteByID() after soft delete error = %v, want %v", err, ErrNoteNotFound)
	}

	archived, err := ListArchivedNotes()
	if err != nil {
		t.Fatalf("ListArchivedNotes() error = %v", err)
	}
	if len(archived) != 1 || archived[0].ID != created.ID {
		t.Fatalf("ListArchivedNotes() = %+v", archived)
	}
	if archived[0].DeletedAt == nil || archived[0].DeletedAt.IsZero() {
		t.Fatalf("ListArchivedNotes() did not populate DeletedAt: %+v", archived[0])
	}

	if err := RestoreNoteByID(created.ID); err != nil {
		t.Fatalf("RestoreNoteByID() error = %v", err)
	}

	if _, err := GetNoteByID(created.ID); err != nil {
		t.Fatalf("GetNoteByID() after restore error = %v", err)
	}

	if err := PermanentDeleteNoteByID(created.ID); err != nil {
		t.Fatalf("PermanentDeleteNoteByID() error = %v", err)
	}

	_, err = GetNoteByID(created.ID)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("GetNoteByID() after permanent delete error = %v, want %v", err, ErrNoteNotFound)
	}
}

func TestUpdateNoteFields(t *testing.T) {
	setupTestDB(t)

	created, err := AddNote("title", "content", "old-image")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	updated, err := UpdateNoteByID(created.ID, "updated", "new content", "new-image")
	if err != nil {
		t.Fatalf("UpdateNoteByID() error = %v", err)
	}
	if updated.Title != "updated" || updated.Content != "new content" || updated.ImageData != "new-image" {
		t.Fatalf("UpdateNoteByID() = %+v", updated)
	}

	got, err := GetNoteByID(created.ID)
	if err != nil {
		t.Fatalf("GetNoteByID() error = %v", err)
	}
	if !got.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("CreatedAt changed after update: got %v, want %v", got.CreatedAt, created.CreatedAt)
	}
	if got.Title != "updated" || got.Content != "new content" || got.ImageData != "new-image" {
		t.Fatalf("GetNoteByID() after update = %+v", got)
	}
}

func TestStorageNotFound(t *testing.T) {
	setupTestDB(t)

	tests := []struct {
		name string
		fn   func() error
	}{
		{name: "update", fn: func() error {
			_, err := UpdateNoteByID(404, "title", "content", "")
			return err
		}},
		{name: "soft delete", fn: func() error { return SoftDeleteNoteByID(404) }},
		{name: "restore", fn: func() error { return RestoreNoteByID(404) }},
		{name: "permanent delete", fn: func() error { return PermanentDeleteNoteByID(404) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); !errors.Is(err, ErrNoteNotFound) {
				t.Fatalf("%s error = %v, want %v", tt.name, err, ErrNoteNotFound)
			}
		})
	}
}
