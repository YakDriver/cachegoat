package config

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type CacheConfig struct {
	Path      string `yaml:"path"`
	MaxSizeGB int    `yaml:"max_size_gb"`
}

type Config struct {
	BuildCache    CacheConfig `yaml:"build_cache"`
	ModCache      CacheConfig `yaml:"mod_cache"`
	ProtectBuilds bool        `yaml:"protect_builds"`
	LogPath       string      `yaml:"log_path"`
}

func Load() (*Config, error) {
	cfg := defaults()

	// Load from ~/.cachegoat.yml if exists
	home, _ := os.UserHomeDir()
	if data, err := os.ReadFile(filepath.Join(home, ".cachegoat.yml")); err == nil {
		_ = yaml.Unmarshal(data, cfg)
	}

	// Fall back to go env if not set in config
	if cfg.BuildCache.Path == "" {
		cfg.BuildCache.Path = goEnv("GOCACHE")
	}
	if cfg.ModCache.Path == "" {
		cfg.ModCache.Path = goEnv("GOMODCACHE")
	}

	// Fall back to standard env vars if go env didn't work
	if cfg.BuildCache.Path == "" {
		cfg.BuildCache.Path = os.Getenv("GOCACHE")
	}
	if cfg.ModCache.Path == "" {
		cfg.ModCache.Path = os.Getenv("GOMODCACHE")
	}

	// Environment overrides (highest priority)
	if v := os.Getenv("CACHEGOAT_BUILD_PATH"); v != "" {
		cfg.BuildCache.Path = v
	}
	if v := os.Getenv("CACHEGOAT_MOD_PATH"); v != "" {
		cfg.ModCache.Path = v
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		BuildCache:    CacheConfig{MaxSizeGB: 30},
		ModCache:      CacheConfig{MaxSizeGB: 10},
		ProtectBuilds: true,
		LogPath:       "/tmp/cachegoat.log",
	}
}

func goEnv(key string) string {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (c *Config) String() string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(c)
	return buf.String()
}
