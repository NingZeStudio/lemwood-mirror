package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

const defaultConfigTemplate = `# 柠枺镜像配置文件
# YAML 支持注释，后台保存时会保留本模板结构并回填最新配置值。

server_address: {{ yaml .ServerAddress }}
server_port: {{ .ServerPort }}

# 启用的 API 版本：v1（仅旧版）、v2（仅新版）、both（两者并行，默认）
api_version: {{ yaml .APIVersion }}

# 定时扫描 cron 表达式（分钟粒度）
check_cron: {{ yaml .CheckCron }}

# Release 资源存储目录（相对项目根目录）
storage_path: {{ yaml .StoragePath }}

# GitHub Token，可留空并使用环境变量 GITHUB_TOKEN 覆盖
github_token: {{ yaml .GitHubToken }}

# 对外下载地址基准（为空时回退到 server_address）
download_url_base: {{ yaml .DownloadUrlBase }}

# 单文件下载超时（分钟），Git 镜像同步也复用此超时
download_timeout_minutes: {{ .DownloadTimeoutMinutes }}
concurrent_downloads: {{ .ConcurrentDownloads }}

proxy_url: {{ yaml .ProxyURL }}
asset_proxy_url: {{ yaml .AssetProxyURL }}
xget_domain: {{ yaml .XgetDomain }}
xget_enabled: {{ .XgetEnabled }}

admin_enabled: {{ .AdminEnabled }}
admin_user: {{ yaml .AdminUser }}
admin_password: {{ yaml .AdminPassword }}
admin_max_retries: {{ .AdminMaxRetries }}
admin_lock_duration: {{ .AdminLockDuration }}

two_factor_enabled: {{ .TwoFactorEnabled }}
two_factor_secret: {{ yaml .TwoFactorSecret }}

captcha_enabled: {{ .CaptchaEnabled }}
captcha_app_id: {{ yaml .CaptchaAppId }}
captcha_secret_key: {{ yaml .CaptchaSecretKey }}

traffic_limit_gb: {{ .TrafficLimitGB }}
ban_record_file: {{ yaml .BanRecordFile }}
external_blacklist_url: {{ yaml .ExternalBlacklistURL }}
appeal_contact: {{ yaml .AppealContact }}

mysql_host: {{ yaml .MySQLHost }}
mysql_port: {{ .MySQLPort }}
mysql_user: {{ yaml .MySQLUser }}
mysql_password: {{ yaml .MySQLPassword }}
mysql_database: {{ yaml .MySQLDatabase }}
mysql_migration: {{ .MySQLMigration }}

self_update_enabled: {{ .SelfUpdateEnabled }}
self_update_repo_url: {{ yaml .SelfUpdateRepoURL }}
self_update_channel: {{ yaml .SelfUpdateChannel }}
self_update_check_cron: {{ yaml .SelfUpdateCheckCron }}
self_update_auto_restart: {{ .SelfUpdateAutoRestart }}

# 启动器列表
# mode:
#   - release: 仅同步 Release 资源
#   - clone: 仅同步 Git 镜像到 repo/{name}.git
#   - all: 同时同步 Release 和 Git 镜像
launchers:
{{- range .Launchers }}
  - name: {{ yaml .Name }}
    source_url: {{ yaml .SourceURL }}
    repo_selector: {{ yaml .RepoSelector }}
    mode: {{ yaml .Mode }}
    include_prerelease: {{ .IncludePrerelease }}
    max_versions: {{ .MaxVersions }}
{{- end }}
`

type LauncherMode string

type SelfUpdateChannel string

const (
	LauncherModeRelease LauncherMode = "release"
	LauncherModeClone   LauncherMode = "clone"
	LauncherModeAll     LauncherMode = "all"

	SelfUpdateChannelNotify  SelfUpdateChannel = "notify"
	SelfUpdateChannelRelease SelfUpdateChannel = "release"
	SelfUpdateChannelPreview SelfUpdateChannel = "preview"
)

type LauncherConfig struct {
	Name              string `json:"name" yaml:"name"`
	SourceURL         string `json:"source_url" yaml:"source_url"`
	RepoSelector      string `json:"repo_selector" yaml:"repo_selector"`
	Mode              string `json:"mode" yaml:"mode"`
	IncludePrerelease bool   `json:"include_prerelease" yaml:"include_prerelease"`
	MaxVersions       int    `json:"max_versions" yaml:"max_versions"`
}

func NormalizeLauncherMode(mode string) (LauncherMode, error) {
	switch LauncherMode(mode) {
	case "", LauncherModeRelease:
		return LauncherModeRelease, nil
	case LauncherModeClone:
		return LauncherModeClone, nil
	case LauncherModeAll:
		return LauncherModeAll, nil
	default:
		return "", fmt.Errorf("无效的 launcher.mode %q，需要 release、clone 或 all", mode)
	}
}

