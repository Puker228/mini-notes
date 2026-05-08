build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o app ./cmd/app

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o app ./cmd/app

run:
	./app

start: build run
start-linux: build-linux run
