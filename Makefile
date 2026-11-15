BINARY   := zenith-wallpaper
DESTBIN  := $(HOME)/.local/bin/$(BINARY)
UNITDIR  := $(HOME)/.config/systemd/user

# Use mise when it is installed; otherwise the go on PATH (cloud agents, CI).
MISE     := $(shell command -v mise 2>/dev/null)
ifneq ($(MISE),)
GO       := $(MISE) exec go -- go
GOFMT    := $(MISE) exec go -- gofmt
else
GO       := go
GOFMT    := gofmt
endif

.PHONY: all build fmt test check install install-units uninstall clean release

all: build

build:
	$(GO) build -o $(BINARY) .

fmt:
	$(GOFMT) -w *.go

# Checks gofmt, runs vet + tests, then verifies the darwin build still compiles
# (platform-specific files are build-tagged; see AGENTS.md).
# `|| exit 1` keeps a missing or failing gofmt from passing as "no diffs".
test:
	@out=$$($(GOFMT) -l *.go) || exit 1; test -z "$$out" || { echo "gofmt needed (run make fmt):"; echo "$$out"; exit 1; }
	$(GO) vet ./...
	$(GO) test ./...
	GOOS=darwin GOARCH=arm64 $(GO) build -o /dev/null .
	GOOS=darwin GOARCH=arm64 $(GO) vet .

# Same checks as test (and CI); the entry point AGENTS.md names for agents.
check: test

install: build
	install -m 0755 $(BINARY) $(DESTBIN)

install-units:
	install -m 0644 $(BINARY).service $(UNITDIR)/$(BINARY).service
	install -m 0644 $(BINARY).timer   $(UNITDIR)/$(BINARY).timer
	systemctl --user daemon-reload
	systemctl --user enable --now $(BINARY).timer

uninstall:
	-systemctl --user disable --now $(BINARY).timer 2>/dev/null
	-rm -f $(UNITDIR)/$(BINARY).service $(UNITDIR)/$(BINARY).timer
	-systemctl --user daemon-reload
	-rm -f $(DESTBIN)

clean:
	rm -f $(BINARY)

# Release: start the Release workflow (release.yml) on main. It waits for
# approval on the `release` GitHub environment, then creates the vX.Y tag,
# triggers the RPM build in kter/linux-pkg and bumps kter/homebrew-tap.
release:
ifndef VERSION
	$(error VERSION is required, e.g. make release VERSION=1.1)
endif
	@git fetch -q origin
	@git rev-parse -q --verify "refs/tags/v$(VERSION)" >/dev/null && { echo "ERROR: tag v$(VERSION) already exists"; exit 1; } || true
	gh api -X POST repos/kter/zenith-wallpaper/actions/workflows/release.yml/dispatches -f ref=main -f 'inputs[version]=$(VERSION)'
	@echo "Release workflow started for v$(VERSION) on main — approve it in the 'release' environment: https://github.com/kter/zenith-wallpaper/actions/workflows/release.yml"
