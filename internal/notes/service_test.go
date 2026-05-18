package notes

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
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

	created, err := AddNote(context.Background(), "title", "content", "", "")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
	if created.ID == 0 {
		t.Fatal("AddNote() returned zero ID")
	}

	got, err := GetNoteByID(context.Background(), created.ID)
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

	result, err := ListNotes(context.Background(), ListParams{})
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

	created, err := AddNote(context.Background(), "title", "content", "image-data", "")
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

	got, err := GetNoteByID(context.Background(), created.ID)
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

	alpha, err := AddNote(context.Background(), "Alpha", "first searchable note", "", "")
	if err != nil {
		t.Fatalf("AddNote(alpha) error = %v", err)
	}
	beta, err := AddNote(context.Background(), "Beta", "second note", "", "")
	if err != nil {
		t.Fatalf("AddNote(beta) error = %v", err)
	}
	gamma, err := AddNote(context.Background(), "Gamma", "third searchable note", "", "")
	if err != nil {
		t.Fatalf("AddNote(gamma) error = %v", err)
	}

	if err := SoftDeleteNoteByID(context.Background(), beta.ID); err != nil {
		t.Fatalf("SoftDeleteNoteByID() error = %v", err)
	}

	result, err := ListNotes(context.Background(), ListParams{})
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if result.Total != 2 || len(result.Notes) != 2 {
		t.Fatalf("ListNotes() = %+v", result)
	}

	searchResult, err := ListNotes(context.Background(), ListParams{
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

	nextPage, err := ListNotes(context.Background(), ListParams{
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

	created, err := AddNote(context.Background(), "Database", "sqlite", "", "")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	result, err := ListNotes(context.Background(), ListParams{Query: "lite"})
	if err != nil {
		t.Fatalf("ListNotes(substring search) error = %v", err)
	}
	if result.Total != 1 || len(result.Notes) != 1 || result.Notes[0].ID != created.ID {
		t.Fatalf("ListNotes(substring search) = %+v, want sqlite note", result)
	}
}

func TestListNotesEncryptedOnly(t *testing.T) {
	setupTestDB(t)

	regular, err := AddNote(context.Background(), "Regular", "plain content", "", "")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
	private, err := AddPrivateNote("Private", "encrypted content", "", "secret")
	if err != nil {
		t.Fatalf("AddPrivateNote() error = %v", err)
	}

	result, err := ListNotes(context.Background(), ListParams{EncryptedOnly: true})
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

func TestListNotesReturnsSortedTags(t *testing.T) {
	setupTestDB(t)

	created, err := AddNote(context.Background(), "Tagged", "content", "", "beta, Alpha, alpha")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
	if _, err := AddNote(context.Background(), "Z shared tag", "content", "", "beta"); err != nil {
		t.Fatalf("AddNote(shared tag) error = %v", err)
	}

	result, err := ListNotes(context.Background(), ListParams{Sort: "title", Order: "asc"})
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if len(result.Notes) != 2 {
		t.Fatalf("ListNotes() = %+v", result)
	}
	got := result.Notes[0]
	if got.ID != created.ID {
		t.Fatalf("ListNotes() first note = %+v, want tagged note", got)
	}
	want := []string{"Alpha", "alpha", "beta"}
	if len(got.Tags) != len(want) {
		t.Fatalf("Tags = %+v, want %+v", got.Tags, want)
	}
	for i := range want {
		if got.Tags[i] != want[i] {
			t.Fatalf("Tags = %+v, want %+v", got.Tags, want)
		}
	}

	detail, err := GetNoteByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetNoteByID() error = %v", err)
	}
	for i := range want {
		if detail.Tags[i] != want[i] {
			t.Fatalf("GetNoteByID().Tags = %+v, want %+v", detail.Tags, want)
		}
	}
}

func TestListNotesFiltersByTag(t *testing.T) {
	setupTestDB(t)

	work, err := AddNote(context.Background(), "Work", "quarterly plan", "", "work, planning")
	if err != nil {
		t.Fatalf("AddNote(work) error = %v", err)
	}
	personal, err := AddNote(context.Background(), "Personal", "weekend plan", "", "personal, planning")
	if err != nil {
		t.Fatalf("AddNote(personal) error = %v", err)
	}
	archived, err := AddNote(context.Background(), "Archived", "old work", "", "archive-only")
	if err != nil {
		t.Fatalf("AddNote(archived) error = %v", err)
	}
	if err := SoftDeleteNoteByID(context.Background(), archived.ID); err != nil {
		t.Fatalf("SoftDeleteNoteByID() error = %v", err)
	}

	result, err := ListNotes(context.Background(), ListParams{Tag: "planning", Sort: "title", Order: "asc"})
	if err != nil {
		t.Fatalf("ListNotes(tag) error = %v", err)
	}
	if result.Total != 2 || len(result.Notes) != 2 {
		t.Fatalf("ListNotes(tag) = %+v", result)
	}
	if result.Notes[0].ID != personal.ID || result.Notes[1].ID != work.ID {
		t.Fatalf("ListNotes(tag) notes = %+v, want Personal and Work", result.Notes)
	}

	searchResult, err := ListNotes(context.Background(), ListParams{Query: "quarterly", Tag: "planning"})
	if err != nil {
		t.Fatalf("ListNotes(search tag) error = %v", err)
	}
	if searchResult.Total != 1 || len(searchResult.Notes) != 1 || searchResult.Notes[0].ID != work.ID {
		t.Fatalf("ListNotes(search tag) = %+v, want work note", searchResult)
	}

	tags, err := ListTags(context.Background())
	if err != nil {
		t.Fatalf("ListTags() error = %v", err)
	}
	wantTags := []string{"personal", "planning", "work"}
	if !slices.Equal(tags, wantTags) {
		t.Fatalf("ListTags() = %+v, want %+v", tags, wantTags)
	}
}

func TestUpdatePrivateNoteByID(t *testing.T) {
	setupTestDB(t)

	created, err := AddPrivateNote("private", "old secret", "old-image", "secret")
	if err != nil {
		t.Fatalf("AddPrivateNote() error = %v", err)
	}

	updated, err := UpdatePrivateNoteByID(created.ID, "updated private", "new secret", "new-image", "secret", "")
	if err != nil {
		t.Fatalf("UpdatePrivateNoteByID() error = %v", err)
	}
	if updated.Title != "updated private" || updated.Content != "new secret" || updated.ImageData != "new-image" || !updated.IsEncrypted {
		t.Fatalf("UpdatePrivateNoteByID() = %+v", updated)
	}

	decrypted, err := DecryptNoteByID(created.ID, "secret")
	if err != nil {
		t.Fatalf("DecryptNoteByID() error = %v", err)
	}
	if decrypted.Title != "updated private" || decrypted.Content != "new secret" || decrypted.ImageData != "new-image" {
		t.Fatalf("DecryptNoteByID() = %+v", decrypted)
	}

	var storedContent string
	if err := db.QueryRow(`SELECT content FROM notes WHERE id = ?;`, created.ID).Scan(&storedContent); err != nil {
		t.Fatalf("QueryRow() error = %v", err)
	}
	if storedContent == "new secret" {
		t.Fatal("stored private content is plaintext, want encrypted content")
	}
}

func TestUpdatePrivateNoteByIDCanChangePassword(t *testing.T) {
	setupTestDB(t)

	created, err := AddPrivateNote("private", "old secret", "", "secret")
	if err != nil {
		t.Fatalf("AddPrivateNote() error = %v", err)
	}

	if _, err := UpdatePrivateNoteByID(created.ID, "private", "new secret", "", "secret", "new-secret"); err != nil {
		t.Fatalf("UpdatePrivateNoteByID() error = %v", err)
	}
	if _, err := DecryptNoteByID(created.ID, "secret"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("DecryptNoteByID(old password) error = %v, want %v", err, ErrInvalidPassword)
	}
	decrypted, err := DecryptNoteByID(created.ID, "new-secret")
	if err != nil {
		t.Fatalf("DecryptNoteByID(new password) error = %v", err)
	}
	if decrypted.Content != "new secret" {
		t.Fatalf("DecryptNoteByID(new password).Content = %q, want new secret", decrypted.Content)
	}
}

func TestUpdatePrivateNoteByIDRejectsWrongPassword(t *testing.T) {
	setupTestDB(t)

	created, err := AddPrivateNote("private", "old secret", "", "secret")
	if err != nil {
		t.Fatalf("AddPrivateNote() error = %v", err)
	}

	if _, err := UpdatePrivateNoteByID(created.ID, "private", "new secret", "", "wrong", ""); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("UpdatePrivateNoteByID() error = %v, want %v", err, ErrInvalidPassword)
	}
	decrypted, err := DecryptNoteByID(created.ID, "secret")
	if err != nil {
		t.Fatalf("DecryptNoteByID() error = %v", err)
	}
	if decrypted.Content != "old secret" {
		t.Fatalf("DecryptNoteByID().Content = %q, want old secret", decrypted.Content)
	}
}

func TestTogglePinNoteOrdersPinnedFirst(t *testing.T) {
	setupTestDB(t)

	alpha, err := AddNote(context.Background(), "Alpha", "first", "", "")
	if err != nil {
		t.Fatalf("AddNote(alpha) error = %v", err)
	}
	beta, err := AddNote(context.Background(), "Beta", "second", "", "")
	if err != nil {
		t.Fatalf("AddNote(beta) error = %v", err)
	}

	pinned, err := TogglePinNoteByID(context.Background(), alpha.ID)
	if err != nil {
		t.Fatalf("TogglePinNoteByID() error = %v", err)
	}
	if !pinned.IsPinned {
		t.Fatalf("TogglePinNoteByID() = %+v, want pinned", pinned)
	}

	result, err := ListNotes(context.Background(), ListParams{Sort: "title", Order: "desc"})
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if len(result.Notes) != 2 {
		t.Fatalf("ListNotes() = %+v", result)
	}
	if result.Notes[0].ID != alpha.ID || !result.Notes[0].IsPinned {
		t.Fatalf("ListNotes() first note = %+v, want pinned Alpha before Beta", result.Notes[0])
	}
	if result.Notes[1].ID != beta.ID {
		t.Fatalf("ListNotes() second note = %+v, want Beta", result.Notes[1])
	}

	unpinned, err := TogglePinNoteByID(context.Background(), alpha.ID)
	if err != nil {
		t.Fatalf("TogglePinNoteByID(unpin) error = %v", err)
	}
	if unpinned.IsPinned {
		t.Fatalf("TogglePinNoteByID(unpin) = %+v, want unpinned", unpinned)
	}
}

func TestArchiveRestoreDelete(t *testing.T) {
	setupTestDB(t)

	created, err := AddNote(context.Background(), "title", "content", "", "")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	if err := SoftDeleteNoteByID(context.Background(), created.ID); err != nil {
		t.Fatalf("SoftDeleteNoteByID() error = %v", err)
	}

	_, err = GetNoteByID(context.Background(), created.ID)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("GetNoteByID() after soft delete error = %v, want %v", err, ErrNoteNotFound)
	}

	archived, err := ListArchivedNotes(context.Background())
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

	if _, err := GetNoteByID(context.Background(), created.ID); err != nil {
		t.Fatalf("GetNoteByID() after restore error = %v", err)
	}

	if err := PermanentDeleteNoteByID(created.ID); err != nil {
		t.Fatalf("PermanentDeleteNoteByID() error = %v", err)
	}

	_, err = GetNoteByID(context.Background(), created.ID)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("GetNoteByID() after permanent delete error = %v, want %v", err, ErrNoteNotFound)
	}
}

func TestUpdateNoteFields(t *testing.T) {
	setupTestDB(t)

	created, err := AddNote(context.Background(), "title", "content", "old-image", "")
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

	got, err := GetNoteByID(context.Background(), created.ID)
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
		{name: "toggle pin", fn: func() error {
			_, err := TogglePinNoteByID(context.Background(), 404)
			return err
		}},
		{name: "soft delete", fn: func() error { return SoftDeleteNoteByID(context.Background(), 404) }},
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
