package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// setupSQLiteDB 在临时目录创建一个独立的 SQLite 连接，供测试使用。
// 调用方负责在 t.Cleanup 中关闭。
func setupSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_stats.db")
	d, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open error = %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// applyMaySchemaOnly 仅创建 5 月份存在的表（不含 repo_downloads /
// repo_ip_daily_traffic / daily_traffic / daily_repo_traffic / stats_snapshot），
// 并插入若干 ip_daily_traffic 历史数据，模拟生产 5 月库的状态。
func applyMaySchemaOnly(t *testing.T, d *sql.DB) {
	t.Helper()
	mayQueries := []string{
		`CREATE TABLE IF NOT EXISTS visits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip TEXT, path TEXT, user_agent TEXT, referer TEXT,
			country TEXT, region TEXT, city TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS downloads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_name TEXT, launcher TEXT, version TEXT, ip TEXT, country TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS ip_blacklist (
			ip TEXT PRIMARY KEY, reason TEXT,
			source TEXT DEFAULT 'manual', ban_type TEXT DEFAULT 'manual',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS ip_daily_traffic (
			ip TEXT, date TEXT, bytes_downloaded INTEGER DEFAULT 0,
			PRIMARY KEY (ip, date)
		)`,
		`CREATE TABLE IF NOT EXISTS system_info (
			key TEXT PRIMARY KEY, value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range mayQueries {
		if _, err := d.Exec(q); err != nil {
			t.Fatalf("applyMaySchemaOnly exec error = %v, query=%s", err, q)
		}
	}

	// 插入历史 ip_daily_traffic 数据（不同 IP、不同日期）
	rows := []struct{ ip, date string; bytes int64 }{
		{"1.1.1.1", "2026-05-01", 1024},
		{"2.2.2.2", "2026-05-01", 2048},
		{"1.1.1.1", "2026-05-02", 4096},
	}
	for _, r := range rows {
		if _, err := d.Exec(
			"INSERT INTO ip_daily_traffic (ip, date, bytes_downloaded) VALUES (?, ?, ?)",
			r.ip, r.date, r.bytes,
		); err != nil {
			t.Fatalf("insert ip_daily_traffic error = %v", err)
		}
	}
}

func TestGetSchemaVersion_DefaultsToZero(t *testing.T) {
	d := setupSQLiteDB(t)
	// 仅建 system_info 表，不写 schema_version 行
	if _, err := d.Exec(`CREATE TABLE system_info (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatalf("create system_info error = %v", err)
	}
	// 切换包级 DB 指向测试连接
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	v, err := getSchemaVersion()
	if err != nil {
		t.Fatalf("getSchemaVersion error = %v", err)
	}
	if v != 0 {
		t.Fatalf("expected 0 when no row, got %d", v)
	}
}

func TestSetSchemaVersion_Idempotent(t *testing.T) {
	d := setupSQLiteDB(t)
	if _, err := d.Exec(`CREATE TABLE system_info (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatalf("create system_info error = %v", err)
	}
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	for i := 0; i < 3; i++ {
		if err := setSchemaVersion(2); err != nil {
			t.Fatalf("setSchemaVersion attempt %d error = %v", i, err)
		}
	}
	v, err := getSchemaVersion()
	if err != nil {
		t.Fatalf("getSchemaVersion error = %v", err)
	}
	if v != 2 {
		t.Fatalf("expected 2, got %d", v)
	}
}

func TestRunMigrations_FreshInstall(t *testing.T) {
	d := setupSQLiteDB(t)
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	if err := createTables(); err != nil {
		t.Fatalf("createTables error = %v", err)
	}
	// createTables 内部已调用 runMigrations，验证 schema_version 与空聚合表
	v, err := getSchemaVersion()
	if err != nil {
		t.Fatalf("getSchemaVersion error = %v", err)
	}
	if v != CurrentSchemaVersion {
		t.Fatalf("expected schema_version=%d, got %d", CurrentSchemaVersion, v)
	}
	var count int
	if err := DB.QueryRow("SELECT COUNT(*) FROM daily_traffic").Scan(&count); err != nil {
		t.Fatalf("count daily_traffic error = %v", err)
	}
	if count != 0 {
		t.Fatalf("fresh install daily_traffic should be empty, got %d rows", count)
	}
}

func TestRunMigrations_MaySchemaAggregate(t *testing.T) {
	d := setupSQLiteDB(t)
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	// 模拟 5 月 schema + 历史数据
	applyMaySchemaOnly(t, d)

	// 跑 createTables（自动补全新表 + 调 runMigrations）
	if err := createTables(); err != nil {
		t.Fatalf("createTables error = %v", err)
	}

	// 校验 schema_version
	v, err := getSchemaVersion()
	if err != nil {
		t.Fatalf("getSchemaVersion error = %v", err)
	}
	if v != CurrentSchemaVersion {
		t.Fatalf("expected schema_version=%d, got %d", CurrentSchemaVersion, v)
	}

	// 校验 daily_traffic 聚合结果
	// 2026-05-01: 1024 + 2048 = 3072
	// 2026-05-02: 4096
	type row struct {
		date  string
		bytes int64
	}
	want := map[string]int64{
		"2026-05-01": 3072,
		"2026-05-02": 4096,
	}
	got := map[string]int64{}
	rows, err := DB.Query("SELECT date, bytes_downloaded FROM daily_traffic ORDER BY date")
	if err != nil {
		t.Fatalf("query daily_traffic error = %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.date, &r.bytes); err != nil {
			t.Fatalf("scan error = %v", err)
		}
		got[r.date] = r.bytes
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err = %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("daily_traffic rows count mismatch: got %d, want %d (got=%v)", len(got), len(want), got)
	}
	for date, wantBytes := range want {
		if gotBytes, ok := got[date]; !ok {
			t.Fatalf("daily_traffic missing date %s", date)
		} else if gotBytes != wantBytes {
			t.Fatalf("daily_traffic[%s] = %d, want %d", date, gotBytes, wantBytes)
		}
	}

	// 校验 daily_repo_traffic 为空（5月库无 repo_ip_daily_traffic 数据，新建空表聚合 0 行）
	var repoCount int
	if err := DB.QueryRow("SELECT COUNT(*) FROM daily_repo_traffic").Scan(&repoCount); err != nil {
		t.Fatalf("count daily_repo_traffic error = %v", err)
	}
	if repoCount != 0 {
		t.Fatalf("daily_repo_traffic should be empty, got %d", repoCount)
	}

	// 校验 stats_snapshot 表存在且为空
	var snapCount int
	if err := DB.QueryRow("SELECT COUNT(*) FROM stats_snapshot").Scan(&snapCount); err != nil {
		t.Fatalf("count stats_snapshot error = %v", err)
	}
	if snapCount != 0 {
		t.Fatalf("stats_snapshot should be empty, got %d", snapCount)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	d := setupSQLiteDB(t)
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	applyMaySchemaOnly(t, d)
	if err := createTables(); err != nil {
		t.Fatalf("createTables error = %v", err)
	}

	// 记录第一次聚合后的行数
	var beforeCount int
	if err := DB.QueryRow("SELECT COUNT(*) FROM daily_traffic").Scan(&beforeCount); err != nil {
		t.Fatalf("count before error = %v", err)
	}

	// 再次调用 runMigrations，应跳过所有迁移
	if err := runMigrations(); err != nil {
		t.Fatalf("second runMigrations error = %v", err)
	}

	var afterCount int
	if err := DB.QueryRow("SELECT COUNT(*) FROM daily_traffic").Scan(&afterCount); err != nil {
		t.Fatalf("count after error = %v", err)
	}
	if afterCount != beforeCount {
		t.Fatalf("idempotent check failed: before=%d, after=%d", beforeCount, afterCount)
	}
}

func TestRunMigrations_SkipsAlreadyApplied(t *testing.T) {
	d := setupSQLiteDB(t)
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	// 建表 + 预设 schema_version = CurrentSchemaVersion
	if err := createTables(); err != nil {
		t.Fatalf("createTables error = %v", err)
	}
	if err := setSchemaVersion(CurrentSchemaVersion); err != nil {
		t.Fatalf("setSchemaVersion error = %v", err)
	}

	// 插入一条 daily_traffic 数据，验证不会被改动
	if _, err := DB.Exec("INSERT INTO daily_traffic (date, bytes_downloaded) VALUES (?, ?)", "2099-01-01", int64(999)); err != nil {
		t.Fatalf("insert sentinel error = %v", err)
	}

	if err := runMigrations(); err != nil {
		t.Fatalf("runMigrations error = %v", err)
	}

	// 应只保留 sentinel 一行（迁移被跳过，不会聚合空 ip_daily_traffic 写入新行）
	var count int
	if err := DB.QueryRow("SELECT COUNT(*) FROM daily_traffic").Scan(&count); err != nil {
		t.Fatalf("count error = %v", err)
	}
	if count != 1 {
		t.Fatalf("expected daily_traffic unchanged with 1 row, got %d", count)
	}
}
