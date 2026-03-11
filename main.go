package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/YakDriver/cachegoat/internal/cleaner"
	"github.com/YakDriver/cachegoat/internal/config"
)

const version = "v0.2.0"

func main() {
	dryRun := flag.Bool("dry-run", false, "show what would be cleaned without deleting")
	force := flag.Bool("force", false, "run even if Go build is active")
	showConfig := flag.Bool("config", false, "show resolved configuration")
	recommend := flag.Bool("recommend", false, "show setup recommendations")
	schedule := flag.Bool("schedule", false, "create and enable scheduled cleanup")
	unschedule := flag.Bool("unschedule", false, "remove scheduled cleanup")
	flag.Parse()

	checkLatestVersion()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if *schedule {
		if err := cleaner.Schedule(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *unschedule {
		if err := cleaner.Unschedule(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *recommend {
		cleaner.Recommend(cfg)
		return
	}

	if *showConfig {
		fmt.Print(cfg.String())
		return
	}

	c := cleaner.New(cfg, *dryRun, *force)
	if err := c.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func checkLatestVersion() {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/YakDriver/cachegoat/releases/latest")
	if err != nil {
		return // fail silently
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return // fail silently
	}

	if release.TagName != "" && release.TagName != version {
		fmt.Printf("New version available: %s (current: %s)\n", release.TagName, version)
	}
}
