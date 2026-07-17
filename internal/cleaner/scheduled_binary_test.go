package cleaner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScheduledBinaryLaunchd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	agents := filepath.Join(home, "Library/LaunchAgents")
	if err := os.MkdirAll(agents, 0755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(home, "go", "bin", "cachegoat")
	if err := os.MkdirAll(filepath.Dir(bin), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("binary"), 0755); err != nil {
		t.Fatal(err)
	}

	plist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.cachegoat</string>
  <key>ProgramArguments</key>
  <array>
    <string>` + bin + `</string>
  </array>
  <key>StartInterval</key>
  <integer>7200</integer>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(agents, "com.cachegoat.plist"), []byte(plist), 0644); err != nil {
		t.Fatal(err)
	}

	got := scheduledBinaryLaunchd()
	if want := evalSymlinks(bin); got != want {
		t.Errorf("scheduledBinaryLaunchd() = %q, want %q", got, want)
	}
}

func TestScheduledBinaryLaunchdMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no plist present
	if got := scheduledBinaryLaunchd(); got != "" {
		t.Errorf("expected empty when no plist, got %q", got)
	}
}
