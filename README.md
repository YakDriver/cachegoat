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
- Warns about large cache sizes
- Checks if scheduled cleanup is configured
- Shows OS-specific scheduling instructions

## Configuration

cachegoat uses this priority order:
1. Environment variables
2. `~/.cachegoat.yml`
3. `GOCACHE`/`GOMODCACHE` environment variables
4. `go env` output

### Environment Variables

| Variable | Description |
|----------|-------------|
| `CACHEGOAT_BUILD_PATH` | Build cache path (overrides GOCACHE) |
| `CACHEGOAT_MOD_PATH` | Module cache path (overrides GOMODCACHE) |

### Config File

Create `~/.cachegoat.yml`:

```yaml
build_cache:
  path: /tmp/go-cache
  max_size_gb: 30        # purge when cache exceeds this size

mod_cache:
  path: /tmp/go-mod-cache
  max_size_gb: 10        # prune when cache exceeds this size
  max_age_days: 7        # only remove files older than this

protect_builds: true     # skip cleanup if go build/test is running
log_path: /tmp/cachegoat.log
```

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
