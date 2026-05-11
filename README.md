# Mini Notes

Mini Notes is a small web app for creating, viewing, editing, and deleting notes.
Notes are stored in a local SQLite database, and the UI is rendered with Go HTML
templates through Echo.

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
directory.

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

The Compose setup exposes port `8800`, sets `NOTES_DB_PATH=/data/notes.db`, and
uses the named volume `mini-notes-data` for persistent SQLite storage.

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

Builds and starts the app.

```sh
make build-all
```

Builds release binaries for macOS, Linux, and Windows into `dist/`.

```sh
make clean
```

Removes generated binaries and the `dist/` directory.

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

## Project Structure

```text
cmd/app/main.go              Application entry point and HTTP routes
cmd/app/templates/           HTML templates embedded into the binary
internal/notes/model.go      Note model
internal/notes/service.go    Notes service API
internal/notes/storage.go    SQLite storage logic
internal/notes/handlers.go   HTTP handlers
internal/notes/*_test.go     Tests
Dockerfile                   Multi-stage Docker build
docker-compose.yaml          Docker Compose service and persistent volume
```

## Tests

Test files end with `_test.go`, and test functions start with `Test`.
The tests cover note storage, HTTP handlers, and Echo CSRF configuration.

```sh
go test ./...
```
