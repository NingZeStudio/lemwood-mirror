package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultConfigTemplate = `# 柠泽资源站配置文件
# YAML 支持注释，后台保存时会保留本模板结构并回填最新配置值。

server_address: {{ yaml .ServerAddress }}
server_port: {{ .ServerPort }}

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

# PoW 下载验证（替代极验，自动验证客户端正规性）
pow_enabled: {{ .PowEnabled }}
pow_algorithm: {{ yaml .PowAlgorithm }}
pow_cost: {{ .PowCost }}
pow_key_length: {{ .PowKeyLength }}
pow_difficulty: {{ .PowDifficulty }}
pow_challenge_ttl: {{ yaml .PowChallengeTTL }}
# PoW HMAC 签名密钥；留空时启动随机生成（挑战内存态+短 TTL，重启即失效，无需持久化）
pow_hmac_secret: {{ yaml .PowHMACSecret }}

# 下载授权令牌有效期（解析为 Go duration，如 5m / 30s）
download_token_ttl: {{ yaml .DownloadTokenTTL }}

traffic_limit_gb: {{ .TrafficLimitGB }}
bandwidth_limit_mbps: {{ .BandwidthLimitMbps }}
ban_record_file: {{ yaml .BanRecordFile }}
external_blacklist_url: {{ yaml .ExternalBlacklistURL }}
appeal_contact: {{ yaml .AppealContact }}

mysql_host: {{ yaml .MySQLHost }}
mysql_port: {{ .MySQLPort }}
mysql_user: {{ yaml .MySQLUser }}
mysql_password: {{ yaml .MySQLPassword }}
mysql_database: {{ yaml .MySQLDatabase }}
mysql_migration: {{ .MySQLMigration }}

database_mode: {{ yaml .DatabaseMode }}

postgres_host: {{ yaml .PostgresHost }}
postgres_port: {{ .PostgresPort }}
postgres_user: {{ yaml .PostgresUser }}
postgres_password: {{ yaml .PostgresPassword }}
postgres_database: {{ yaml .PostgresDatabase }}
postgres_sslmode: {{ yaml .PostgresSSLMode }}
postgres_migration_batch: {{ .PostgresMigrationBatch }}
postgres_migration_delay: {{ yaml .PostgresMigrationDelay }}

self_update_enabled: {{ .SelfUpdateEnabled }}
self_update_repo_url: {{ yaml .SelfUpdateRepoURL }}
self_update_channel: {{ yaml .SelfUpdateChannel }}
self_update_check_cron: {{ yaml .SelfUpdateCheckCron }}
self_update_auto_restart: {{ .SelfUpdateAutoRestart }}

# 启动器列表
# mode:
#   - release: 仅同步 Release 资源
#   - clone / all: 已废弃（Git 镜像功能已移除），仅为兼容旧配置保留
launchers:
{{- range .Launchers }}
  - name: {{ yaml .Name }}
    source_url: {{ yaml .SourceURL }}
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

type Config struct {
	ServerAddress          string           `json:"server_address" yaml:"server_address"`
	ServerPort             int              `json:"server_port" yaml:"server_port"`
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
	PowEnabled             bool             `json:"pow_enabled" yaml:"pow_enabled"`
	PowAlgorithm           string           `json:"pow_algorithm" yaml:"pow_algorithm"`
	PowCost                int              `json:"pow_cost" yaml:"pow_cost"`
	PowKeyLength           int              `json:"pow_key_length" yaml:"pow_key_length"`
	PowDifficulty          int              `json:"pow_difficulty" yaml:"pow_difficulty"`
	PowChallengeTTL        string           `json:"pow_challenge_ttl" yaml:"pow_challenge_ttl"`
	PowHMACSecret          string           `json:"pow_hmac_secret,omitempty" yaml:"pow_hmac_secret,omitempty"`
	DownloadTokenTTL       string           `json:"download_token_ttl,omitempty" yaml:"download_token_ttl,omitempty"`
	Launchers              []LauncherConfig `json:"launchers" yaml:"launchers"`
	TrafficLimitGB         int              `json:"traffic_limit_gb" yaml:"traffic_limit_gb"`
	BandwidthLimitMbps     int              `json:"bandwidth_limit_mbps" yaml:"bandwidth_limit_mbps"`
	BanRecordFile          string           `json:"ban_record_file" yaml:"ban_record_file"`
	ExternalBlacklistURL   string           `json:"external_blacklist_url" yaml:"external_blacklist_url"`
	AppealContact          string           `json:"appeal_contact" yaml:"appeal_contact"`
	MySQLHost              string           `json:"mysql_host" yaml:"mysql_host"`
	MySQLPort              int              `json:"mysql_port" yaml:"mysql_port"`
	MySQLUser              string           `json:"mysql_user" yaml:"mysql_user"`
	MySQLPassword          string           `json:"mysql_password" yaml:"mysql_password"`
	MySQLDatabase          string           `json:"mysql_database" yaml:"mysql_database"`
	MySQLMigration         bool             `json:"mysql_migration" yaml:"mysql_migration"`
	DatabaseMode           string           `json:"database_mode" yaml:"database_mode"`
	PostgresHost           string           `json:"postgres_host" yaml:"postgres_host"`
	PostgresPort           int              `json:"postgres_port" yaml:"postgres_port"`
	PostgresUser           string           `json:"postgres_user" yaml:"postgres_user"`
	PostgresPassword       string           `json:"postgres_password" yaml:"postgres_password"`
	PostgresDatabase       string           `json:"postgres_database" yaml:"postgres_database"`
	PostgresSSLMode        string           `json:"postgres_sslmode" yaml:"postgres_sslmode"`
	PostgresMigrationBatch int              `json:"postgres_migration_batch" yaml:"postgres_migration_batch"`
	PostgresMigrationDelay string           `json:"postgres_migration_delay" yaml:"postgres_migration_delay"`
	SelfUpdateEnabled      bool             `json:"self_update_enabled" yaml:"self_update_enabled"`
	SelfUpdateRepoURL      string           `json:"self_update_repo_url" yaml:"self_update_repo_url"`
	SelfUpdateChannel      string           `json:"self_update_channel" yaml:"self_update_channel"`
	SelfUpdateCheckCron    string           `json:"self_update_check_cron" yaml:"self_update_check_cron"`
	SelfUpdateAutoRestart  bool             `json:"self_update_auto_restart" yaml:"self_update_auto_restart"`
}

func DefaultConfig() *Config {
	return &Config{
		ServerPort:             8080,
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
		BandwidthLimitMbps:     200,
		BanRecordFile:          "banned_ips.txt",
		AppealContact:          "QQ群 1104690837",
		MySQLPort:              3306,
		PostgresPort:           5432,
		PostgresSSLMode:        "disable",
		PostgresMigrationBatch: 200,
		PostgresMigrationDelay: "250ms",
		DatabaseMode:           "auto",
		SelfUpdateEnabled:      true,
		SelfUpdateChannel:      string(SelfUpdateChannelRelease),
		SelfUpdateCheckCron:    "0 */6 * * *",
		SelfUpdateAutoRestart:  true,
		Launchers:              []LauncherConfig{},
		PowEnabled:             true,
		PowAlgorithm:           "PBKDF2-SHA256",
		PowCost:                500,
		PowKeyLength:           32,
		PowDifficulty:          6,
		PowChallengeTTL:        "10m",
		DownloadTokenTTL:       "10m",
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

			// 校验 config.yaml 已成功写入且可读，再删除旧 config.json；
			// 任何一步失败仅记录日志，不返回错误（迁移本身已成功）。
			yamlPath := configYAMLPath(projectRoot)
			if _, verifyErr := os.Stat(yamlPath); verifyErr == nil {
				if _, verifyErr := os.ReadFile(yamlPath); verifyErr == nil {
					if removeErr := os.Remove(legacyPath); removeErr != nil {
						log.Printf("[配置迁移] 删除旧 config.json 失败（可手动删除）: %v", removeErr)
					} else {
						log.Printf("[配置迁移] 已从 config.json 迁移至 config.yaml，并删除旧文件")
					}
				}
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

	// 配置项缺失自动补充：渲染模板对比磁盘文件，不一致则重写（新版本新增字段自动补齐）。
	if newBytes, rErr := cfg.renderYAML(); rErr == nil && !bytes.Equal(newBytes, b) {
		if wErr := os.WriteFile(cfgPath, newBytes, 0o644); wErr != nil {
			log.Printf("[配置] 自动补充缺失配置项失败: %v", wErr)
		} else {
			log.Printf("[配置] config.yaml 已自动补充缺失的配置项")
		}
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
	if cfg.BandwidthLimitMbps <= 0 {
		cfg.BandwidthLimitMbps = 200
	}
	if cfg.BanRecordFile == "" {
		cfg.BanRecordFile = "banned_ips.txt"
	}
	if cfg.AppealContact == "" {
		cfg.AppealContact = "QQ群 1104690837"
	}
	if cfg.PowAlgorithm == "" {
		cfg.PowAlgorithm = "PBKDF2-SHA256"
	}
	if cfg.PowCost <= 0 {
		cfg.PowCost = 500
	}
	if cfg.PowKeyLength <= 0 {
		cfg.PowKeyLength = 32
	}
	if cfg.PowDifficulty <= 0 {
		cfg.PowDifficulty = 6
	}
	if cfg.PowChallengeTTL == "" {
		cfg.PowChallengeTTL = "10m"
	}
	if cfg.DownloadTokenTTL == "" {
		cfg.DownloadTokenTTL = "10m"
	}
	if cfg.PostgresPort <= 0 {
		cfg.PostgresPort = 5432
	}
	if cfg.PostgresSSLMode == "" {
		cfg.PostgresSSLMode = "disable"
	}
	if cfg.PostgresMigrationBatch <= 0 {
		cfg.PostgresMigrationBatch = 200
	}
	if cfg.PostgresMigrationDelay == "" {
		cfg.PostgresMigrationDelay = "250ms"
	}
	if delay, err := time.ParseDuration(cfg.PostgresMigrationDelay); err != nil || delay < 0 {
		return fmt.Errorf("无效的 postgres_migration_delay %q", cfg.PostgresMigrationDelay)
	}
	switch strings.ToLower(strings.TrimSpace(cfg.DatabaseMode)) {
	case "", "auto":
		cfg.DatabaseMode = "auto"
	case "sqlite", "mysql", "pgsql":
		cfg.DatabaseMode = strings.ToLower(strings.TrimSpace(cfg.DatabaseMode))
	case "postgres", "postgresql":
		cfg.DatabaseMode = "pgsql"
	default:
		return fmt.Errorf("无效的 database_mode %q，需要 auto、sqlite、mysql 或 pgsql", cfg.DatabaseMode)
	}
	return nil
}

func (c *Config) renderYAML() ([]byte, error) {
	tpl, err := template.New("config").Funcs(template.FuncMap{
		"yaml": yamlScalar,
	}).Parse(defaultConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("解析配置模板失败: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, c); err != nil {
		return nil, fmt.Errorf("渲染配置模板失败: %w", err)
	}
	return buf.Bytes(), nil
}

func (c *Config) Save(projectRoot string) error {
	cfgPath := configYAMLPath(projectRoot)
	data, err := c.renderYAML()
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
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