func NormalizeSelfUpdateChannel(channel string) (SelfUpdateChannel, error) {
	switch SelfUpdateChannel(channel) {
	case "", SelfUpdateChannelNotify:
		return SelfUpdateChannelNotify, nil
	case SelfUpdateChannelRelease:
		return SelfUpdateChannelRelease, nil
	case SelfUpdateChannelPreview:
		return SelfUpdateChannelPreview, nil
	default:
		return "", fmt.Errorf("无效的 self_update_channel %q，需要 notify、release 或 preview", channel)
	}
}

func ShouldSyncRelease(mode string) bool {
	normalized, err := NormalizeLauncherMode(mode)
	if err != nil {
		return false
	}
	return normalized == LauncherModeRelease || normalized == LauncherModeAll
}

func ShouldSyncClone(mode string) bool {
	normalized, err := NormalizeLauncherMode(mode)
	if err != nil {
		return false
	}
	return normalized == LauncherModeClone || normalized == LauncherModeAll
}

type Config struct {
	ServerAddress          string           `json:"server_address" yaml:"server_address"`
	ServerPort             int              `json:"server_port" yaml:"server_port"`
	APIVersion             string           `json:"api_version" yaml:"api_version"`
	CheckCron              string           `json:"check_cron" yaml:"check_cron"`
	StoragePath            string           `json:"storage_path" yaml:"storage_path"`
	GitHubToken            string           `json:"github_token" yaml:"github_token"`
	AdminUser              string           `json:"admin_user" yaml:"admin_user"`
	AdminPassword          string           `json:"admin_password" yaml:"admin_password"`
	AdminEnabled           bool             `json:"admin_enabled" yaml:"admin_enabled"`
	AdminMaxRetries        int              `json:"admin_max_retries" yaml:"admin_max_retries"`
	AdminLockDuration      int              `json:"admin_lock_duration" yaml:"admin_lock_duration"`
	ProxyURL               string           `json:"proxy_url" yaml:"proxy_url"`
	AssetProxyURL          string           `json:"asset_proxy_url" yaml:"asset_proxy_url"`
	XgetDomain             string           `json:"xget_domain" yaml:"xget_domain"`
	XgetEnabled            bool             `json:"xget_enabled" yaml:"xget_enabled"`
	DownloadTimeoutMinutes int              `json:"download_timeout_minutes" yaml:"download_timeout_minutes"`
	ConcurrentDownloads    int              `json:"concurrent_downloads" yaml:"concurrent_downloads"`
	DownloadUrlBase        string           `json:"download_url_base,omitempty" yaml:"download_url_base,omitempty"`
	TwoFactorEnabled       bool             `json:"two_factor_enabled" yaml:"two_factor_enabled"`
	TwoFactorSecret        string           `json:"two_factor_secret" yaml:"two_factor_secret"`
	CaptchaEnabled         bool             `json:"captcha_enabled" yaml:"captcha_enabled"`
	CaptchaAppId           string           `json:"captcha_app_id" yaml:"captcha_app_id"`
	CaptchaSecretKey       string           `json:"captcha_secret_key" yaml:"captcha_secret_key"`
	Launchers              []LauncherConfig `json:"launchers" yaml:"launchers"`
	TrafficLimitGB         int              `json:"traffic_limit_gb" yaml:"traffic_limit_gb"`
	BanRecordFile          string           `json:"ban_record_file" yaml:"ban_record_file"`
	ExternalBlacklistURL   string           `json:"external_blacklist_url" yaml:"external_blacklist_url"`
	AppealContact          string           `json:"appeal_contact" yaml:"appeal_contact"`
	MySQLHost              string           `json:"mysql_host" yaml:"mysql_host"`
	MySQLPort              int              `json:"mysql_port" yaml:"mysql_port"`
	MySQLUser              string           `json:"mysql_user" yaml:"mysql_user"`
	MySQLPassword          string           `json:"mysql_password" yaml:"mysql_password"`
	MySQLDatabase          string           `json:"mysql_database" yaml:"mysql_database"`
	MySQLMigration         bool             `json:"mysql_migration" yaml:"mysql_migration"`
	SelfUpdateEnabled      bool             `json:"self_update_enabled" yaml:"self_update_enabled"`
	SelfUpdateRepoURL      string           `json:"self_update_repo_url" yaml:"self_update_repo_url"`
	SelfUpdateChannel      string           `json:"self_update_channel" yaml:"self_update_channel"`
	SelfUpdateCheckCron    string           `json:"self_update_check_cron" yaml:"self_update_check_cron"`
	SelfUpdateAutoRestart  bool             `json:"self_update_auto_restart" yaml:"self_update_auto_restart"`
}

func DefaultConfig() *Config {
	return &Config{
		ServerPort:             8080,
		APIVersion:             "both",
		CheckCron:              "*/10 * * * *",
		StoragePath:            "download",
		DownloadTimeoutMinutes: 40,
		ConcurrentDownloads:    3,
		XgetDomain:             "https://xget.xi-xu.me",
		XgetEnabled:            true,
		AdminEnabled:           true,
		AdminMaxRetries:        10,
		AdminLockDuration:      120,
		TrafficLimitGB:         0,
		BanRecordFile:          "banned_ips.txt",
		AppealContact:          "QQ群 https://qm.qq.com/q/FOGt99aayY",
		MySQLPort:              3306,
		SelfUpdateChannel:      string(SelfUpdateChannelNotify),
		Launchers:              []LauncherConfig{},
	}
}

