# Mini Notes

Mini Notes is a small web app for creating, viewing, editing, and deleting notes.
Notes are stored in a local SQLite database, and the UI is rendered with Go HTML
templates.

## Features

- Create notes with a title and content
- Attach an optional image to a note
- View all notes in reverse creation order
- Open a note detail page
- Edit existing notes
- Archive, restore, and permanently delete notes
- Store data locally in SQLite
- Build a standalone binary with embedded templates

## Requirements

- Go 1.26.1 or newer

## Run Locally

```sh
go run ./cmd/app
```

The app starts on:

```text
http://localhost:8800/note
```

By default, the SQLite database is created as `notes.db` in the project
directory.

## Configuration

You can change the database path with `NOTES_DB_PATH`:

```sh
NOTES_DB_PATH=/tmp/mini-notes.db go run ./cmd/app
```

## Make Commands

```sh
make run
```

Runs the app with `go run`.

```sh
make build
```

Builds the `mini-notes` binary.

```sh
make start
```

Builds and starts the binary.

```sh
make release
```

Builds and starts the app with `GIN_MODE=release`.

```sh
make build-all
```

Builds release binaries for macOS, Linux, and Windows into `dist/`.

```sh
make clean
```

Removes generated binaries and the `dist/` directory.

## Project Structure

```text
cmd/app/main.go              Application entry point and HTTP routes
cmd/app/templates/           HTML templates embedded into the binary
internal/notes/model.go      Note model
internal/notes/service.go    SQLite storage logic
internal/notes/handlers.go   HTTP handlers
internal/notes/*_test.go     Tests
internal/csrf/               CSRF middleware and tests
```

## Tests

Test files end with `_test.go`, and test functions start with `Test`.
The tests cover note storage, HTTP handlers, and CSRF middleware.

```sh
go test ./...
```
