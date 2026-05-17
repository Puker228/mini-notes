-- name: ListTags :many
SELECT tags.name
FROM tags
         JOIN note_tag ON note_tag.tag_id = tags.id
         JOIN notes ON notes.id = note_tag.note_id
WHERE (notes.deleted_at IS NULL OR notes.deleted_at = '')
GROUP BY tags.id, tags.name
ORDER BY LOWER(tags.name), tags.name