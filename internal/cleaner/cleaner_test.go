package cleaner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YakDriver/cachegoat/internal/config"
)

func TestDirSizeGB(t *testing.T) {
	tmp := t.TempDir()

	// Create 1MB file
	data := make([]byte, 1024*1024)
	if err := os.WriteFile(filepath.Join(tmp, "test.bin"), data, 0644); err != nil {
		t.Fatal(err)
	}

	size := dirSizeGB(tmp)
	expected := 1.0 / 1024 // 1MB in GB
	if size < expected*0.9 || size > expected*1.1 {
		t.Errorf("expected ~%.6fGB, got %.6f", expected, size)
	}
}

func TestDirSizeGB_Empty(t *testing.T) {
	tmp := t.TempDir()
	size := dirSizeGB(tmp)
	if size != 0 {
		t.Errorf("expected 0, got %f", size)
	}
}

func TestDirSizeGB_NonExistent(t *testing.T) {
	size := dirSizeGB("/nonexistent/path/12345")
	if size != 0 {
		t.Errorf("expected 0 for nonexistent path, got %f", size)
	}
}

func TestCleanerDryRun(t *testing.T) {
	tmp := t.TempDir()
	testFile := filepath.Join(tmp, "test.bin")
	if err := os.WriteFile(testFile, make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		BuildCache:    config.CacheConfig{Path: tmp, MaxSizeGB: 0}, // 0 = always clean
		ProtectBuilds: false,
	}

	c := New(cfg, true, false) // dry-run
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}

	// File should still exist in dry-run
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("file should not be deleted in dry-run mode")
	}
}

func TestCleanBuildCache(t *testing.T) {
	tmp := t.TempDir()
	testFile := filepath.Join(tmp, "test.bin")
	if err := os.WriteFile(testFile, make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		BuildCache:    config.CacheConfig{Path: tmp, MaxSizeGB: 0},
		ProtectBuilds: false,
		LogPath:       "",
	}

	c := New(cfg, false, false)
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}

	// Directory should be recreated but empty
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty dir after purge, got %d entries", len(entries))
	}
}

func TestCleanModCache_AgeBasedPruning(t *testing.T) {
	tmp := t.TempDir()

	oldFile := filepath.Join(tmp, "old.bin")
	newFile := filepath.Join(tmp, "new.bin")

	if err := os.WriteFile(oldFile, make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	// Make old file 10 days old
	oldTime := time.Now().AddDate(0, 0, -10)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		ModCache:      config.CacheConfig{Path: tmp, MaxSizeGB: 0, MaxAgeDays: 7},
		ProtectBuilds: false,
	}

	c := New(cfg, false, false)
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}

	// Old file should be deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old file should be deleted")
	}

	// New file should remain
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Error("new file should remain")
	}
}
