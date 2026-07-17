# cachegoat

Automatic Go cache cleanup tool. Keeps your build and module caches from eating all your disk space.

## Installation

```bash
go install github.com/YakDriver/cachegoat@latest
```

## Usage

```bash
cachegoat               # run cleanup
cachegoat --dry-run     # show what would be cleaned
cachegoat --config      # show resolved configuration
cachegoat --force       # run even if Go build is active
cachegoat --help        # show usage
cachegoat --recommend   # show setup recommendations
cachegoat --schedule    # create and enable scheduled cleanup
cachegoat --unschedule  # remove scheduled cleanup
```

### Recommendations

The `--recommend` flag analyzes your setup and provides suggestions:

- Detects CrowdStrike and recommends moving caches to `/tmp` to avoid scanning overhead
- **Interactive setup**: Offers to automatically update cache paths in your shell profile
- Warns about large cache sizes
- Checks if scheduled cleanup is configured
- **One-click scheduling**: Offers to automatically set up scheduled cleanup
- Shows OS-specific scheduling instructions

Run `cachegoat --recommend` for a complete interactive setup experience that can configure both optimal cache paths and automatic scheduling.

## Configuration

cachegoat uses this priority order:
1. `CACHEGOAT_BUILD_PATH` and `CACHEGOAT_MOD_PATH` environment variables (highest priority)
2. `~/.cachegoat.yml` configuration file
3. `go env GOCACHE` and `go env GOMODCACHE` 
4. `GOCACHE` and `GOMODCACHE` environment variables (fallback)

### Environment Variables

| Variable | Description |
|----------|-------------|
| `CACHEGOAT_BUILD_PATH` | Build cache path (overrides GOCACHE) |
| `CACHEGOAT_MOD_PATH` | Module cache path (overrides GOMODCACHE) |

### Config File

Create `~/.cachegoat.yml`:

```yaml
build_cache:
  path: /tmp/go-cache      # optional: defaults to 'go env GOCACHE'
  max_size_gb: 30          # purge when cache exceeds this size

mod_cache:
  path: /tmp/go-mod-cache  # optional: defaults to 'go env GOMODCACHE'
  max_size_gb: 10          # purge when cache exceeds this size

protect_builds: true       # skip cleanup if go build/test is running
keep_warm: true            # refresh idle cache files so macOS/Linux temp cleaners don't prune them
log_path: /tmp/cachegoat.log
```

## Keeping /tmp caches warm

Storing caches under `/tmp` avoids CrowdStrike scanning overhead, but OS temp-directory cleaners prune `/tmp` on a schedule — macOS (`/usr/libexec/tmp_cleaner`) deletes files untouched for 3 days, and Linux's `systemd-tmpfiles` does the same on its own timer. When that happens to an in-use module cache, Go is left with half-populated `mod@version/` directories and builds fail with errors like `open .../foo.go: no such file or directory`. Go won't re-extract a directory it thinks already exists, so the only reliable fix is wiping the whole cache.

With `keep_warm` enabled (the default), each run refreshes the access time of idle cache files so the cleaner never considers them old enough to delete. Only idle files are touched, and only their access time is advanced — modification times are preserved, so Go's own build-cache trimming is unaffected. Caches that just crossed their size threshold are purged first and skipped, so keep-warm never fights the size-based cleanup or adds disk usage.

Keep-warm runs even while a build is active (`protect_builds` only defers the destructive purge) — an active build is exactly when idle dependencies most need protecting. Because it runs every 2 hours by default and refreshes files after a single idle day, `/tmp` caches stay usable indefinitely between size-based purges, with two days of margin before the cleaner's 3-day cutoff.

## Automatic Scheduling

The easiest way to set up scheduled cleanup:

```bash
cachegoat --schedule    # creates and enables scheduler for your OS
cachegoat --unschedule  # removes it
```

This automatically configures:
- macOS: launchd (runs every 2 hours)
- Linux with systemd: systemd timer
- Linux without systemd: cron

### Manual Setup

<details>
<summary>macOS (launchd)</summary>

Create `~/Library/LaunchAgents/com.cachegoat.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.cachegoat</string>
  <key>ProgramArguments</key>
  <array>
    <string>/Users/YOUR_USERNAME/go/bin/cachegoat</string>
  </array>
  <key>StartInterval</key>
  <integer>7200</integer>
</dict>
</plist>
```

Load it:
```bash
launchctl load ~/Library/LaunchAgents/com.cachegoat.plist
```
</details>

<details>
<summary>Linux (cron)</summary>

```bash
crontab -e
```

Add (runs every 2 hours):
```
0 */2 * * * /home/YOUR_USERNAME/go/bin/cachegoat
```
</details>

<details>
<summary>Linux (systemd timer)</summary>

Create `~/.config/systemd/user/cachegoat.service`:
```ini
[Unit]
Description=Go cache cleanup

[Service]
ExecStart=%h/go/bin/cachegoat
```

Create `~/.config/systemd/user/cachegoat.timer`:
```ini
[Unit]
Description=Run cachegoat every 2 hours

[Timer]
OnBootSec=15min
OnUnitActiveSec=2h

[Install]
WantedBy=timers.target
```

Enable:
```bash
systemctl --user enable --now cachegoat.timer
```
</details>

## License

MPL-2.0
