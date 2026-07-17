//go:build !darwin && !linux

package cleaner

import (
	"io/fs"
	"time"
)

// fileATime reports a file's access time. Platforms without a real
// implementation land here. They have no /tmp-style cleaner for keep-warm to
// defend against (and cachegoat does not schedule on them), so we report the
// current time: keep-warm then treats every file as recently accessed and does
// nothing. Returning the real modification time instead would be worse — since
// keep-warm preserves ModTime, every idle file would look idle forever and get
// re-touched on every run, never converging.
func fileATime(_ fs.FileInfo) time.Time {
	return time.Now()
}
