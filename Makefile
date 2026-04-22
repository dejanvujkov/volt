BIN     ?= volt
BAT_DIR := third_party/bat
BAT_BIN := $(BAT_DIR)/bat

.PHONY: all build bat run tidy clean

all: build

## build: compile the volt binary into ./bin/volt
build:
	@mkdir -p bin
	go build -o bin/$(BIN) ./cmd/volt

## bat: build the vendored tshakalekholoane/bat binary (Linux only)
bat:
	$(MAKE) -C $(BAT_DIR) build

## run: launch the TUI
run: build
	./bin/$(BIN)

## tidy: refresh go.sum
tidy:
	go mod tidy

## clean: remove compiled binaries
clean:
	rm -rf bin
	rm -f $(BAT_BIN)
