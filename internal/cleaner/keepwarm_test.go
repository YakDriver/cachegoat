package cleaner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YakDriver/cachegoat/internal/config"
)

// writeFileAged writes a file and sets its access and modification times to
// the given age relative to now.
func writeFileAged(t *testing.T, path string, mode os.FileMode, atime, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, atime, mtime); err != nil {
		t.Fatal(err)
	}
}

func atimeOf(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fileATime(info)
}

func TestKeepWarmTouchesIdleFile(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "idle.bin")
	oldTime := time.Now().Add(-5 * 24 * time.Hour)
	writeFileAged(t, f, 0644, oldTime, oldTime)

	c := New(&config.Config{}, false, false)
	touched, scanned := c.keepWarm(tmp)

	if touched != 1 || scanned != 1 {
		t.Fatalf("touched=%d scanned=%d, want 1/1", touched, scanned)
	}
	if got := atimeOf(t, f); time.Since(got) > time.Minute {
		t.Errorf("access time not refreshed: %v", got)
	}
	// Modification time must be preserved so Go's own build-cache trim is undisturbed.
	info, _ := os.Stat(f)
	if delta := info.ModTime().Sub(oldTime); delta < -time.Second || delta > time.Second {
		t.Errorf("mod time changed: got %v, want ~%v", info.ModTime(), oldTime)
	}
}

func TestKeepWarmSkipsRecentFile(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "recent.bin")
	// Accessed just now, but written long ago.
	writeFileAged(t, f, 0644, time.Now(), time.Now().Add(-10*24*time.Hour))

	c := New(&config.Config{}, false, false)
	touched, scanned := c.keepWarm(tmp)

	if scanned != 1 {
		t.Fatalf("scanned=%d, want 1", scanned)
	}
	if touched != 0 {
		t.Errorf("touched=%d, want 0 (recently accessed file should be skipped)", touched)
	}
}

// TestKeepWarmReadOnlyFile proves keep-warm can refresh read-only files, which
// is how Go stores module cache entries (mode 0444).
func TestKeepWarmReadOnlyFile(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "readonly.bin")
	oldTime := time.Now().Add(-5 * 24 * time.Hour)
	writeFileAged(t, f, 0444, oldTime, oldTime)

	c := New(&config.Config{}, false, false)
	touched, _ := c.keepWarm(tmp)

	if touched != 1 {
		t.Fatalf("touched=%d, want 1 (read-only file should still be warmed)", touched)
	}
	if got := atimeOf(t, f); time.Since(got) > time.Minute {
		t.Errorf("access time not refreshed on read-only file: %v", got)
	}
}

func TestKeepWarmDryRun(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "idle.bin")
	oldTime := time.Now().Add(-5 * 24 * time.Hour)
	writeFileAged(t, f, 0644, oldTime, oldTime)

	c := New(&config.Config{}, true, false) // dry-run
	touched, _ := c.keepWarm(tmp)

	if touched != 1 {
		t.Fatalf("touched=%d, want 1 (dry-run should still report)", touched)
	}
	// Access time must be unchanged in dry-run.
	if got := atimeOf(t, f); time.Since(got) < 4*24*time.Hour {
		t.Errorf("dry-run must not modify access time, got %v", got)
	}
}

func TestKeepWarmEmptyPath(t *testing.T) {
	c := New(&config.Config{}, false, false)
	if touched, scanned := c.keepWarm(""); touched != 0 || scanned != 0 {
		t.Errorf("empty path: touched=%d scanned=%d, want 0/0", touched, scanned)
	}
}

// TestRunKeepsWarm exercises the full Run wiring: when a cache is under its
// size threshold (no purge), idle files are kept warm.
func TestRunKeepsWarm(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "idle.bin")
	oldTime := time.Now().Add(-5 * 24 * time.Hour)
	writeFileAged(t, f, 0644, oldTime, oldTime)

	cfg := &config.Config{
		BuildCache:    config.CacheConfig{Path: tmp, MaxSizeGB: 999}, // won't purge
		ProtectBuilds: false,
		KeepWarm:      true,
	}
	c := New(cfg, false, false)
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}

	if got := atimeOf(t, f); time.Since(got) > time.Minute {
		t.Errorf("Run did not keep idle file warm: %v", got)
	}
}

// TestRunKeepWarmDisabled verifies the toggle is honored.
func TestRunKeepWarmDisabled(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "idle.bin")
	oldTime := time.Now().Add(-5 * 24 * time.Hour)
	writeFileAged(t, f, 0644, oldTime, oldTime)

	cfg := &config.Config{
		BuildCache:    config.CacheConfig{Path: tmp, MaxSizeGB: 999},
		ProtectBuilds: false,
		KeepWarm:      false,
	}
	c := New(cfg, false, false)
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}

	if got := atimeOf(t, f); time.Since(got) < 4*24*time.Hour {
		t.Errorf("keep-warm disabled but access time changed: %v", got)
	}
}
