package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LauncherConfig 描述如何从源页面发现启动器的 GitHub 仓库 URL。
// 如果 RepoSelector 以 "regex:" 开头，它将被视为正则表达式来匹配锚点 href。
// 如果 RepoSelector 为空，则使用第一个包含 "github.com" 的锚点 href。
// SourceURL 可以直接是 GitHub 仓库 URL（例如 https://github.com/owner/repo），在这种情况下选择器被忽略。

type LauncherConfig struct {
	Name         string `json:"name"`
	SourceURL    string `json:"source_url"`
	RepoSelector string `json:"repo_selector"`
}

type Config struct {
	ServerAddress          string           `json:"server_address"`
	ServerPort             int              `json:"server_port"`
	CheckCron              string           `json:"check_cron"`
	StoragePath            string           `json:"storage_path"`
	GitHubToken            string           `json:"github_token"`
	AdminUser              string           `json:"admin_user"`
	AdminPassword          string           `json:"admin_password"`
	ProxyURL               string           `json:"proxy_url"`
	AssetProxyURL          string           `json:"asset_proxy_url"`
	XgetDomain             string           `json:"xget_domain"`
	XgetEnabled            bool             `json:"xget_enabled"`
	DownloadTimeoutMinutes int              `json:"download_timeout_minutes"`
	ConcurrentDownloads    int              `json:"concurrent_downloads"`
	DownloadUrlBase        string           `json:"download_url_base,omitempty"`
	Launchers              []LauncherConfig `json:"launchers"`
}

func LoadConfig(projectRoot string) (*Config, error) {
	cfgPath := filepath.Join(projectRoot, "config.json")
	f, err := os.Open(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("打开 config.json 失败: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("读取 config.json 失败: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("解析 config.json 失败: %w", err)
	}
	if cfg.StoragePath == "" {
		return nil, errors.New("config.storage_path 不能为空")
	}
	if cfg.CheckCron == "" {
		cfg.CheckCron = "*/10 * * * *" // 默认每 10 分钟
	}
	// 允许环境变量覆盖 GitHub 令牌
	if env := os.Getenv("GITHUB_TOKEN"); env != "" {
		cfg.GitHubToken = env
	}
	return &cfg, nil
}

func (c *Config) Save(projectRoot string) error {
	cfgPath := filepath.Join(projectRoot, "config.json")
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 config.json 失败: %w", err)
	}
	if err := os.WriteFile(cfgPath, b, 0644); err != nil {
		return fmt.Errorf("写入 config.json 失败: %w", err)
	}
	return nil
}
