.PHONY: build run run-bin start release test fmt build-all clean docker-build docker-run docker-up docker-down docker-logs

APP := mini-notes
CMD := ./cmd/app
DIST_DIR := dist
LDFLAGS := -s -w
DOCKER_IMAGE := mini-notes:latest
DOCKER_COMPOSE := docker compose

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

docker-build:
	docker build -t $(DOCKER_IMAGE) .

docker-run: docker-build
	docker run --rm -p 8800:8800 -v mini-notes-data:/data $(DOCKER_IMAGE)

docker-up:
	$(DOCKER_COMPOSE) up -d --build

docker-down:
	$(DOCKER_COMPOSE) down

docker-logs:
	$(DOCKER_COMPOSE) logs -f mini-notes

$(DIST_DIR):
	mkdir -p $(DIST_DIR)

build-all: $(DIST_DIR)
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-darwin-amd64   $(CMD)
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-darwin-arm64   $(CMD)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-linux-amd64    $(CMD)
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-linux-arm64    $(CMD)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-windows-amd64.exe $(CMD)
