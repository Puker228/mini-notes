.PHONY: build run start release

APP := app
CMD := ./cmd/app
LDFLAGS := -s -w

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(APP) $(CMD)

run:
	./$(APP)

start: build
	./$(APP)

release: build
	GIN_MODE=release ./$(APP)
