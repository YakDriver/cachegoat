package main

import "testing"

func TestEscapeModulePath(t *testing.T) {
	cases := map[string]string{
		"github.com/YakDriver/cachegoat": "github.com/!yak!driver/cachegoat",
		"github.com/BurntSushi/toml":     "github.com/!burnt!sushi/toml",
		"example.com/foo":                "example.com/foo",
		"gopkg.in/yaml.v3":               "gopkg.in/yaml.v3",
	}
	for in, want := range cases {
		if got := escapeModulePath(in); got != want {
			t.Errorf("escapeModulePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFirstProxy(t *testing.T) {
	cases := map[string]string{
		"https://proxy.golang.org,direct": "https://proxy.golang.org",
		"https://a|https://b":             "https://a",
		"https://corp.example.com/mod":    "https://corp.example.com/mod",
		"direct":                          "",
		"off":                             "",
	}
	for in, want := range cases {
		t.Setenv("GOPROXY", in)
		if got := firstProxy(); got != want {
			t.Errorf("firstProxy() with GOPROXY=%q = %q, want %q", in, got, want)
		}
	}
}

func TestVersionLess(t *testing.T) {
	type tc struct {
		a, b string
		want bool
	}
	cases := []tc{
		{"v0.2.0", "v0.3.0", true},
		{"v0.3.0", "v0.3.0", false},
		{"v0.3.1", "v0.3.0", false},
		{"v0.9.9", "v1.0.0", true},
		{"v1.0.0", "v0.9.9", false},
		{"v0.3.0", "v0.3.1", true},
	}
	for _, c := range cases {
		if got := versionLess(c.a, c.b); got != c.want {
			t.Errorf("versionLess(%q, %q) = %t, want %t", c.a, c.b, got, c.want)
		}
	}
}

func TestReleaseVersionRegex(t *testing.T) {
	release := []string{"v0.3.0", "v1.2.3", "v10.20.30"}
	notRelease := []string{"v0.3.0+dirty", "(devel)", "dev", "v1.2.3-rc1", "v0.3.1-0.20260717-abcdef", ""}

	for _, v := range release {
		if !releaseVersion.MatchString(v) {
			t.Errorf("expected %q to be a release version", v)
		}
	}
	for _, v := range notRelease {
		if releaseVersion.MatchString(v) {
			t.Errorf("expected %q NOT to be a release version", v)
		}
	}
}
