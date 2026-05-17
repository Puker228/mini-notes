-- name: ListTags :many
SELECT tags.name
FROM tags
         JOIN note_tag ON note_tag.tag_id = tags.id
         JOIN notes ON notes.id = note_tag.note_id
WHERE (notes.deleted_at IS NULL OR notes.deleted_at = '')
GROUP BY tags.id, tags.name
ORDER BY LOWER(tags.name), tags.name;

-- name: ListTagsByNoteID :many
SELECT tags.name
FROM tags
         JOIN note_tag ON note_tag.tag_id = tags.id
WHERE note_tag.note_id = sqlc.arg(note_id)
ORDER BY LOWER(tags.name), tags.name;

-- name: ListArchivedNotes :many
SELECT id,
       title,
       content,
       image_data,
       created_at,
       updated_at,
       deleted_at,
       is_pinned,
       is_encrypted
FROM notes
WHERE deleted_at IS NOT NULL
  AND deleted_at != ''
ORDER BY deleted_at DESC;

-- name: GetNoteByID :one
SELECT id, title, content, image_data, created_at, updated_at, deleted_at, is_pinned, is_encrypted
FROM notes
WHERE id = sqlc.arg(id) AND (deleted_at IS NULL OR deleted_at = '');

-- name: CreateNote :one
INSERT INTO notes (title, content, image_data, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
RETURNING ID;
