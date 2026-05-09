package notes

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/ncruces/go-sqlite3/driver"
)

var db *sql.DB

var ErrNoteNotFound = errors.New("note not found")

func InitDB(path string) error {
	database, err := sql.Open("sqlite3", fmt.Sprintf("file:%s", path))
	if err != nil {
		return err
	}

	database.SetMaxOpenConns(1)

	if err := database.Ping(); err != nil {
		_ = database.Close()
		return err
	}

	if _, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			image_data TEXT NOT NULL DEFAULT ''
		);
	`); err != nil {
		_ = database.Close()
		return err
	}

	// migrate existing DBs that lack image_data column
	_, _ = database.Exec(`ALTER TABLE notes ADD COLUMN image_data TEXT NOT NULL DEFAULT '';`)

	db = database
	return nil
}

func CloseDB() error {
	if db == nil {
		return nil
	}

	err := db.Close()
	db = nil
	return err
}

func ListNotes() ([]Note, error) {
	rows, err := db.Query(`
		SELECT id, title, content, image_data
		FROM notes
		ORDER BY id DESC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var note Note
		if err := rows.Scan(&note.ID, &note.Title, &note.Content, &note.ImageData); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}

func AddNote(title, content, imageData string) (Note, error) {
	result, err := db.Exec(`
		INSERT INTO notes (title, content, image_data)
		VALUES (?, ?, ?);
	`, title, content, imageData)
	if err != nil {
		return Note{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Note{}, err
	}

	return Note{
		ID:        id,
		Title:     title,
		Content:   content,
		ImageData: imageData,
	}, nil
}

func GetNoteByID(ID int64) (Note, error) {
	var note Note
	err := db.QueryRow(`
		SELECT id, title, content, image_data
		FROM notes
		WHERE id = ?;
	`, ID).Scan(&note.ID, &note.Title, &note.Content, &note.ImageData)
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNoteNotFound
	}
	if err != nil {
		return Note{}, err
	}

	return note, nil
}

func UpdateNoteByID(ID int64, title, content, imageData string) (Note, error) {
	result, err := db.Exec(`
		UPDATE notes
		SET title = ?, content = ?, image_data = ?
		WHERE id = ?;
	`, title, content, imageData, ID)
	if err != nil {
		return Note{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Note{}, err
	}
	if rowsAffected == 0 {
		return Note{}, ErrNoteNotFound
	}

	return Note{ID: ID, Title: title, Content: content, ImageData: imageData}, nil
}

func DeleteNoteByID(ID int64) error {
	result, err := db.Exec(`
		DELETE FROM notes
		WHERE id = ?;
	`, ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNoteNotFound
	}

	return nil
}
