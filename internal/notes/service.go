package notes

import (
	"errors"
)

func nextID() int64 {
	if len(Notes) == 0 {
		return 1
	}

	maxNote := Notes[0]

	for _, note := range Notes {
		if note.ID >= maxNote.ID {
			maxNote = note
		}
	}

	return maxNote.ID + 1
}

func AddNote(title string, content string) Note {
	note := Note{
		ID:      nextID(),
		Title:   title,
		Content: content,
	}
	Notes = append(Notes, note)
	return note
}

var ErrNoteNotFound = errors.New("note not found")

func GetNoteByID(ID int64) (Note, error) {
	for _, note := range Notes {
		if note.ID == ID {
			return note, nil
		}
	}
	return Note{}, ErrNoteNotFound
}

func DeleteNoteByID(ID int64) {
	for i, note := range Notes {
		if note.ID == ID {
			Notes = append(Notes[:i], Notes[i+1:]...)
		}
	}
}
