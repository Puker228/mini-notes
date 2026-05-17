CREATE TABLE IF NOT EXISTS notes
(
    id
    INTEGER
    PRIMARY
    KEY
    AUTOINCREMENT,
    title
    TEXT
    NOT
    NULL,
    content
    TEXT
    NOT
    NULL,
    image_data
    TEXT
    NOT
    NULL
    DEFAULT
    '',
    created_at
    TEXT
    NOT
    NULL
    DEFAULT
    '',
    updated_at
    TEXT
    NOT
    NULL
    DEFAULT
    '',
    deleted_at
    TEXT,
    encryption_salt
    TEXT,
    encryption_nonce
    TEXT,
    is_pinned
    BOOL
    DEFAULT
    0,
    is_encrypted
    BOOL
    DEFAULT
    0
);

CREATE TABLE IF NOT EXISTS tags
(
    id
    INTEGER
    PRIMARY
    KEY
    AUTOINCREMENT,
    name
    VARCHAR
(
    255
) UNIQUE
    );

CREATE TABLE IF NOT EXISTS note_tag
(
    note_id
    INTEGER
    NOT
    NULL
    REFERENCES
    notes
(
    id
) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags
(
    id
)
  ON DELETE CASCADE,
    PRIMARY KEY
(
    note_id,
    tag_id
)
    );

CREATE
VIRTUAL TABLE IF NOT EXISTS notes_fts
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
			AFTER
DELETE
ON notes
BEGIN
INSERT INTO notes_fts(notes_fts, rowid, title, content)
VALUES ('delete', old.id, old.title, old.content);
END;

CREATE TRIGGER IF NOT EXISTS notes_au
			AFTER
UPDATE
    ON notes
BEGIN
INSERT INTO notes_fts(notes_fts, rowid, title, content)
VALUES ('delete', old.id, old.title, old.content);

INSERT INTO notes_fts(rowid, title, content)
VALUES (new.id, new.title, new.content);
END;