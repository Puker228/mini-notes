package notes

import (
	"errors"
)

var (
	ErrNoteNotFound     = errors.New("note not found")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrNoteNotEncrypted = errors.New("note is not encrypted")
)

func ListNotes(p ListParams) (ListResult, error) {
	return listNotes(p)
}

func ListArchivedNotes() ([]Note, error) {
	return listArchivedNotes()
}

func ListTags() ([]string, error) {
	return listTags()
}

func AddNote(title, content, imageData, tags string) (Note, error) {
	return addNote(title, content, imageData, tags)
}

func AddPrivateNote(title, content, imageData, password string) (Note, error) {
	return addPrivateNote(title, content, imageData, password)
}

func DecryptNoteByID(ID int64, password string) (Note, error) {
	return decryptNoteByID(ID, password)
}

func GetNoteByID(ID int64) (Note, error) {
	return getNoteByID(ID)
}

func UpdateNoteByID(ID int64, title, content, imageData string) (Note, error) {
	return updateNoteByID(ID, title, content, imageData)
}

func UpdatePrivateNoteByID(ID int64, title, content, imageData, currentPassword, newPassword string) (Note, error) {
	return updatePrivateNoteByID(ID, title, content, imageData, currentPassword, newPassword)
}

func TogglePinNoteByID(ID int64) (Note, error) {
	return togglePinNoteByID(ID)
}

func SoftDeleteNoteByID(ID int64) error {
	return softDeleteNoteByID(ID)
}

func RestoreNoteByID(ID int64) error {
	return restoreNoteByID(ID)
}

func PermanentDeleteNoteByID(ID int64) error {
	return permanentDeleteNoteByID(ID)
}
