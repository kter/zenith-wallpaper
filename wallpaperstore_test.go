package main

import "testing"

const plistHeader = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">`

func TestSpacesOverrideCount(t *testing.T) {
	tests := []struct {
		name  string
		plist string
		want  int
	}{
		{
			// Two Spaces carry their own wallpaper; nested dicts inside each
			// Space config must not inflate the count.
			"two overrides",
			plistHeader + `
<dict>
	<key>AllSpacesAndDisplays</key>
	<dict>
		<key>Type</key><string>linked</string>
	</dict>
	<key>Spaces</key>
	<dict>
		<key>7D2FE1A0-0000-0000-0000-000000000001</key>
		<dict>
			<key>Default</key>
			<dict>
				<key>Content</key>
				<dict>
					<key>Choices</key>
					<array><dict><key>Provider</key><string>com.apple.wallpaper.choice.image</string></dict></array>
				</dict>
			</dict>
			<key>Type</key><string>individual</string>
		</dict>
		<key>7D2FE1A0-0000-0000-0000-000000000002</key>
		<dict>
			<key>Type</key><string>individual</string>
		</dict>
	</dict>
	<key>SystemDefault</key>
	<dict>
		<key>Type</key><string>individual</string>
	</dict>
</dict>
</plist>`,
			2,
		},
		{
			"empty Spaces dict",
			plistHeader + `
<dict>
	<key>Spaces</key>
	<dict/>
	<key>SystemDefault</key>
	<dict><key>Type</key><string>individual</string></dict>
</dict>
</plist>`,
			0,
		},
		{
			"no Spaces key",
			plistHeader + `
<dict>
	<key>AllSpacesAndDisplays</key>
	<dict><key>Type</key><string>linked</string></dict>
	<key>Displays</key>
	<dict>
		<key>DISPLAY-UUID</key>
		<dict><key>Type</key><string>individual</string></dict>
	</dict>
</dict>
</plist>`,
			0,
		},
		{
			// A "Spaces" key buried inside another entry's config must not
			// be mistaken for the root-level dictionary.
			"nested Spaces key ignored",
			plistHeader + `
<dict>
	<key>SystemDefault</key>
	<dict>
		<key>Spaces</key>
		<dict>
			<key>DECOY</key>
			<dict/>
		</dict>
	</dict>
</dict>
</plist>`,
			0,
		},
	}
	for _, tt := range tests {
		got, err := spacesOverrideCount([]byte(tt.plist))
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: count = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestSpacesOverrideCountMalformed(t *testing.T) {
	if _, err := spacesOverrideCount([]byte("not a plist")); err == nil {
		t.Error("expected error for malformed plist, got nil")
	}
	if _, err := spacesOverrideCount([]byte(plistHeader + "<dict><key>Spaces</key>")); err == nil {
		t.Error("expected error for truncated plist, got nil")
	}
}
