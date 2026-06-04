BINARY   := zenith-wallpaper
DESTBIN  := $(HOME)/.local/bin/$(BINARY)
UNITDIR  := $(HOME)/.config/systemd/user

MISE     := $(shell command -v mise 2>/dev/null || echo mise)
GO       := $(MISE) exec go -- go

.PHONY: all build install install-units uninstall clean release

all: build

build:
	$(GO) build -o $(BINARY) .

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

# Release: create vX.Y tag and push to trigger RPM build in kter/linux-pkg
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
	@echo "Released v$(VERSION) — RPM build triggered in kter/linux-pkg"
