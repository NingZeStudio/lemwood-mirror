package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LauncherConfig describes how to discover a launcher's GitHub repo URL from a source page.
// If RepoSelector starts with "regex:", it will be treated as a regular expression to match anchor hrefs.
// If RepoSelector is empty, the first anchor href containing "github.com" will be used.
// SourceURL can directly be a GitHub repo URL (e.g., https://github.com/owner/repo), in which case selector is ignored.

type LauncherConfig struct {
	Name         string `json:"name"`
	SourceURL    string `json:"source_url"`
	RepoSelector string `json:"repo_selector"`
}

type Config struct {
	ServerAddress       string      `json:"server_address"`
	ServerPort          int         `json:"server_port"`
	CheckCron           string      `json:"check_cron"`
	StoragePath            string           `json:"storage_path"`
	GitHubToken            string           `json:"github_token"`
	ProxyURL               string           `json:"proxy_url"`
	AssetProxyURL          string           `json:"asset_proxy_url"`
	XgetDomain             string           `json:"xget_domain"`
	XgetEnabled            bool             `json:"xget_enabled"`
	DownloadTimeoutMinutes int    `json:"download_timeout_minutes"`
    ConcurrentDownloads    int    `json:"concurrent_downloads"`
	Launchers              []LauncherConfig `json:"launchers"`
}

func LoadConfig(projectRoot string) (*Config, error) {
	cfgPath := filepath.Join(projectRoot, "config.json")
	f, err := os.Open(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("open config.json: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read config.json: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config.json: %w", err)
	}
	if cfg.StoragePath == "" {
		return nil, errors.New("config.storage_path must not be empty")
	}
	if cfg.CheckCron == "" {
		cfg.CheckCron = "*/10 * * * *" // default every 10 minutes
	}
	// Allow env var override for GitHub token
	if env := os.Getenv("GITHUB_TOKEN"); env != "" {
		cfg.GitHubToken = env
	}
	return &cfg, nil
}
