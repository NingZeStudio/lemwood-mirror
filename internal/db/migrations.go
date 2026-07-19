package db

import (
	"database/sql"
	"fmt"
	"log"
)

// CurrentSchemaVersion 是当前代码所期望的最新 schema 版本。
// 每次新增 Migration 时递增此常量。
const CurrentSchemaVersion = 2

// Migration 描述一个版本化的数据库迁移步骤。
// Version 必须严格递增；Up 在已应用更低版本的迁移后被调用。
// Up 必须是幂等的：在已应用过同版本迁移的库上重复执行不应报错或产生重复数据。
type Migration struct {
	Version     int
	Description string
	Up          func(d *sql.DB) error
}

// migrations 是按 Version 升序排列的迁移注册表。
// 新增迁移时追加到末尾并递增 CurrentSchemaVersion。
var migrations = []Migration{
	{
		Version:     1,
		Description: "schema baseline 兜底（MySQL no-op；SQLite 补 source/ban_type 列）",
		Up:          migrateV1SchemaBaseline,
	},
	{
		Version:     2,
		Description: "历史流量数据聚合到无 IP 聚合表",
		Up:          migrateV2AggregateTraffic,
	},
}

// getSchemaVersion 从 system_info 表读取 schema_version，缺失视为 0。
func getSchemaVersion() (int, error) {
	var version int
	err := DB.QueryRow("SELECT value FROM system_info WHERE `key` = ?", "schema_version").Scan(&version)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("读取 schema_version 失败: %w", err)
	}
	return version, nil
}

// setSchemaVersion 写入或更新 schema_version。
func setSchemaVersion(version int) error {
	var query string
	if isMySQL {
		query = "INSERT INTO system_info (`key`, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)"
	} else {
		query = "INSERT OR REPLACE INTO system_info (key, value) VALUES (?, ?)"
	}
	if _, err := DB.Exec(query, "schema_version", version); err != nil {
		return fmt.Errorf("写入 schema_version=%d 失败: %w", version, err)
	}
	return nil
}

// runMigrations 顺序应用所有 Version > 当前版本的迁移。
// 每个迁移结束后立即写 schema_version 作为提交点；任一迁移失败立即返回错误。
func runMigrations() error {
	current, err := getSchemaVersion()
	if err != nil {
		return err
	}

	if current > CurrentSchemaVersion {
		log.Printf("[数据库迁移] 警告: 当前 schema_version=%d 高于代码版本 %d，跳过迁移", current, CurrentSchemaVersion)
		return nil
	}

	if current == CurrentSchemaVersion {
		log.Printf("[数据库迁移] 当前 schema_version=%d，无待执行迁移", current)
		return nil
	}

	log.Printf("[数据库迁移] 当前 schema_version=%d，目标=%d，开始迁移", current, CurrentSchemaVersion)

	for _, m := range migrations {
		if m.Version <= current {
			continue
		}
		log.Printf("[数据库迁移] 应用 v%d: %s", m.Version, m.Description)
		if err := m.Up(DB); err != nil {
			return fmt.Errorf("迁移 v%d 失败: %w", m.Version, err)
		}
		if err := setSchemaVersion(m.Version); err != nil {
			return err
		}
		log.Printf("[数据库迁移] v%d 完成", m.Version)
	}

	log.Printf("[数据库迁移] 全部迁移完成，schema_version=%d", CurrentSchemaVersion)
	return nil
}

// migrateV1SchemaBaseline 对历史库做列兜底。
// MySQL：建表已含所有列，no-op。
// SQLite：检测 ip_blacklist 是否有 source 列，缺失则 ALTER TABLE ADD COLUMN。
func migrateV1SchemaBaseline(d *sql.DB) error {
	if isMySQL {
		return nil
	}

	rows, err := d.Query("PRAGMA table_info(ip_blacklist)")
	if err != nil {
		return fmt.Errorf("查询 ip_blacklist 列信息失败: %w", err)
	}
	defer rows.Close()

	hasSourceColumn := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == "source" {
			hasSourceColumn = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历 ip_blacklist 列信息失败: %w", err)
	}

	if hasSourceColumn {
		return nil
	}

	log.Println("[数据库迁移] v1: 为 ip_blacklist 表添加 source 和 ban_type 列")
	alterQueries := []string{
		"ALTER TABLE ip_blacklist ADD COLUMN source TEXT DEFAULT 'manual'",
		"ALTER TABLE ip_blacklist ADD COLUMN ban_type TEXT DEFAULT 'manual'",
	}
	for _, q := range alterQueries {
		if _, err := d.Exec(q); err != nil {
			return fmt.Errorf("添加列失败: %w, query: %s", err, q)
		}
	}
	return nil
}

// migrateV2AggregateTraffic 将 ip_daily_traffic 历史数据
// 聚合到无 IP 聚合表 daily_traffic。
// 幂等：使用 INSERT IGNORE / INSERT OR IGNORE，重复执行不产生重复行。
// 注：repo 镜像功能已移除，本迁移不再聚合 repo_ip_daily_traffic；
// 已应用过 v2 的库（schema_version>=2）不会重复执行本迁移。
func migrateV2AggregateTraffic(d *sql.DB) error {
	var insertPrefix string
	if isMySQL {
		insertPrefix = "INSERT IGNORE INTO"
	} else {
		insertPrefix = "INSERT OR IGNORE INTO"
	}

	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(insertPrefix + ` daily_traffic (date, bytes_downloaded)
		SELECT date, SUM(bytes_downloaded) FROM ip_daily_traffic GROUP BY date`); err != nil {
		return fmt.Errorf("聚合 daily_traffic 失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交聚合事务失败: %w", err)
	}

	log.Println("[数据库迁移] v2: 历史流量聚合完成")
	return nil
}
