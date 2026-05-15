package notes

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
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
		CREATE TABLE IF NOT EXISTS notes
		(
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			title            TEXT NOT NULL,
			content          TEXT NOT NULL,
			image_data       TEXT NOT NULL DEFAULT '',
			created_at       TEXT NOT NULL DEFAULT '',
			updated_at       TEXT NOT NULL DEFAULT '',
			deleted_at       TEXT,
			encryption_salt  TEXT,
			encryption_nonce TEXT,
			is_pinned        BOOL          DEFAULT 0,
			is_encrypted     BOOL          DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS tags
		(
			id   INTEGER PRIMARY KEY AUTOINCREMENT,
			name VARCHAR(255) UNIQUE
		);
		
		CREATE TABLE IF NOT EXISTS note_tag
		(
			note_id INTEGER NOT NULL REFERENCES notes (id) ON DELETE CASCADE,
			tag_id  INTEGER NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
		
			PRIMARY KEY (note_id, tag_id)
		);
		
		CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts
			USING fts5
		(
			title,
			content,
			content='notes',
			content_rowid='id'
		);
		
		CREATE TRIGGER IF NOT EXISTS notes_ai
			AFTER INSERT
			ON notes
		BEGIN
			INSERT INTO notes_fts(rowid, title, content)
			VALUES (new.id, new.title, new.content);
		END;
		
		CREATE TRIGGER IF NOT EXISTS notes_ad
			AFTER DELETE
			ON notes
		BEGIN
			INSERT INTO notes_fts(notes_fts, rowid, title, content)
			VALUES ('delete', old.id, old.title, old.content);
		END;
		
		CREATE TRIGGER IF NOT EXISTS notes_au
			AFTER UPDATE
			ON notes
		BEGIN
			INSERT INTO notes_fts(notes_fts, rowid, title, content)
			VALUES ('delete', old.id, old.title, old.content);
		
			INSERT INTO notes_fts(rowid, title, content)
			VALUES (new.id, new.title, new.content);
		END;
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
		`ALTER TABLE notes ADD COLUMN is_pinned BOOL DEFAULT 0;`,
		`ALTER TABLE notes ADD COLUMN is_encrypted BOOL DEFAULT 0;`,
	} {
		_, _ = database.Exec(migration)
	}

	if _, err := database.Exec(`INSERT INTO notes_fts(notes_fts) VALUES ('rebuild');`); err != nil {
		_ = database.Close()
		return err
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
	if err := s.Scan(&note.ID, &note.Title, &note.Content, &note.ImageData, &createdAt, &updatedAt, &deletedAt, &note.IsPinned, &note.IsEncrypted); err != nil {
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

func buildFTSQuery(query string) string {
	parts := strings.Fields(query)
	for i, part := range parts {
		part = strings.ReplaceAll(part, `"`, `""`)
		parts[i] = `"` + part + `"`
	}
	return strings.Join(parts, " AND ")
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

	filters := []string{
		"(deleted_at IS NULL OR deleted_at = '')",
	}
	args := []any{}
	if query := buildFTSQuery(p.Query); query != "" {
		likeQ := "%" + p.Query + "%"
		filters = append(filters, `notes.id IN (
			SELECT rowid FROM notes_fts WHERE notes_fts MATCH ?
			UNION
			SELECT id FROM notes WHERE title LIKE ? OR content LIKE ?
		)`)
		args = append(args, query, likeQ, likeQ)
	}
	if p.EncryptedOnly {
		filters = append(filters, "is_encrypted = 1")
	}
	if p.Tag != "" {
		filters = append(filters, `notes.id IN (
			SELECT note_tag.note_id
			FROM note_tag
			JOIN tags ON tags.id = note_tag.tag_id
			WHERE tags.name = ?
		)`)
		args = append(args, p.Tag)
	}
	whereClause := strings.Join(filters, " AND ")

	var total int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM notes
		WHERE %s
	`, whereClause)
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return ListResult{}, err
	}

	query := fmt.Sprintf(`
		SELECT notes.id, notes.title, notes.content, notes.image_data, notes.created_at, notes.updated_at, notes.deleted_at, notes.is_pinned, notes.is_encrypted
		FROM notes
		WHERE %s
		ORDER BY notes.is_pinned DESC, notes.%s %s
		LIMIT ? OFFSET ?
	`, whereClause, sortCol, sortOrder)

	offset := (p.Page - 1) * p.PageSize
	args = append(args, p.PageSize, offset)
	rows, err := db.Query(query, args...)
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
	if err := attachTags(notes); err != nil {
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

func listTags() ([]string, error) {
	rows, err := db.Query(`
		SELECT tags.name
		FROM tags
		JOIN note_tag ON note_tag.tag_id = tags.id
		JOIN notes ON notes.id = note_tag.note_id
		WHERE (notes.deleted_at IS NULL OR notes.deleted_at = '')
		GROUP BY tags.id, tags.name
		ORDER BY LOWER(tags.name), tags.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func renderMD(content string) (string, error) {
	md := []byte(content)
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	unsafeRes := markdown.Render(doc, renderer)
	res := bluemonday.UGCPolicy().SanitizeBytes(unsafeRes)
	return string(res), nil
}

func listTagsByNoteID(noteID int64) ([]string, error) {
	rows, err := db.Query(`
		SELECT tags.name
		FROM tags
		JOIN note_tag ON note_tag.tag_id = tags.id
		WHERE note_tag.note_id = ?
		ORDER BY LOWER(tags.name), tags.name
	`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func attachTags(notes []Note) error {
	for i := range notes {
		tags, err := listTagsByNoteID(notes[i].ID)
		if err != nil {
			return err
		}
		notes[i].Tags = tags
	}
	return nil
}

func listArchivedNotes() ([]Note, error) {
	rows, err := db.Query(`
		SELECT id, title, content, image_data, created_at, updated_at, deleted_at, is_pinned, is_encrypted
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

func parseTags(tags string) []string {
	tagsParts := strings.Split(tags, ",")
	cleanTags := make([]string, 0, len(tagsParts))

	for _, tag := range tagsParts {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			cleanTags = append(cleanTags, tag)
		}
	}

	return cleanTags
}

func addTags(cleanTags []string, noteID int64) error {
	for _, tag := range cleanTags {
		if _, err := db.Exec(`
			INSERT OR IGNORE INTO tags (name)
			VALUES (?);
		`, tag); err != nil {
			return fmt.Errorf("addTags: %v", err)
		}

		var id int64
		err := db.QueryRow(`
			SELECT id
			FROM tags
			WHERE name = ?;
		`, tag).Scan(&id)
		if err != nil {
			return fmt.Errorf("addTags: %v", err)
		}

		_, err = db.Exec(`
			INSERT OR IGNORE INTO note_tag (note_id, tag_id)
			VALUES (?, ?);
		`, noteID, id)
		if err != nil {
			return fmt.Errorf("addTags: %v", err)
		}
	}
	return nil
}

func addNote(title, content, imageData, tags string) (Note, error) {
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

	cleanTags := parseTags(tags)
	if err := addTags(cleanTags, id); err != nil {
		return Note{}, err
	}

	t, _ := time.Parse(timeLayout, now)
	return Note{
		ID:        id,
		Title:     title,
		Content:   content,
		ImageData: imageData,
		Tags:      cleanTags,
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
		INSERT INTO notes (title, content, image_data, created_at, updated_at, encryption_salt, encryption_nonce, is_encrypted)
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
		SELECT id, title, content, image_data, created_at, updated_at, deleted_at, is_pinned, is_encrypted
		FROM notes
		WHERE id = ? AND (deleted_at IS NULL OR deleted_at = '');
	`, ID)
	note, err := scanNote(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNoteNotFound
	}
	if err != nil {
		return Note{}, err
	}
	note.Tags, err = listTagsByNoteID(note.ID)
	return note, err
}

func decryptNoteByID(ID int64, password string) (Note, error) {
	var note Note
	var createdAt, updatedAt string
	var deletedAt sql.NullString
	var ciphertext, salt, nonce []byte

	row := db.QueryRow(`
		SELECT id, title, content, image_data, created_at, updated_at, deleted_at, is_pinned, is_encrypted, encryption_salt, encryption_nonce
		FROM notes
		WHERE id = ? AND (deleted_at IS NULL OR deleted_at = '');
	`, ID)
	if err := row.Scan(&note.ID, &note.Title, &ciphertext, &note.ImageData, &createdAt, &updatedAt, &deletedAt, &note.IsPinned, &note.IsEncrypted, &salt, &nonce); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Note{}, ErrNoteNotFound
		}
		return Note{}, err
	}

	note.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	note.UpdatedAt, _ = time.Parse(timeLayout, updatedAt)
	if deletedAt.Valid && deletedAt.String != "" {
		t, _ := time.Parse(timeLayout, deletedAt.String)
		note.DeletedAt = &t
	}

	if !note.IsEncrypted {
		note.Content = string(ciphertext)
		return note, nil
	}

	plaintext, err := NewPasswordEncryptor(password).Decrypt(salt, nonce, ciphertext)
	if err != nil {
		return Note{}, ErrInvalidPassword
	}
	note.Content = string(plaintext)
	return note, nil
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

func updatePrivateNoteByID(ID int64, title, content, imageData, currentPassword, newPassword string) (Note, error) {
	var ciphertext, salt, nonce []byte
	var isEncrypted bool

	row := db.QueryRow(`
		SELECT content, encryption_salt, encryption_nonce, is_encrypted
		FROM notes
		WHERE id = ? AND (deleted_at IS NULL OR deleted_at = '');
	`, ID)
	if err := row.Scan(&ciphertext, &salt, &nonce, &isEncrypted); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Note{}, ErrNoteNotFound
		}
		return Note{}, err
	}
	if !isEncrypted {
		return Note{}, ErrNoteNotEncrypted
	}
	if _, err := NewPasswordEncryptor(currentPassword).Decrypt(salt, nonce, ciphertext); err != nil {
		return Note{}, ErrInvalidPassword
	}

	password := currentPassword
	if newPassword != "" {
		password = newPassword
	}

	newSalt, newNonce, newCiphertext, err := NewPasswordEncryptor(password).Encrypt([]byte(content))
	if err != nil {
		return Note{}, err
	}

	now := time.Now().UTC().Format(timeLayout)
	result, err := db.Exec(`
		UPDATE notes
		SET title = ?, content = ?, image_data = ?, updated_at = ?, encryption_salt = ?, encryption_nonce = ?, is_encrypted = 1
		WHERE id = ? AND (deleted_at IS NULL OR deleted_at = '');
	`, title, newCiphertext, imageData, now, newSalt, newNonce, ID)
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
	return Note{ID: ID, Title: title, Content: content, ImageData: imageData, UpdatedAt: t, IsEncrypted: true}, nil
}

func togglePinNoteByID(ID int64) (Note, error) {
	result, err := db.Exec(`
		UPDATE notes
		SET is_pinned = CASE WHEN is_pinned = 1 THEN 0 ELSE 1 END
		WHERE id = ? AND (deleted_at IS NULL OR deleted_at = '');
	`, ID)
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

	return getNoteByID(ID)
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
