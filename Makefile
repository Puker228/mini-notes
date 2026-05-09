.PHONY: build run run-bin start release build-all clean

APP := mini-notes
CMD := ./cmd/app
LDFLAGS := -s -w

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(APP) $(CMD)

run:
	go run $(CMD)

run-bin:
	./$(APP)

start: build
	./$(APP)

release: build
	GIN_MODE=release ./$(APP)

clean:
	rm -f $(APP)
	rm -rf dist/

build-all:
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/$(APP)-darwin-amd64   $(CMD)
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/$(APP)-darwin-arm64   $(CMD)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/$(APP)-linux-amd64    $(CMD)
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/$(APP)-linux-arm64    $(CMD)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/$(APP)-windows-amd64.exe $(CMD)
