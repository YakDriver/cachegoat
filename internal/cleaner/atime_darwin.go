//go:build darwin

package cleaner

import (
	"io/fs"
	"syscall"
	"time"
)

// fileATime returns the access time of a file, falling back to the
// modification time if the platform-specific data is unavailable.
func fileATime(info fs.FileInfo) time.Time {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return info.ModTime()
	}
	sec, nsec := st.Atimespec.Unix()
	return time.Unix(sec, nsec)
}
