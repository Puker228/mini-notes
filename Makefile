.PHONY: build run run-bin start release

APP := app
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
