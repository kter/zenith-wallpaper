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

# Release: create vX.Y tag and push; release.yml then triggers the RPM build
# in kter/linux-pkg and bumps the Homebrew formula in kter/homebrew-tap
release:
ifndef VERSION
	$(error VERSION is required, e.g. make release VERSION=1.1)
endif
	@git diff --quiet || { echo "ERROR: working tree is dirty"; exit 1; }
	@test "$$(git rev-parse --abbrev-ref HEAD)" = "main" || { echo "ERROR: not on main branch"; exit 1; }
	@git fetch -q origin
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" || { echo "ERROR: main is not in sync with origin/main"; exit 1; }
	git tag v$(VERSION)
	git push origin v$(VERSION)
	@echo "Released v$(VERSION) — RPM build (kter/linux-pkg) and Homebrew bump (kter/homebrew-tap) triggered"
