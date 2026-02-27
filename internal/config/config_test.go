package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := defaults()

	if cfg.BuildCache.MaxSizeGB != 30 {
		t.Errorf("expected build max 30GB, got %d", cfg.BuildCache.MaxSizeGB)
	}
	if cfg.ModCache.MaxSizeGB != 10 {
		t.Errorf("expected mod max 10GB, got %d", cfg.ModCache.MaxSizeGB)
	}
	if cfg.ModCache.MaxAgeDays != 7 {
		t.Errorf("expected mod max age 7 days, got %d", cfg.ModCache.MaxAgeDays)
	}
	if !cfg.ProtectBuilds {
		t.Error("expected protect_builds true by default")
	}
}

func TestLoadFromYAML(t *testing.T) {
	tmp := t.TempDir()
	home := os.Getenv("HOME")
	t.Setenv("HOME", tmp)
	t.Cleanup(func() { _ = os.Setenv("HOME", home) })

	yaml := `
build_cache:
  path: /custom/build
  max_size_gb: 50
mod_cache:
  path: /custom/mod
  max_size_gb: 20
  max_age_days: 14
protect_builds: false
`
	if err := os.WriteFile(filepath.Join(tmp, ".cachegoat.yml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.BuildCache.Path != "/custom/build" {
		t.Errorf("expected /custom/build, got %s", cfg.BuildCache.Path)
	}
	if cfg.BuildCache.MaxSizeGB != 50 {
		t.Errorf("expected 50GB, got %d", cfg.BuildCache.MaxSizeGB)
	}
	if cfg.ModCache.MaxAgeDays != 14 {
		t.Errorf("expected 14 days, got %d", cfg.ModCache.MaxAgeDays)
	}
	if cfg.ProtectBuilds {
		t.Error("expected protect_builds false")
	}
}

func TestEnvOverrides(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CACHEGOAT_BUILD_PATH", "/env/build")
	t.Setenv("CACHEGOAT_MOD_PATH", "/env/mod")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.BuildCache.Path != "/env/build" {
		t.Errorf("expected /env/build, got %s", cfg.BuildCache.Path)
	}
	if cfg.ModCache.Path != "/env/mod" {
		t.Errorf("expected /env/mod, got %s", cfg.ModCache.Path)
	}
}

func TestConfigString(t *testing.T) {
	cfg := defaults()
	cfg.BuildCache.Path = "/test/path"

	s := cfg.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
	if !contains(s, "/test/path") {
		t.Error("expected path in output")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
