package notes

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	notesdb "github.com/Puker228/mini-notes/internal/notes/db"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
	_ "github.com/ncruces/go-sqlite3/driver"
)

var (
	db      *sql.DB
	queries *notesdb.Queries
)

const timeLayout = time.RFC3339

//go:embed db/schema.sql
var schemaSQL string

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

	if _, err := database.Exec(schemaSQL); err != nil {
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
	queries = notesdb.New(database)
	return nil
}

func CloseDB() error {
	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	queries = nil
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

func listNotes(ctx context.Context, p ListParams) (ListResult, error) {
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

func listTags(ctx context.Context) ([]string, error) {
	return queries.ListTags(ctx)
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
	return queries.ListTagsByNoteID(context.Background(), noteID)
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

func listArchivedNotes(ctx context.Context) ([]Note, error) {
	archivedNotes, err := queries.ListArchivedNotes(ctx)
	if err != nil {
		return nil, err
	}

	notes := make([]Note, 0, len(archivedNotes))
	for _, archivedNote := range archivedNotes {
		createdAt, _ := time.Parse(timeLayout, archivedNote.CreatedAt)
		updatedAt, _ := time.Parse(timeLayout, archivedNote.UpdatedAt)

		var deletedAt *time.Time
		if archivedNote.DeletedAt.Valid && archivedNote.DeletedAt.String != "" {
			t, _ := time.Parse(timeLayout, archivedNote.DeletedAt.String)
			deletedAt = &t
		}

		notes = append(notes, Note{
			ID:          archivedNote.ID,
			Title:       archivedNote.Title,
			Content:     archivedNote.Content,
			ImageData:   archivedNote.ImageData,
			IsPinned:    archivedNote.IsPinned,
			IsEncrypted: archivedNote.IsEncrypted,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
			DeletedAt:   deletedAt,
		})
	}

	return notes, nil
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

func addNote(ctx context.Context, title, content, imageData, tags string) (Note, error) {
	now := time.Now().UTC().Format(timeLayout)

	createdNoteID, err := queries.CreateNote(ctx, notesdb.CreateNoteParams{
		Title:     title,
		Content:   content,
		ImageData: imageData,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return Note{}, err
	}

	cleanTags := parseTags(tags)
	if err := addTags(cleanTags, createdNoteID); err != nil {
		return Note{}, err
	}

	t, _ := time.Parse(timeLayout, now)
	return Note{
		ID:        createdNoteID,
		Title:     title,
		Content:   content,
		ImageData: imageData,
		Tags:      cleanTags,
		CreatedAt: t,
		UpdatedAt: t,
	}, nil
}

func addPrivateNote(ctx context.Context, title, content, imageData, password string) (Note, error) {
	now := time.Now().UTC().Format(timeLayout)

	encryptor := NewPasswordEncryptor(password)
	plainText := []byte(content)

	salt, nonce, cipherText, err := encryptor.Encrypt(plainText)
	if err != nil {
		return Note{}, err
	}

	createdNoteID, err := queries.CreatePrivateNote(ctx, notesdb.CreatePrivateNoteParams{
		Title:           title,
		Content:         string(cipherText),
		ImageData:       imageData,
		CreatedAt:       now,
		UpdatedAt:       now,
		EncryptionSalt:  salt,
		EncryptionNonce: nonce,
		IsEncrypted:     true,
	})
	if err != nil {
		return Note{}, err
	}

	t, _ := time.Parse(timeLayout, now)
	return Note{
		ID:        createdNoteID,
		Title:     title,
		Content:   content,
		ImageData: imageData,
		CreatedAt: t,
		UpdatedAt: t,
	}, nil
}

func getNoteByID(ctx context.Context, ID int64) (Note, error) {
	noteRow, err := queries.GetNoteByID(ctx, ID)
	createdAt, _ := time.Parse(timeLayout, noteRow.CreatedAt)
	updatedAt, _ := time.Parse(timeLayout, noteRow.UpdatedAt)

	var deletedAt *time.Time
	if noteRow.DeletedAt.Valid && noteRow.DeletedAt.String != "" {
		t, _ := time.Parse(timeLayout, noteRow.DeletedAt.String)
		deletedAt = &t
	}
	note := Note{
		ID:          noteRow.ID,
		Title:       noteRow.Title,
		Content:     noteRow.Content,
		ImageData:   noteRow.ImageData,
		IsPinned:    noteRow.IsPinned,
		IsEncrypted: noteRow.IsEncrypted,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		DeletedAt:   deletedAt,
	}

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

func togglePinNoteByID(ctx context.Context, ID int64) (Note, error) {
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

	return getNoteByID(ctx, ID)
}

func softDeleteNoteByID(ctx context.Context, ID int64) error {
	now := time.Now().UTC().Format(timeLayout)
	rows, err := queries.SoftDeleteNoteByID(ctx, notesdb.SoftDeleteNoteByIDParams{
		DeletedAt: sql.NullString{String: now, Valid: true},
		ID:        ID,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNoteNotFound
	}
	return nil
}

func restoreNoteByID(ctx context.Context, ID int64) error {
	rows, err := queries.RestoreNoteByID(ctx, ID)
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
