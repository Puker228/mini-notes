package notes

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

var db *sql.DB

const timeLayout = time.RFC3339

type scanner interface {
	Scan(dest ...any) error
}

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
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			title      TEXT NOT NULL,
			content    TEXT NOT NULL,
			image_data TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT '',
			deleted_at TEXT,
			encryption_salt TEXT,
			encryption_nonce TEXT,
			is_encypted BOOL DEFAULT 0
		);
	`); err != nil {
		_ = database.Close()
		return err
	}

	for _, migration := range []string{
		`ALTER TABLE notes ADD COLUMN image_data TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE notes ADD COLUMN created_at TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE notes ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE notes ADD COLUMN deleted_at TEXT;`,
		`ALTER TABLE notes ADD COLUMN encryption_salt TEXT;`,
		`ALTER TABLE notes ADD COLUMN encryption_nonce TEXT;`,
		`ALTER TABLE notes ADD COLUMN is_encypted BOOL DEFAULT 0;`,
	} {
		_, _ = database.Exec(migration)
	}

	for _, pragma := range []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA synchronous=NORMAL;`,
		`PRAGMA foreign_keys=ON;`,
	} {
		if _, err := database.Exec(pragma); err != nil {
			_ = database.Close()
			return err
		}
	}

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

func scanNote(s scanner) (Note, error) {
	var note Note
	var createdAt, updatedAt string
	var deletedAt sql.NullString
	if err := s.Scan(&note.ID, &note.Title, &note.Content, &note.ImageData, &createdAt, &updatedAt, &deletedAt); err != nil {
		return Note{}, err
	}
	note.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	note.UpdatedAt, _ = time.Parse(timeLayout, updatedAt)
	if deletedAt.Valid && deletedAt.String != "" {
		t, _ := time.Parse(timeLayout, deletedAt.String)
		note.DeletedAt = &t
	}
	return note, nil
}

func listNotes(p ListParams) (ListResult, error) {
	if p.PageSize <= 0 {
		p.PageSize = 10
	}
	if p.Page <= 0 {
		p.Page = 1
	}

	sortCol := "created_at"
	switch p.Sort {
	case "updated_at", "title":
		sortCol = p.Sort
	}
	sortOrder := "DESC"
	if strings.ToUpper(p.Order) == "ASC" {
		sortOrder = "ASC"
	}

	likeQ := "%" + p.Query + "%"

	var total int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM notes
		WHERE (deleted_at IS NULL OR deleted_at = '')
		  AND (title LIKE ? OR content LIKE ?)
	`, likeQ, likeQ).Scan(&total); err != nil {
		return ListResult{}, err
	}

	query := fmt.Sprintf(`
		SELECT id, title, content, image_data, created_at, updated_at, deleted_at
		FROM notes
		WHERE (deleted_at IS NULL OR deleted_at = '')
		  AND (title LIKE ? OR content LIKE ?)
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, sortCol, sortOrder)

	offset := (p.Page - 1) * p.PageSize
	rows, err := db.Query(query, likeQ, likeQ, p.PageSize, offset)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			return ListResult{}, err
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}

	totalPages := (total + p.PageSize - 1) / p.PageSize
	if totalPages == 0 {
		totalPages = 1
	}

	return ListResult{
		Notes:      notes,
		Total:      total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: totalPages,
		HasPrev:    p.Page > 1,
		HasNext:    p.Page < totalPages,
		PrevPage:   p.Page - 1,
		NextPage:   p.Page + 1,
	}, nil
}

func listArchivedNotes() ([]Note, error) {
	rows, err := db.Query(`
		SELECT id, title, content, image_data, created_at, updated_at, deleted_at
		FROM notes
		WHERE deleted_at IS NOT NULL AND deleted_at != ''
		ORDER BY deleted_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

func addNote(title, content, imageData string) (Note, error) {
	now := time.Now().UTC().Format(timeLayout)
	result, err := db.Exec(`
		INSERT INTO notes (title, content, image_data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?);
	`, title, content, imageData, now, now)
	if err != nil {
		return Note{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Note{}, err
	}

	t, _ := time.Parse(timeLayout, now)
	return Note{
		ID:        id,
		Title:     title,
		Content:   content,
		ImageData: imageData,
		CreatedAt: t,
		UpdatedAt: t,
	}, nil
}

func addPrivateNote(title, content, imageData, password string) (Note, error) {
	now := time.Now().UTC().Format(timeLayout)

	encryptor := NewPasswordEncryptor(password)
	plainText := []byte(content)

	salt, nonce, cipherText, err := encryptor.Encrypt(plainText)
	if err != nil {
		return Note{}, err
	}

	result, err := db.Exec(`
		INSERT INTO notes (title, content, image_data, created_at, updated_at, encryption_salt, encryption_nonce, is_encypted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`, title, cipherText, imageData, now, now, salt, nonce, true)
	if err != nil {
		return Note{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Note{}, err
	}

	t, _ := time.Parse(timeLayout, now)
	return Note{
		ID:        id,
		Title:     title,
		Content:   content,
		ImageData: imageData,
		CreatedAt: t,
		UpdatedAt: t,
	}, nil
}

func getNoteByID(ID int64) (Note, error) {
	row := db.QueryRow(`
		SELECT id, title, content, image_data, created_at, updated_at, deleted_at
		FROM notes
		WHERE id = ? AND (deleted_at IS NULL OR deleted_at = '');
	`, ID)
	note, err := scanNote(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNoteNotFound
	}
	return note, err
}

func updateNoteByID(ID int64, title, content, imageData string) (Note, error) {
	now := time.Now().UTC().Format(timeLayout)
	result, err := db.Exec(`
		UPDATE notes
		SET title = ?, content = ?, image_data = ?, updated_at = ?
		WHERE id = ? AND (deleted_at IS NULL OR deleted_at = '');
	`, title, content, imageData, now, ID)
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

	t, _ := time.Parse(timeLayout, now)
	return Note{ID: ID, Title: title, Content: content, ImageData: imageData, UpdatedAt: t}, nil
}

func softDeleteNoteByID(ID int64) error {
	now := time.Now().UTC().Format(timeLayout)
	result, err := db.Exec(`
		UPDATE notes SET deleted_at = ?
		WHERE id = ? AND (deleted_at IS NULL OR deleted_at = '');
	`, now, ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNoteNotFound
	}
	return nil
}

func restoreNoteByID(ID int64) error {
	result, err := db.Exec(`UPDATE notes SET deleted_at = NULL WHERE id = ?;`, ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNoteNotFound
	}
	return nil
}

func permanentDeleteNoteByID(ID int64) error {
	result, err := db.Exec(`DELETE FROM notes WHERE id = ?;`, ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNoteNotFound
	}
	return nil
}
