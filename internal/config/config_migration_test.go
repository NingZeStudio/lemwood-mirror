package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeLegacyJSON 在临时目录写一份 5月风格的 config.json。
// 5月配置特征：
//   - 含 api_version 字段（当前已移除，应被静默忽略）
//   - launcher 无 mode 字段（应被 NormalizeLauncherMode 归一化为 "release"）
//   - 无 self_update_* 字段（应使用 DefaultConfig 的默认值）
func writeLegacyJSON(t *testing.T, dir string, content any) {
	t.Helper()
	b, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("marshal legacy json error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), b, 0o644); err != nil {
		t.Fatalf("write config.json error = %v", err)
	}
}

func TestLoadConfig_LegacyJSONMigration(t *testing.T) {
	dir := t.TempDir()
	// 5月风格 config.json：含已废弃 api_version、launcher 无 mode、无 self_update_*
	writeLegacyJSON(t, dir, map[string]any{
		"server_address": "0.0.0.0",
		"server_port":    9090,
		"storage_path":   "download",
		"api_version":    "both", // 已废弃字段
		"launchers": []map[string]any{
			{
				"name":               "mirror",
				"source_url":         "https://github.com/owner/repo",
				"include_prerelease": true,
				"max_versions":       3,
				// 故意不写 mode，验证归一化为 release
			},
		},
	})

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig error = %v", err)
	}

	// 校验返回的 cfg：launcher mode 归一化为 release；self_update_channel 使用默认值
	if len(cfg.Launchers) != 1 {
		t.Fatalf("expected 1 launcher, got %d", len(cfg.Launchers))
	}
	if cfg.Launchers[0].Mode != "release" {
		t.Fatalf("expected launcher mode 'release' (normalized from empty), got %q", cfg.Launchers[0].Mode)
	}
	if cfg.SelfUpdateChannel != string(SelfUpdateChannelNotify) {
		t.Fatalf("expected default SelfUpdateChannel 'notify', got %q", cfg.SelfUpdateChannel)
	}
	if cfg.ServerPort != 9090 {
		t.Fatalf("expected ServerPort 9090 from legacy JSON, got %d", cfg.ServerPort)
	}

	// 校验 config.yaml 已生成
	yamlPath := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		t.Fatalf("expected config.yaml to exist after migration: %v", err)
	}

	// 校验 config.json 已被删除
	if _, err := os.Stat(filepath.Join(dir, "config.json")); !os.IsNotExist(err) {
		t.Fatalf("expected config.json to be deleted after migration, got err=%v", err)
	}

	// 重新调用 LoadConfig 验证 YAML 自洽
	cfg2, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("second LoadConfig error = %v", err)
	}
	if cfg2.ServerPort != 9090 {
		t.Fatalf("second load: expected ServerPort 9090, got %d", cfg2.ServerPort)
	}
	if cfg2.Launchers[0].Mode != "release" {
		t.Fatalf("second load: expected launcher mode 'release', got %q", cfg2.Launchers[0].Mode)
	}
}

func TestLoadConfig_LegacyJSONPreservedOnNormalizeFailure(t *testing.T) {
	dir := t.TempDir()
	// 写一份合法 JSON 但 launcher.mode 非法，使 NormalizeConfig 失败
	writeLegacyJSON(t, dir, map[string]any{
		"server_address": "0.0.0.0",
		"server_port":    8080,
		"storage_path":   "download",
		"launchers": []map[string]any{
			{
				"name":         "bad",
				"source_url":   "https://github.com/owner/repo",
				"mode":         "invalid_mode", // 非法值
				"max_versions": 3,
			},
		},
	})

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected LoadConfig to fail due to invalid launcher mode")
	}

	// 校验 config.json 仍然存在（迁移失败不应删除原始备份）
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatalf("expected config.json preserved on migration failure, got err=%v", err)
	}

	// 校验 config.yaml 未生成（Save 在 NormalizeConfig 之后，不应被调用）
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected config.yaml NOT to exist on migration failure, got err=%v", err)
	}
}

func TestLoadConfig_FreshInstallWritesDefault(t *testing.T) {
	dir := t.TempDir()
	// 空目录，无 config.yaml 也无 config.json

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig error = %v", err)
	}

	yamlPath := filepath.Join(dir, "config.yaml")
	b, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read config.yaml error = %v", err)
	}
	// 与嵌入的 default.yaml 内容一致
	if string(b) != string(defaultConfigYAML) {
		t.Fatalf("fresh install config.yaml content does not match default.yaml")
	}

	// DefaultConfig 的关键字段
	if cfg.ServerPort != 8080 {
		t.Fatalf("expected default ServerPort 8080, got %d", cfg.ServerPort)
	}
	if cfg.StoragePath != "download" {
		t.Fatalf("expected default StoragePath 'download', got %q", cfg.StoragePath)
	}
	if cfg.SelfUpdateChannel != string(SelfUpdateChannelNotify) {
		t.Fatalf("expected default SelfUpdateChannel 'notify', got %q", cfg.SelfUpdateChannel)
	}
}

func TestLoadConfig_YAMLTakesPrecedence(t *testing.T) {
	dir := t.TempDir()

	// 写 config.yaml，端口 7777
	yamlContent := []byte("server_address: \"0.0.0.0\"\nserver_port: 7777\nstorage_path: \"download\"\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), yamlContent, 0o644); err != nil {
		t.Fatalf("write config.yaml error = %v", err)
	}

	// 写 config.json，端口 9999（应被忽略）
	writeLegacyJSON(t, dir, map[string]any{
		"server_address": "0.0.0.0",
		"server_port":    9999,
		"storage_path":   "download",
	})

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig error = %v", err)
	}

	// 应读取 YAML（端口 7777），不是 JSON（端口 9999）
	if cfg.ServerPort != 7777 {
		t.Fatalf("expected YAML to take precedence (port 7777), got %d", cfg.ServerPort)
	}

	// config.json 不应被删除（非迁移路径，不动用户文件）
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatalf("expected config.json preserved when YAML already exists, got err=%v", err)
	}
}