func NormalizeMaxVersions(v int) int {
	if v <= 0 {
		return 3
	}
	return v
}

func configYAMLPath(projectRoot string) string {
	return filepath.Join(projectRoot, "config.yaml")
}

func legacyConfigJSONPath(projectRoot string) string {
	return filepath.Join(projectRoot, "config.json")
}

func LoadConfig(projectRoot string) (*Config, error) {
	cfgPath := configYAMLPath(projectRoot)
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		legacyPath := legacyConfigJSONPath(projectRoot)
		if _, legacyErr := os.Stat(legacyPath); legacyErr == nil {
			cfg, err := loadLegacyJSON(legacyPath)
			if err != nil {
				return nil, err
			}
			if err := NormalizeConfig(cfg); err != nil {
				return nil, err
			}
			if err := cfg.Save(projectRoot); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		// 释放嵌入的默认配置文件（default.yaml）
		if err := os.WriteFile(cfgPath, defaultConfigYAML, 0o644); err != nil {
			return nil, fmt.Errorf("写入默认 config.yaml 失败: %w", err)
		}
		cfg := DefaultConfig()
		if err := yaml.Unmarshal(defaultConfigYAML, cfg); err != nil {
			return nil, fmt.Errorf("解析默认配置失败: %w", err)
		}
		if err := NormalizeConfig(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	} else if err != nil {
		return nil, fmt.Errorf("检查 config.yaml 失败: %w", err)
	}

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("读取 config.yaml 失败: %w", err)
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("解析 config.yaml 失败: %w", err)
	}
	if err := NormalizeConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func NormalizeConfig(cfg *Config) error {
	if cfg.StoragePath == "" {
		return errors.New("config.storage_path 不能为空")
	}
	for i := range cfg.Launchers {
		normalizedMode, err := NormalizeLauncherMode(cfg.Launchers[i].Mode)
		if err != nil {
			return fmt.Errorf("launcher %q 配置无效: %w", cfg.Launchers[i].Name, err)
		}
		cfg.Launchers[i].Mode = string(normalizedMode)
	}
	if cfg.CheckCron == "" {
		cfg.CheckCron = "*/10 * * * *"
	}
	channel, err := NormalizeSelfUpdateChannel(cfg.SelfUpdateChannel)
	if err != nil {
		return err
	}
	cfg.SelfUpdateChannel = string(channel)
	if cfg.AdminEnabled {
		if cfg.AdminUser == "" || cfg.AdminPassword == "" {
			fmt.Println("警告: 管理员账号或密码未配置，管理后台已自动禁用")
			cfg.AdminEnabled = false
		}
		if cfg.AdminMaxRetries <= 0 {
			cfg.AdminMaxRetries = 10
		}
		if cfg.AdminLockDuration <= 0 {
			cfg.AdminLockDuration = 120
		}
	} else {
		fmt.Println("提示: 管理后台当前处于禁用状态")
	}
	if env := os.Getenv("GITHUB_TOKEN"); env != "" {
		cfg.GitHubToken = env
	}
	if cfg.TrafficLimitGB < 0 {
		cfg.TrafficLimitGB = 5
	}
	if cfg.BanRecordFile == "" {
		cfg.BanRecordFile = "banned_ips.txt"
	}
	if cfg.AppealContact == "" {
		cfg.AppealContact = "QQ群 https://qm.qq.com/q/FOGt99aayY"
	}
	switch strings.ToLower(cfg.APIVersion) {
	case "", "both":
		cfg.APIVersion = "both"
	case "v1", "v2":
		// 合法值，保留
	default:
		return fmt.Errorf("无效的 api_version %q，需要 v1、v2 或 both", cfg.APIVersion)
	}
	return nil
}

func (c *Config) Save(projectRoot string) error {
	cfgPath := configYAMLPath(projectRoot)
	tpl, err := template.New("config").Funcs(template.FuncMap{
		"yaml": yamlScalar,
	}).Parse(defaultConfigTemplate)
	if err != nil {
		return fmt.Errorf("解析配置模板失败: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, c); err != nil {
		return fmt.Errorf("渲染配置模板失败: %w", err)
	}
	if err := os.WriteFile(cfgPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("写入 config.yaml 失败: %w", err)
	}
	return nil
}

func loadLegacyJSON(cfgPath string) (*Config, error) {
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("读取旧 config.json 失败: %w", err)
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("解析旧 config.json 失败: %w", err)
	}
	return cfg, nil
}

func yamlScalar(v string) string {
	if v == "" {
		return `""`
	}
	b, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%q", v)
	}
	return strings.TrimSpace(string(b))
}
