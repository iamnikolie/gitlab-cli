.PHONY: build install uninstall test vet

BIN := gl
PREFIX ?= $(HOME)/.local

build:
	go build -o $(BIN) .

install: build
	mkdir -p $(PREFIX)/bin
	ln -sf $(CURDIR)/$(BIN) $(PREFIX)/bin/$(BIN)

uninstall:
	rm -f $(PREFIX)/bin/$(BIN)

test:
	go test ./...

vet:
	go vet ./...
