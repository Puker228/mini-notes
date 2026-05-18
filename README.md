# Mini Notes

Mini Notes is a small web app for creating, viewing, editing, and deleting notes.
Notes are stored in a local SQLite database, and the UI is rendered with Go HTML
templates through Echo.

## Features

- Create notes with a title and Markdown content
- Render Markdown safely with HTML sanitization
- Attach an optional image, stored on disk and served from `/uploads`
- Tag notes and filter the list by tag
- Search notes with SQLite full-text search (FTS5)
- Sort by creation date, update date, or title, with pagination
- Pin notes to keep them at the top of the list
- Create private, password-encrypted notes and decrypt them on demand
- View all notes in reverse creation order
- Open a note detail page
- Edit existing notes
- Archive, restore, and permanently delete notes
- Store data locally in SQLite
- Build a standalone binary with embedded templates and assets

## Requirements

- Go 1.26.1 or newer
- Docker and Docker Compose, if you want to run the app in a container

## Run Locally

```sh
go run ./cmd/app
```

The app starts on:

```text
http://localhost:8800/note
```

By default, the SQLite database is created as `notes.db` in the project
directory, and uploaded images are stored in the `uploads/` directory.

Write logs to a file:

```sh
NOTES_LOG_PATH=mini-notes.log go run ./cmd/app
```

## Run with Docker

Build the image:

```sh
docker build -t mini-notes:latest .
```

Run the container:

```sh
docker run --rm -p 8800:8800 -v mini-notes-data:/data mini-notes:latest
```

The Docker image stores the SQLite database at `/data/notes.db` inside the
container. The command above mounts the `mini-notes-data` Docker volume so notes
are kept between container restarts.

The app is available at:

```text
http://localhost:8800/note
```

## Run with Docker Compose

```sh
docker compose up -d --build
```

View logs:

```sh
docker compose logs -f mini-notes
```

Stop the app:

```sh
docker compose down
```

The Compose setup exposes port `8800`, sets `NOTES_DB_PATH=/data/notes.db`,
`NOTES_UPLOADS_PATH=/data/uploads`, and
`NOTES_LOG_PATH=/data/logs/mini-notes.log`, and uses the named volume
`mini-notes-data` for persistent SQLite storage, uploaded images, and logs.

## Configuration

You can change the database path with `NOTES_DB_PATH`:

```sh
NOTES_DB_PATH=/tmp/mini-notes.db go run ./cmd/app
```

You can change where uploaded images are stored with `NOTES_UPLOADS_PATH`:

```sh
NOTES_UPLOADS_PATH=/tmp/mini-notes-uploads go run ./cmd/app
```

You can write logs to a file with `NOTES_LOG_PATH`:

```sh
NOTES_LOG_PATH=/tmp/mini-notes.log go run ./cmd/app
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
make run-bin
```

Runs the already built `mini-notes` binary.

```sh
make start
```

Builds and starts the binary.

```sh
make release
```

Builds and starts the app.

```sh
make test
```

Runs the test suite.

```sh
make fmt
```

Formats the `cmd` and `internal` packages with `gofmt`.

```sh
make build-all
```

Builds release binaries for macOS, Linux, and Windows into `dist/`.

```sh
make clean-apps
```

Removes generated binaries and the `dist/` directory.

```sh
make clean-all
```

Removes binaries, the `dist/` directory, the `uploads/` directory, and the
SQLite database files.

```sh
make docker-build
```

Builds the Docker image as `mini-notes:latest`.

```sh
make docker-run
```

Builds and runs the Docker image on port `8800` with the `mini-notes-data`
volume mounted at `/data`.

```sh
make docker-up
```

Starts the app with Docker Compose in the background.

```sh
make docker-down
```

Stops and removes the Docker Compose containers.

```sh
make docker-logs
```

Follows Docker Compose logs for the app service.

## Database Queries

SQL queries live in `internal/notes/db/query.sql` and the schema in
`internal/notes/db/schema.sql`. Type-safe Go code is generated from them with
[sqlc](https://sqlc.dev), configured in `sqlc.yaml`:

```sh
sqlc generate
```

## Project Structure

```text
cmd/app/main.go              Application entry point and HTTP routes
cmd/app/templates/           HTML templates embedded into the binary
cmd/app/static/              CSS assets embedded into the binary
internal/notes/model.go      Note model and list parameters
internal/notes/service.go    Notes service API
internal/notes/storage.go    SQLite storage logic
internal/notes/handlers.go   HTTP handlers
internal/notes/db/           sqlc schema, queries, and generated code
internal/notes/*_test.go     Tests
Dockerfile                   Multi-stage Docker build
docker-compose.yaml          Docker Compose service and persistent volume
.github/workflows/test.yaml  CI workflow that runs the tests
sqlc.yaml                    sqlc configuration
```

## Tests

Test files end with `_test.go`, and test functions start with `Test`.
The tests cover note storage, HTTP handlers, and Echo CSRF configuration.

```sh
go test ./...
```

Tests also run automatically on every push and pull request to `main` through
the GitHub Actions workflow in `.github/workflows/test.yaml`.
