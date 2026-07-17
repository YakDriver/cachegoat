package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// releaseVersion matches a clean release tag like "v1.2.3" (no prerelease or
// pseudo-version suffix). Update checks only fire for these.
var releaseVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// version returns a human-readable version derived from the binary's embedded
// build info, so it always matches the version that was installed — no
// constant to bump. A binary installed via `go install module@vX` reports that
// version exactly (proxy downloads carry no VCS metadata, so no commit suffix
// is added). A build from a source tree appends the short commit hash when
// available, and reports "dev" when no module version is embedded.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	v := info.Main.Version
	if v == "" || v == "(devel)" {
		v = "dev"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			v += " (" + s.Value[:7] + ")"
			break
		}
	}
	return v
}

// moduleInfo returns the module path and installed version from build info.
func moduleInfo() (path, current string) {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Path, info.Main.Version
	}
	return "", ""
}

// updateNotice returns a message when a newer release is available on the Go
// module proxy, or "" when up to date, on a dev build, or offline. It does not
// hit the GitHub API; it uses the same proxy `go install` uses, honoring
// GOPROXY.
func updateNotice() string {
	path, current := moduleInfo()
	if path == "" || !releaseVersion.MatchString(current) {
		return "" // dev build or pseudo-version: don't nag
	}

	latest := latestVersion(path)
	if !releaseVersion.MatchString(latest) || !versionLess(current, latest) {
		return ""
	}

	return fmt.Sprintf("A newer cachegoat is available: %s (installed %s)\n  Upgrade: go install %s@latest", latest, current, path)
}

// latestVersion asks the Go module proxy for the latest version of modulePath.
// It returns "" if the proxy is disabled, unreachable, or has no such module.
func latestVersion(modulePath string) string {
	base := firstProxy()
	if base == "" || modulePath == "" {
		return ""
	}

	url := strings.TrimRight(base, "/") + "/" + escapeModulePath(modulePath) + "/@latest"
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var out struct {
		Version string `json:"Version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ""
	}
	return out.Version
}

// firstProxy returns the first HTTP(S) endpoint in GOPROXY, or "" when module
// downloading is disabled (e.g. GOPROXY=off) or only "direct" is configured.
func firstProxy() string {
	proxy := os.Getenv("GOPROXY")
	if proxy == "" {
		if out, err := exec.Command("go", "env", "GOPROXY").Output(); err == nil {
			proxy = strings.TrimSpace(string(out))
		}
	}
	if proxy == "" {
		proxy = "https://proxy.golang.org"
	}

	for _, p := range strings.FieldsFunc(proxy, func(r rune) bool { return r == ',' || r == '|' }) {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "http://") {
			return p
		}
	}
	return ""
}

// escapeModulePath applies the Go module path case-encoding used by the proxy:
// each uppercase letter becomes "!" followed by its lowercase form.
func escapeModulePath(p string) string {
	var b strings.Builder
	for _, r := range p {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// versionLess reports whether release version a sorts before b. Both must match
// releaseVersion (vMAJOR.MINOR.PATCH).
func versionLess(a, b string) bool {
	pa, pb := parseRelease(a), parseRelease(b)
	for i := range 3 {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func parseRelease(v string) [3]int {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		out[i], _ = strconv.Atoi(parts[i])
	}
	return out
}
