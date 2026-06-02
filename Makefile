BINARY   := zenith-wallpaper
DESTBIN  := $(HOME)/.local/bin/$(BINARY)
UNITDIR  := $(HOME)/.config/systemd/user

MISE     := $(shell command -v mise 2>/dev/null || echo mise)
GO       := $(MISE) exec go -- go

.PHONY: all build install install-units uninstall clean

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
