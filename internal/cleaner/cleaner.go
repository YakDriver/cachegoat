package cleaner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/YakDriver/cachegoat/internal/config"
)

type Cleaner struct {
	cfg    *config.Config
	dryRun bool
	force  bool
	log    *os.File
}

func New(cfg *config.Config, dryRun, force bool) *Cleaner {
	return &Cleaner{cfg: cfg, dryRun: dryRun, force: force}
}

func (c *Cleaner) Run() error {
	if c.cfg.ProtectBuilds && !c.force && goBuildActive() {
		fmt.Println("Go build active, skipping (use --force to override)")
		return nil
	}

	if c.cfg.LogPath != "" && !c.dryRun {
		if f, err := os.OpenFile(c.cfg.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			c.log = f
			defer func() { _ = f.Close() }()
		}
	}

	c.cleanBuildCache()
	c.cleanModCache()
	return nil
}

func (c *Cleaner) cleanBuildCache() {
	path := c.cfg.BuildCache.Path
	if path == "" {
		return
	}
	sizeGB := dirSizeGB(path)
	c.logf("build cache: %s (%.1fGB)", path, sizeGB)

	if sizeGB >= float64(c.cfg.BuildCache.MaxSizeGB) {
		c.logf("purging build cache (>=%.0fGB threshold)", float64(c.cfg.BuildCache.MaxSizeGB))
		if !c.dryRun {
			_ = exec.Command("go", "clean", "-cache").Run()
		}
	}
}

func (c *Cleaner) cleanModCache() {
	path := c.cfg.ModCache.Path
	if path == "" {
		return
	}
	sizeGB := dirSizeGB(path)
	c.logf("mod cache: %s (%.1fGB)", path, sizeGB)

	if sizeGB >= float64(c.cfg.ModCache.MaxSizeGB) {
		c.logf("purging mod cache (>=%.0fGB threshold)", float64(c.cfg.ModCache.MaxSizeGB))
		if !c.dryRun {
			_ = exec.Command("go", "clean", "-modcache").Run()
		}
	}
}

func (c *Cleaner) logf(format string, args ...any) {
	msg := fmt.Sprintf("%s: %s", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
	fmt.Println(msg)
	if c.log != nil {
		_, _ = fmt.Fprintln(c.log, msg)
	}
}

func goBuildActive() bool {
	return exec.Command("pgrep", "-qf", "go (build|test|install|run)").Run() == nil
}

func dirSizeGB(path string) float64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return float64(size) / (1024 * 1024 * 1024)
}
