BIN          ?= volt
BAT_DIR      := third_party/bat
BAT_BUILD    := $(BAT_DIR)/bin/bat
EMBED_DIR    := internal/batbin/batdata
EMBED_BIN    := $(EMBED_DIR)/bat

# Use the upstream bat's version string (git describe) when we're building
# from inside its worktree; fall back to "bundled" when that fails (e.g.
# shallow clones, tarball installs).
BAT_TAG      := $(shell git -C $(BAT_DIR) describe --always --dirty --tags --long 2>/dev/null || echo bundled)
VOLT_TAG     := $(shell git describe --always --dirty --tags 2>/dev/null || echo dev)

BAT_LDFLAGS  := -X main.tag=$(BAT_TAG)
VOLT_LDFLAGS := -X main.voltVersion=$(VOLT_TAG)

.PHONY: all build bat embed run tidy clean submodule

all: build

## build: compile volt with the bundled bat binary baked in
build: embed
	@mkdir -p bin
	go build -ldflags="$(VOLT_LDFLAGS)" -o bin/$(BIN) ./cmd/volt

## embed: build the upstream bat binary and copy it into the embed slot
embed: $(EMBED_BIN)

$(EMBED_BIN): $(BAT_BUILD)
	@mkdir -p $(EMBED_DIR)
	cp $(BAT_BUILD) $(EMBED_BIN)

## bat: build the vendored tshakalekholoane/bat binary
bat: $(BAT_BUILD)

$(BAT_BUILD): | submodule
	@mkdir -p $(BAT_DIR)/bin
	cd $(BAT_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags="$(BAT_LDFLAGS)" -o bin/bat .

## submodule: ensure third_party/bat source is present
submodule:
	@if [ ! -f $(BAT_DIR)/go.mod ]; then \
		echo "Initialising bat submodule…"; \
		git submodule update --init --recursive $(BAT_DIR); \
	fi

## run: launch the TUI (builds first)
run: build
	./bin/$(BIN)

## tidy: refresh go.sum
tidy:
	go mod tidy

## clean: remove compiled binaries and embed slot
clean:
	rm -rf bin
	rm -f $(BAT_BUILD)
	rm -f $(EMBED_BIN)
