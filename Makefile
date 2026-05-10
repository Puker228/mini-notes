.PHONY: build run run-bin start release test fmt build-all clean

APP := mini-notes
CMD := ./cmd/app
DIST_DIR := dist
LDFLAGS := -s -w

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(APP) $(CMD)

run:
	go run $(CMD)

run-bin:
	./$(APP)

start: build
	./$(APP)

release: build
	./$(APP)

test:
	go test ./...

fmt:
	gofmt -w cmd internal

clean:
	rm -f $(APP)
	rm -rf $(DIST_DIR)/

$(DIST_DIR):
	mkdir -p $(DIST_DIR)

build-all: $(DIST_DIR)
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-darwin-amd64   $(CMD)
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-darwin-arm64   $(CMD)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-linux-amd64    $(CMD)
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-linux-arm64    $(CMD)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-windows-amd64.exe $(CMD)
