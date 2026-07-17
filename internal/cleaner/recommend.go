package cleaner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/YakDriver/cachegoat/internal/config"
)

func Recommend(cfg *config.Config) {
	fmt.Println("cachegoat recommendations:")
	fmt.Println()

	// OS info
	fmt.Printf("System: %s/%s\n\n", runtime.GOOS, runtime.GOARCH)

	// Check for CrowdStrike
	if hasCrowdStrike() {
		fmt.Println("⚠️  CrowdStrike detected")
		needsUpdate := false
		if !strings.HasPrefix(cfg.BuildCache.Path, "/tmp") {
			fmt.Printf("   → Consider moving build cache to /tmp to avoid scanning\n")
			fmt.Printf("     Current: %s\n", cfg.BuildCache.Path)
			needsUpdate = true
		} else {
			fmt.Println("   ✓ Build cache already in /tmp (good)")
		}
		if !strings.HasPrefix(cfg.ModCache.Path, "/tmp") {
			fmt.Printf("   → Consider moving mod cache to /tmp\n")
			fmt.Printf("     Current: %s\n", cfg.ModCache.Path)
			needsUpdate = true
		} else {
			fmt.Println("   ✓ Mod cache already in /tmp (good)")
		}

		if needsUpdate {
			fmt.Print("\n❓ Apply CrowdStrike recommendations, including updating env vars in your shell profile? (y/N): ")
			var response string
			_, _ = fmt.Scanln(&response)
			if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
				applyCacheRecommendations()
			}
		}
		fmt.Println()
	} else {
		fmt.Println("✓ No CrowdStrike detected")
		fmt.Println()
	}

	// Check cache sizes
	buildSize := dirSizeGB(cfg.BuildCache.Path)
	modSize := dirSizeGB(cfg.ModCache.Path)

	if buildSize > 50 {
		fmt.Printf("⚠️  Build cache is large (%.1fGB)\n", buildSize)
		fmt.Printf("   → Consider lowering max_size_gb threshold (currently %d)\n\n", cfg.BuildCache.MaxSizeGB)
	}

	if modSize > 5 {
		fmt.Printf("⚠️  Mod cache is %.1fGB\n", modSize)
		fmt.Printf("   → Run 'go clean -modcache' if you have stale dependencies\n\n")
	}

	// Check scheduling
	if !hasScheduledCleanup() {
		fmt.Println("\n⚠️  No scheduled cleanup detected")
		fmt.Print("❓ Set up automatic scheduled cleanup? (y/N): ")
		var response string
		_, _ = fmt.Scanln(&response)
		if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
			if err := Schedule(); err != nil {
				fmt.Printf("❌ Failed to schedule cleanup: %v\n", err)
			} else {
				fmt.Println("✅ Scheduled cleanup configured successfully!")
			}
		} else {
			fmt.Println("\nManual setup instructions:")
			printScheduleInstructions()
		}
	} else {
		fmt.Println("\n✓ Scheduled cleanup detected")
		if sched, cur := scheduledBinary(), currentBinary(); sched != "" && cur != "" && sched != cur {
			fmt.Printf("⚠️  Scheduled cleanup runs a different binary than the cachegoat on your PATH:\n")
			fmt.Printf("     scheduled: %s\n", sched)
			fmt.Printf("     on PATH:   %s\n", cur)
			fmt.Printf("   → Re-run 'cachegoat --unschedule && cachegoat --schedule' to point it at the current binary.\n")
		}
	}

	fmt.Println("\nCurrent config:")
	fmt.Print(cfg.String())
}

func printScheduleInstructions() {
	bin := findBinary()
	switch runtime.GOOS {
	case "darwin":
		fmt.Println("   → Create ~/Library/LaunchAgents/com.cachegoat.plist:")
		fmt.Printf(`
<?xml version="1.0" encoding="UTF-8"?>
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

   Then run: launchctl load ~/Library/LaunchAgents/com.cachegoat.plist
`, bin)
	case "linux":
		if hasSystemd() {
			fmt.Println("   → systemd detected, create ~/.config/systemd/user/cachegoat.service:")
			fmt.Printf(`
[Unit]
Description=Go cache cleanup

[Service]
ExecStart=%s

   And ~/.config/systemd/user/cachegoat.timer:

[Unit]
Description=Run cachegoat every 2 hours

[Timer]
OnBootSec=15min
OnUnitActiveSec=2h

[Install]
WantedBy=timers.target

   Then run: systemctl --user enable --now cachegoat.timer
`, bin)
		} else {
			fmt.Println("   → Add to crontab (crontab -e):")
			fmt.Printf("   0 */2 * * * %s\n", bin)
		}
	default:
		fmt.Println("   → Add to your system scheduler to run every 2 hours")
	}
}

func findBinary() string {
	if path, err := exec.LookPath("cachegoat"); err == nil {
		return path
	}
	return "/path/to/cachegoat"
}

// currentBinary returns the resolved path of the cachegoat that would run when
// invoked normally: the one on PATH, or the running binary as a fallback.
func currentBinary() string {
	if p, err := exec.LookPath("cachegoat"); err == nil {
		return evalSymlinks(p)
	}
	exe, _ := os.Executable()
	return evalSymlinks(exe)
}

// scheduledBinary returns the resolved path of the binary the scheduler is
// configured to run, or "" if it can't be determined.
func scheduledBinary() string {
	switch runtime.GOOS {
	case "darwin":
		return scheduledBinaryLaunchd()
	case "linux":
		if hasSystemd() {
			return scheduledBinarySystemd()
		}
		return scheduledBinaryCron()
	}
	return ""
}

