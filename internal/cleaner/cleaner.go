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
	if c.cfg.LogPath != "" && !c.dryRun {
		if f, err := os.OpenFile(c.cfg.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			c.log = f
			defer func() { _ = f.Close() }()
		}
	}

	var buildPurged, modPurged bool
	if c.cfg.ProtectBuilds && !c.force && goBuildActive() {
		// A build is running: skip the destructive purge, but still keep the
		// cache warm below. Refreshing access times is harmless mid-build and
		// is exactly when idle dependencies most need protecting.
		c.logf("Go build active, skipping cache purge (use --force to override)")
	} else {
		buildPurged = c.cleanBuildCache()
		modPurged = c.cleanModCache()
	}

	// Keep surviving cache files warm so OS temp cleaners don't prune them and
	// leave the cache half-populated. This runs regardless of build activity.
	// Skip a cache that was just purged: it is empty (or nearly so), and there
	// is nothing worth keeping warm.
	if c.cfg.KeepWarm {
		if !buildPurged {
			c.keepWarm(c.cfg.BuildCache.Path)
		}
		if !modPurged {
			c.keepWarm(c.cfg.ModCache.Path)
		}
	}
	return nil
}

func (c *Cleaner) cleanBuildCache() bool {
	path := c.cfg.BuildCache.Path
	if path == "" {
		return false
	}
	sizeGB := dirSizeGB(path)
	c.logf("build cache: %s (%.1fGB)", path, sizeGB)

	if sizeGB >= float64(c.cfg.BuildCache.MaxSizeGB) {
		c.logf("purging build cache (>=%.0fGB threshold)", float64(c.cfg.BuildCache.MaxSizeGB))
		if !c.dryRun {
			_ = exec.Command("go", "clean", "-cache").Run()
		}
		return true
	}
	return false
}

func (c *Cleaner) cleanModCache() bool {
	path := c.cfg.ModCache.Path
	if path == "" {
		return false
	}
	sizeGB := dirSizeGB(path)
	c.logf("mod cache: %s (%.1fGB)", path, sizeGB)

	if sizeGB >= float64(c.cfg.ModCache.MaxSizeGB) {
		c.logf("purging mod cache (>=%.0fGB threshold)", float64(c.cfg.ModCache.MaxSizeGB))
		if !c.dryRun {
			_ = exec.Command("go", "clean", "-modcache").Run()
		}
		return true
	}
	return false
}

func (c *Cleaner) logf(format string, args ...any) {
	msg := fmt.Sprintf("%s: %s", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
	fmt.Println(msg)
	if c.log != nil {
		_, _ = fmt.Fprintln(c.log, msg)
	}
}

// goBuildActive reports whether a Go build/test/install/run process is
// currently running. It is a var so tests can substitute it.
var goBuildActive = func() bool {
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
