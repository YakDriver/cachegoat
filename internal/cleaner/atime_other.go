//go:build !darwin && !linux

package cleaner

import (
	"io/fs"
	"time"
)

// fileATime returns the modification time as a portable fallback on platforms
// where the access time is not readily available.
func fileATime(info fs.FileInfo) time.Time {
	return info.ModTime()
}
