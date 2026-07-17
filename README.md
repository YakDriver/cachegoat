# cachegoat

**Keep Go's caches from eating your disk — and dragging down your builds.** Go never cleans up after itself: on a large codebase with regularly-updated dependencies, the build and module caches quietly grow to tens of gigabytes (sometimes terabytes). cachegoat purges them automatically once they cross a size limit you set, and helps you relocate them to `/tmp` to escape antivirus scan thrash from the likes of CrowdStrike Falcon.

```bash
go install github.com/YakDriver/cachegoat@latest
cachegoat --recommend   # detects your setup, moves caches off the AV hot path, and schedules cleanup
```

**Why you'd want it**

- **Unbounded growth.** Go keeps every build artifact and module version forever. cachegoat caps that, purging only when a cache exceeds the size you configure.
- **Faster builds under antivirus.** Real-time scanners inspect every cache read and write. Moving caches to `/tmp` sidesteps that scan path — a real speedup on locked-down machines.
- **Set and forget.** Runs on a schedule, never cleans mid-build, and keeps `/tmp` caches usable so relocating them doesn't break your builds.

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

With `keep_warm` enabled (the default), each run refreshes the access time of idle cache files so the cleaner never considers them old enough to delete. Only idle files are touched, and the modification time is preserved (the access and inode-change times are advanced), so Go's own build-cache trimming is unaffected. Caches that just crossed their size threshold are purged first and skipped, so keep-warm never fights the size-based cleanup or adds disk usage.

Keep-warm runs even while a build is active (`protect_builds` only defers the destructive purge) — an active build is exactly when idle dependencies most need protecting. Because it runs every 2 hours by default and refreshes files after a single idle day, `/tmp` caches stay usable indefinitely between size-based purges, with two days of margin before the cleaner's 3-day cutoff.

## Troubleshooting: `no such file or directory` during a build

If a build suddenly fails with something like this, even though you changed nothing:

```
# github.com/example/foo
open /tmp/go-mod-cache/github.com/example/foo@v1.2.3/bar.go: no such file or directory
```

…your module cache is probably in a **half-populated state**. When caches live under `/tmp`, the OS temp cleaner deletes files that have gone untouched past its cutoff (3 days on macOS by default), which can leave a module directory that still exists but is missing some of its files. Go assumes an extracted module directory is complete and won't re-extract it, so the build fails with an unhelpful `no such file or directory` (or `cannot find package`).

`keep_warm` (on by default) prevents this in normal day-to-day use, but you're more exposed if:

- **You've tuned the timing.** Both the OS cleaner's cutoff and cachegoat's schedule are configurable. A shorter cleaner cutoff or a less frequent cachegoat run shrinks the safety margin.
- **The machine sat idle past the cutoff.** Coming back from vacation is the classic case — a project's dependencies may not have been touched in well over 3 days.
- **Scheduled runs were missed.** If cachegoat isn't scheduled, or its runs were skipped while the machine was off or asleep across the cutoff, idle files can age out before keep-warm refreshes them.

**The fix is to wipe the affected cache so Go re-extracts it cleanly:**

```bash
go clean -cache -modcache
```

Then rebuild — Go re-downloads and re-extracts whatever it needs. (`go clean -modcache` alone usually does it, since the module cache is the fragile one; the build cache tolerates missing entries and simply rebuilds them.)

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
