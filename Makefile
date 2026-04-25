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

.PHONY: all build bat embed run tidy clean submodule update-bat verify-bat

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

MANIFEST := internal/batbin/BAT_VERSION

## update-bat: download a new upstream bat release into the embed slot
update-bat:
ifndef VERSION
	$(error VERSION is required, e.g. make update-bat VERSION=v0.10.0)
endif
	@URL="https://github.com/tshakalekholoane/bat/releases/download/$(VERSION)/bat"; \
	 SHA_URL="$$URL.sha256"; \
	 echo "→ Fetching bat $(VERSION) from $$URL…"; \
	 curl -fsSL -o $(EMBED_BIN).tmp "$$URL"; \
	 echo "→ Fetching expected sha256…"; \
	 EXPECTED=$$(curl -fsSL "$$SHA_URL" | awk '{print $$1}'); \
	 ACTUAL=$$(shasum -a 256 $(EMBED_BIN).tmp | awk '{print $$1}'); \
	 if [ -n "$$EXPECTED" ] && [ "$$EXPECTED" != "$$ACTUAL" ]; then \
	   echo "✗ sha256 mismatch: expected=$$EXPECTED actual=$$ACTUAL"; \
	   rm -f $(EMBED_BIN).tmp; \
	   exit 1; \
	 fi; \
	 if [ -z "$$EXPECTED" ]; then \
	   echo "⚠ upstream did not publish $$SHA_URL; recording locally computed sha256"; \
	 fi; \
	 mv $(EMBED_BIN).tmp $(EMBED_BIN); \
	 chmod 0755 $(EMBED_BIN); \
	 DATE=$$(date -u +%Y-%m-%d); \
	 printf "tag: %s\nsha256: %s\nurl: %s\nfetched: %s\n" \
	   "$(VERSION)" "$$ACTUAL" "$$URL" "$$DATE" > $(MANIFEST); \
	 echo "→ Done. Review $(MANIFEST), run \`go test ./internal/batbin/...\`, and commit."

## verify-bat: confirm the committed binary matches the manifest sha256
verify-bat:
	@MANIFEST_SHA=$$(grep '^sha256:' $(MANIFEST) | awk '{print $$2}'); \
	 ACTUAL_SHA=$$(shasum -a 256 $(EMBED_BIN) | awk '{print $$1}'); \
	 if [ "$$MANIFEST_SHA" = "$$ACTUAL_SHA" ]; then \
	   echo "✓ $(EMBED_BIN) matches $(MANIFEST)"; \
	 else \
	   echo "✗ sha256 mismatch: manifest=$$MANIFEST_SHA actual=$$ACTUAL_SHA"; \
	   exit 1; \
	 fi
