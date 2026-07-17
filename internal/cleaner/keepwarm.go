package cleaner

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// warmMaxIdle is how long a cache file may go untouched before keep-warm
// refreshes it. macOS deletes files under /tmp once their atime, mtime, and
// ctime are all older than 3 days, so refreshing at 1 day leaves two full days
// of margin before the daily cleaner runs.
const warmMaxIdle = 24 * time.Hour

// keepWarm refreshes the access time of cache files that have gone idle, so
// that OS temp-directory cleaners (such as macOS's tmp_cleaner) do not prune
// them out from under Go and leave the cache in a half-populated, unbuildable
// state.
//
// Only idle files are touched, and only their access time is advanced; the
// modification time is preserved so Go's own build-cache trimming continues to
// work. Files touched recently by builds are left untouched, keeping the cost
// proportional to the number of at-risk files rather than the whole cache.
func (c *Cleaner) keepWarm(path string) (touched, scanned int) {
	if path == "" {
		return 0, 0
	}

	now := time.Now()
	cutoff := now.Add(-warmMaxIdle)

	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		scanned++
		if fileATime(info).After(cutoff) {
			return nil // accessed recently, not at risk
		}
		if c.dryRun {
			touched++
			return nil
		}
		// Advance the access time to now; preserve the modification time.
		if err := os.Chtimes(p, now, info.ModTime()); err == nil {
			touched++
		}
		return nil
	})

	verb := "warmed"
	if c.dryRun {
		verb = "would warm"
	}
	c.logf("keep-warm: %s %d of %d files in %s", verb, touched, scanned, path)
	return touched, scanned
}