var launchdProgramArg = regexp.MustCompile(`(?s)<key>ProgramArguments</key>\s*<array>\s*<string>(.*?)</string>`)

func scheduledBinaryLaunchd() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, "Library/LaunchAgents/com.cachegoat.plist"))
	if err != nil {
		return ""
	}
	m := launchdProgramArg.FindStringSubmatch(string(data))
	if len(m) != 2 {
		return ""
	}
	return evalSymlinks(strings.TrimSpace(m[1]))
}

func scheduledBinarySystemd() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".config/systemd/user/cachegoat.service"))
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "ExecStart="); ok {
			if fields := strings.Fields(rest); len(fields) > 0 {
				return evalSymlinks(fields[0])
			}
		}
	}
	return ""
}

func scheduledBinaryCron() string {
	out, _ := exec.Command("crontab", "-l").Output()
	for line := range strings.SplitSeq(string(out), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") || !strings.Contains(t, "cachegoat") {
			continue
		}
		// cron format: minute hour dom month dow command...
		if fields := strings.Fields(t); len(fields) >= 6 {
			return evalSymlinks(fields[5])
		}
	}
	return ""
}

func hasSystemd() bool {
	return exec.Command("systemctl", "--version").Run() == nil
}

func hasCrowdStrike() bool {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("pgrep", "-q", "falcon").Run() == nil
	case "linux":
		return exec.Command("pgrep", "-q", "falcon-sensor").Run() == nil
	}
	return false
}

func hasScheduledCleanup() bool {
	if runtime.GOOS == "darwin" {
		out, _ := exec.Command("launchctl", "list").Output()
		return strings.Contains(string(out), "cachegoat")
	}
	out, _ := exec.Command("crontab", "-l").Output()
	return strings.Contains(string(out), "cachegoat")
}

func applyCacheRecommendations() {
	fmt.Println("\n🔧 Applying cache recommendations...")

	// Find shell profile files and update them
	updated := false

	// Check common profile files
	home, _ := os.UserHomeDir()
	profileFiles := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".zprofile"),                // zsh login profile
		filepath.Join(home, ".zshenv"),                  // zsh environment (always sourced)
		filepath.Join(home, ".bash_login"),              // bash login alternative
		filepath.Join(home, ".config/fish/config.fish"), // fish shell
		filepath.Join(home, ".tcshrc"),                  // tcsh/csh
		filepath.Join(home, ".cshrc"),                   // csh
		filepath.Join(home, ".kshrc"),                   // korn shell
	}

	for _, profileFile := range profileFiles {
		if updateProfileFile(profileFile) {
			updated = true
		}
	}

	if !updated {
		// Fallback to go env -w if no profile files were updated
		if err := exec.Command("go", "env", "-w", "GOCACHE=/tmp/go-cache").Run(); err != nil {
			fmt.Printf("❌ Failed to set GOCACHE: %v\n", err)
			return
		}

		if err := exec.Command("go", "env", "-w", "GOMODCACHE=/tmp/go-mod-cache").Run(); err != nil {
			fmt.Printf("❌ Failed to set GOMODCACHE: %v\n", err)
			return
		}

		fmt.Println("✅ Cache paths set using 'go env -w'")
		fmt.Println("   → GOCACHE=/tmp/go-cache")
		fmt.Println("   → GOMODCACHE=/tmp/go-mod-cache")
	}

	fmt.Println("\n💡 Restart your shell or run 'source <profile-file>' to apply changes")
}

func updateProfileFile(profileFile string) bool {
	data, err := os.ReadFile(profileFile)
	if err != nil {
		return false // File doesn't exist or can't read
	}

	content := string(data)
	updated := false
	isFish := strings.Contains(profileFile, "fish")

	// Update GOCACHE if found
	if strings.Contains(content, "GOCACHE") {
		var re *regexp.Regexp
		var replacement string

		if isFish {
			re = regexp.MustCompile(`set -x GOCACHE .*`)
			replacement = "set -x GOCACHE /tmp/go-cache"
		} else {
			re = regexp.MustCompile(`export\s+GOCACHE=.*`)
			replacement = "export GOCACHE=/tmp/go-cache"
		}

		if re.MatchString(content) {
			content = re.ReplaceAllString(content, replacement)
			updated = true
			fmt.Printf("✅ Updated GOCACHE in %s\n", profileFile)
		}
	}

	// Update GOMODCACHE if found
	if strings.Contains(content, "GOMODCACHE") {
		var re *regexp.Regexp
		var replacement string

		if isFish {
			re = regexp.MustCompile(`set -x GOMODCACHE .*`)
			replacement = "set -x GOMODCACHE /tmp/go-mod-cache"
		} else {
			re = regexp.MustCompile(`export\s+GOMODCACHE=.*`)
			replacement = "export GOMODCACHE=/tmp/go-mod-cache"
		}

		if re.MatchString(content) {
			content = re.ReplaceAllString(content, replacement)
			updated = true
			fmt.Printf("✅ Updated GOMODCACHE in %s\n", profileFile)
		}
	}

	if updated {
		if err := os.WriteFile(profileFile, []byte(content), 0644); err != nil {
			fmt.Printf("❌ Failed to write %s: %v\n", profileFile, err)
			return false
		}
	}

	return updated
}
