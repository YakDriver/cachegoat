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
			fmt.Scanln(&response)
			if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
				applyCacheRecommendations()
				return
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
		fmt.Println("⚠️  No scheduled cleanup detected")
		printScheduleInstructions()
		fmt.Println()
	} else {
		fmt.Println("✓ Scheduled cleanup detected")
		fmt.Println()
	}

	fmt.Println("Current config:")
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
	
	// Update GOCACHE if found
	if strings.Contains(content, "GOCACHE=") {
		// Replace existing GOCACHE export
		re := regexp.MustCompile(`export\s+GOCACHE=.*`)
		if re.MatchString(content) {
			content = re.ReplaceAllString(content, "export GOCACHE=/tmp/go-cache")
			updated = true
			fmt.Printf("✅ Updated GOCACHE in %s\n", profileFile)
		}
	}
	
	// Update GOMODCACHE if found
	if strings.Contains(content, "GOMODCACHE=") {
		// Replace existing GOMODCACHE export
		re := regexp.MustCompile(`export\s+GOMODCACHE=.*`)
		if re.MatchString(content) {
			content = re.ReplaceAllString(content, "export GOMODCACHE=/tmp/go-mod-cache")
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
