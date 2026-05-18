package notes

import (
	"context"
	"errors"
)

var (
	ErrNoteNotFound     = errors.New("note not found")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrNoteNotEncrypted = errors.New("note is not encrypted")
)

func ListNotes(ctx context.Context, p ListParams) (ListResult, error) {
	return listNotes(ctx, p)
}

func ListArchivedNotes(ctx context.Context) ([]Note, error) {
	return listArchivedNotes(ctx)
}

func ListTags(ctx context.Context) ([]string, error) {
	return listTags(ctx)
}

func AddNote(ctx context.Context, title, content, imageData, tags string) (Note, error) {
	return addNote(ctx, title, content, imageData, tags)
}

func AddPrivateNote(ctx context.Context, title, content, imageData, password string) (Note, error) {
	return addPrivateNote(ctx, title, content, imageData, password)
}

func DecryptNoteByID(ctx context.Context, ID int64, password string) (Note, error) {
	return decryptNoteByID(ctx, ID, password)
}

func GetNoteByID(ctx context.Context, ID int64) (Note, error) {
	return getNoteByID(ctx, ID)
}

func UpdateNoteByID(ID int64, title, content, imageData string) (Note, error) {
	return updateNoteByID(ID, title, content, imageData)
}

func UpdatePrivateNoteByID(ID int64, title, content, imageData, currentPassword, newPassword string) (Note, error) {
	return updatePrivateNoteByID(ID, title, content, imageData, currentPassword, newPassword)
}

func TogglePinNoteByID(ctx context.Context, ID int64) (Note, error) {
	return togglePinNoteByID(ctx, ID)
}

func SoftDeleteNoteByID(ctx context.Context, ID int64) error {
	return softDeleteNoteByID(ctx, ID)
}

func RestoreNoteByID(ctx context.Context, ID int64) error {
	return restoreNoteByID(ctx, ID)
}

func PermanentDeleteNoteByID(ctx context.Context, ID int64) error {
	return permanentDeleteNoteByID(ctx, ID)
}
