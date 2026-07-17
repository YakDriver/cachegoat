package cleaner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func Schedule() error {
	bin := scheduleBinaryPath()

	switch runtime.GOOS {
	case "darwin":
		return scheduleLaunchd(bin)
	case "linux":
		if hasSystemd() {
			return scheduleSystemd(bin)
		}
		return scheduleCron(bin)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// scheduleBinaryPath chooses which binary to schedule. It prefers the cachegoat
// on PATH — the one `go install` overwrites in place — so future upgrades are
// picked up without re-scheduling. If the running binary isn't on PATH (for
// example a one-off build in a source tree), it warns, because upgrades won't
// be reflected automatically.
func scheduleBinaryPath() string {
	exe, _ := os.Executable()
	exe = evalSymlinks(exe)

	if p, err := exec.LookPath("cachegoat"); err == nil {
		p = evalSymlinks(p)
		if exe != "" && p != exe {
			fmt.Printf("Note: scheduling the cachegoat on your PATH:\n  %s\n(not the binary you ran: %s)\n", p, exe)
		}
		return p
	}

	if exe != "" {
		fmt.Printf("Warning: cachegoat is not on your PATH; scheduling:\n  %s\nUpgrades via 'go install' won't be picked up automatically — re-run --schedule after upgrading.\n", exe)
		return exe
	}
	return findBinary()
}

// evalSymlinks resolves symlinks for stable path comparison, returning the
// input unchanged if it can't be resolved.
func evalSymlinks(p string) string {
	if p == "" {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func Unschedule() error {
	switch runtime.GOOS {
	case "darwin":
		return unscheduleLaunchd()
	case "linux":
		if hasSystemd() {
			return unscheduleSystemd()
		}
		return unscheduleCron()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func scheduleLaunchd(bin string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}
	plistPath := filepath.Join(home, "Library/LaunchAgents/com.cachegoat.plist")

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.cachegoat</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
  </array>
  <key>StartInterval</key>
  <integer>7200</integer>
</dict>
</plist>
`, bin)

	// Unload if exists
	_ = exec.Command("launchctl", "unload", plistPath).Run()

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("failed to load plist: %w", err)
	}

	fmt.Printf("✓ Scheduled cleanup every 2 hours\n  %s\n", plistPath)
	return nil
}

func unscheduleLaunchd() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}
	plistPath := filepath.Join(home, "Library/LaunchAgents/com.cachegoat.plist")

	_ = exec.Command("launchctl", "unload", plistPath).Run()
	_ = os.Remove(plistPath)

	fmt.Println("✓ Removed scheduled cleanup")
	return nil
}

func scheduleSystemd(bin string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}
	dir := filepath.Join(home, ".config/systemd/user")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd dir: %w", err)
	}

	service := fmt.Sprintf(`[Unit]
Description=Go cache cleanup

[Service]
ExecStart=%s
`, bin)

	timer := `[Unit]
Description=Run cachegoat every 2 hours

[Timer]
OnBootSec=15min
OnUnitActiveSec=2h

[Install]
WantedBy=timers.target
`

	if err := os.WriteFile(filepath.Join(dir, "cachegoat.service"), []byte(service), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "cachegoat.timer"), []byte(timer), 0644); err != nil {
		return err
	}

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	if err := exec.Command("systemctl", "--user", "enable", "--now", "cachegoat.timer").Run(); err != nil {
		return fmt.Errorf("failed to enable timer: %w", err)
	}

	fmt.Println("✓ Scheduled cleanup every 2 hours (systemd timer)")
	return nil
}

func unscheduleSystemd() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}
	dir := filepath.Join(home, ".config/systemd/user")

	_ = exec.Command("systemctl", "--user", "disable", "--now", "cachegoat.timer").Run()
	_ = os.Remove(filepath.Join(dir, "cachegoat.service"))
	_ = os.Remove(filepath.Join(dir, "cachegoat.timer"))
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	fmt.Println("✓ Removed scheduled cleanup")
	return nil
}

func scheduleCron(bin string) error {
	out, _ := exec.Command("crontab", "-l").Output()
	entry := fmt.Sprintf("0 */2 * * * %s\n", bin)

	if contains(string(out), "cachegoat") {
		return fmt.Errorf("cachegoat already in crontab")
	}

	newCron := string(out) + entry
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCron)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update crontab: %w", err)
	}

	fmt.Println("✓ Scheduled cleanup every 2 hours (cron)")
	return nil
}

func unscheduleCron() error {
	out, _ := exec.Command("crontab", "-l").Output()
	lines := strings.Split(string(out), "\n")
	var newLines []string
	for _, line := range lines {
		if !contains(line, "cachegoat") {
			newLines = append(newLines, line)
		}
	}

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(strings.Join(newLines, "\n"))
	_ = cmd.Run()

	fmt.Println("✓ Removed scheduled cleanup")
	return nil
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
