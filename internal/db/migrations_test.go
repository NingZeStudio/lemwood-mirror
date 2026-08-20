package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

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
	rows := []struct {
		ip, date string
		bytes    int64
	}{
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

// applyV2SchemaWithTraffic 模拟 v2 完成状态的库：含带历史数据的 daily_traffic
// 和 schema_version=2 的 system_info，但无 daily_completed_traffic 表。
func applyV2SchemaWithTraffic(t *testing.T, d *sql.DB) {
	t.Helper()
	queries := []string{
		`CREATE TABLE IF NOT EXISTS daily_traffic (
			date TEXT PRIMARY KEY, bytes_downloaded INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS system_info (
			key TEXT PRIMARY KEY, value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			t.Fatalf("applyV2SchemaWithTraffic exec error = %v, query=%s", err, q)
		}
	}

	rows := []struct {
		date  string
		bytes int64
	}{
		{"2026-05-01", 3072},
		{"2026-05-02", 4096},
	}
	for _, r := range rows {
		if _, err := d.Exec(
			"INSERT INTO daily_traffic (date, bytes_downloaded) VALUES (?, ?)",
			r.date, r.bytes,
		); err != nil {
			t.Fatalf("insert daily_traffic error = %v", err)
		}
	}

	if err := setSchemaVersion(2); err != nil {
		t.Fatalf("setSchemaVersion error = %v", err)
	}
}

// queryTrafficAgg 读取指定每日流量聚合表，返回 date→bytes 映射。
func queryTrafficAgg(t *testing.T, table string) map[string]int64 {
	t.Helper()
	got := map[string]int64{}
	rows, err := DB.Query("SELECT date, bytes_downloaded FROM " + table + " ORDER BY date")
	if err != nil {
		t.Fatalf("query %s error = %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var date string
		var bytes int64
		if err := rows.Scan(&date, &bytes); err != nil {
			t.Fatalf("scan error = %v", err)
		}
		got[date] = bytes
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err = %v", err)
	}
	return got
}

func TestRunMigrations_V3BackfillsCompletedTraffic(t *testing.T) {
	d := setupSQLiteDB(t)
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	applyV2SchemaWithTraffic(t, d)

	if err := runMigrations(); err != nil {
		t.Fatalf("runMigrations error = %v", err)
	}

	// v2 → v3：schema_version 推进到最新
	v, err := getSchemaVersion()
	if err != nil {
		t.Fatalf("getSchemaVersion error = %v", err)
	}
	if v != CurrentSchemaVersion {
		t.Fatalf("expected schema_version=%d, got %d", CurrentSchemaVersion, v)
	}

	// daily_completed_traffic 应从 daily_traffic 完整回填
	want := map[string]int64{
		"2026-05-01": 3072,
		"2026-05-02": 4096,
	}
	got := queryTrafficAgg(t, "daily_completed_traffic")
	if len(got) != len(want) {
		t.Fatalf("daily_completed_traffic rows count mismatch: got %d, want %d (got=%v)", len(got), len(want), got)
	}
	for date, wantBytes := range want {
		if gotBytes, ok := got[date]; !ok {
			t.Fatalf("daily_completed_traffic missing date %s", date)
		} else if gotBytes != wantBytes {
			t.Fatalf("daily_completed_traffic[%s] = %d, want %d", date, gotBytes, wantBytes)
		}
	}
}

func TestMigrateV3CompletedTraffic_Idempotent(t *testing.T) {
	d := setupSQLiteDB(t)
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	applyV2SchemaWithTraffic(t, d)

	// 直接重复执行 v3 Up，验证幂等（不报错、不产生重复行）
	for i := 0; i < 3; i++ {
		if err := migrateV3CompletedTraffic(d); err != nil {
			t.Fatalf("migrateV3CompletedTraffic attempt %d error = %v", i, err)
		}
	}

	got := queryTrafficAgg(t, "daily_completed_traffic")
	if len(got) != 2 {
		t.Fatalf("expected 2 rows after repeated migration, got %d (got=%v)", len(got), got)
	}
	if got["2026-05-01"] != 3072 || got["2026-05-02"] != 4096 {
		t.Fatalf("unexpected backfill result: %v", got)
	}

	// 已回填的行不被覆盖：INSERT OR IGNORE 而非 REPLACE
	if _, err := DB.Exec("UPDATE daily_completed_traffic SET bytes_downloaded = 1 WHERE date = ?", "2026-05-01"); err != nil {
		t.Fatalf("update error = %v", err)
	}
	if err := migrateV3CompletedTraffic(d); err != nil {
		t.Fatalf("re-run migrateV3CompletedTraffic error = %v", err)
	}
	got = queryTrafficAgg(t, "daily_completed_traffic")
	if got["2026-05-01"] != 1 {
		t.Fatalf("re-run should not overwrite existing row, got %d", got["2026-05-01"])
	}
}

func TestRecordDownloadEventAggregatesMatchingDownloads(t *testing.T) {
	d := setupSQLiteDB(t)
	prev := DB
	DB = d
	isMySQL = false
	isPostgres = false
	t.Cleanup(func() { DB = prev })
	if err := createTables(); err != nil {
		t.Fatalf("createTables error = %v", err)
	}
	event := DownloadEvent{
		FilePath:    "fcl/1/file.apk",
		FileName:    "file.apk",
		Launcher:    "fcl",
		Version:     "1",
		ClientIP:    "1.2.3.4",
		Country:     "CN",
		BytesServed: 100,
		Completed:   true,
		StatusCode:  200,
		Date:        "2026-08-20",
	}
	if err := RecordDownloadEvent(event); err != nil {
		t.Fatalf("first RecordDownloadEvent error = %v", err)
	}
	event.BytesServed = 250
	if err := RecordDownloadEvent(event); err != nil {
		t.Fatalf("second RecordDownloadEvent error = %v", err)
	}
	var rows, count int64
	if err := DB.QueryRow("SELECT COUNT(*), COALESCE(SUM(event_count), 0) FROM download_events").Scan(&rows, &count); err != nil {
		t.Fatalf("query aggregate error = %v", err)
	}
	if rows != 1 || count != 2 {
		t.Fatalf("aggregate rows/count = %d/%d, want 1/2", rows, count)
	}
	var bytes int64
	if err := DB.QueryRow("SELECT bytes_served FROM download_events").Scan(&bytes); err != nil {
		t.Fatalf("query bytes error = %v", err)
	}
	if bytes != 350 {
		t.Fatalf("aggregate bytes = %d, want 350", bytes)
	}
}

func TestRecordCompletedTraffic_Roundtrip(t *testing.T) {
	d := setupSQLiteDB(t)
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	if err := createTables(); err != nil {
		t.Fatalf("createTables error = %v", err)
	}

	// 同日两次记录应累加
	if err := RecordCompletedTraffic(1024); err != nil {
		t.Fatalf("RecordCompletedTraffic error = %v", err)
	}
	if err := RecordCompletedTraffic(2048); err != nil {
		t.Fatalf("RecordCompletedTraffic error = %v", err)
	}

	total, err := GetTotalCompletedTraffic()
	if err != nil {
		t.Fatalf("GetTotalCompletedTraffic error = %v", err)
	}
	if total != 3072 {
		t.Fatalf("GetTotalCompletedTraffic = %d, want 3072", total)
	}

	stats, err := GetDailyCompletedTrafficStats(30)
	if err != nil {
		t.Fatalf("GetDailyCompletedTrafficStats error = %v", err)
	}
	if len(stats) != 1 || stats[0].Bytes != 3072 {
		t.Fatalf("GetDailyCompletedTrafficStats = %v, want 1 row of 3072", stats)
	}

	// served 口径表（daily_traffic）不受完整传输记录影响
	servedTotal, err := GetTotalTraffic()
	if err != nil {
		t.Fatalf("GetTotalTraffic error = %v", err)
	}
	if servedTotal != 0 {
		t.Fatalf("GetTotalTraffic = %d, want 0", servedTotal)
	}
}

// applyV3SchemaWithDownloads 模拟 v3 完成状态的库：含 downloads 历史数据与
// schema_version=3 的 system_info，但无 v4 表。用于验证 v4 迁移的建表与回填。
func applyV3SchemaWithDownloads(t *testing.T, d *sql.DB) {
	t.Helper()
	queries := []string{
		`CREATE TABLE IF NOT EXISTS downloads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_name TEXT, launcher TEXT, version TEXT, ip TEXT, country TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS system_info (
			key TEXT PRIMARY KEY, value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			t.Fatalf("applyV3SchemaWithDownloads exec error = %v, query=%s", err, q)
		}
	}
	rows := []struct{ file, launcher, version, country string }{
		{"a.apk", "fcl", "1.0.0", "China"},
		{"b.apk", "fcl", "1.1.0", "Japan"},
		{"c.jar", "hmcl", "3.0", "China"},
	}
	for _, r := range rows {
		if _, err := d.Exec(
			"INSERT INTO downloads (file_name, launcher, version, ip, country) VALUES (?, ?, ?, '', ?)",
			r.file, r.launcher, r.version, r.country,
		); err != nil {
			t.Fatalf("insert downloads error = %v", err)
		}
	}
	if err := setSchemaVersion(3); err != nil {
		t.Fatalf("setSchemaVersion error = %v", err)
	}
}

func TestRunMigrations_V4BackfillsEvents(t *testing.T) {
	d := setupSQLiteDB(t)
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	applyV3SchemaWithDownloads(t, d)

	if err := runMigrations(); err != nil {
		t.Fatalf("runMigrations error = %v", err)
	}

	v, err := getSchemaVersion()
	if err != nil {
		t.Fatalf("getSchemaVersion error = %v", err)
	}
	if v != CurrentSchemaVersion {
		t.Fatalf("expected schema_version=%d, got %d", CurrentSchemaVersion, v)
	}

	// 三条 downloads 全部回填进 download_events
	var count int
	if err := DB.QueryRow("SELECT COUNT(*) FROM download_events").Scan(&count); err != nil {
		t.Fatalf("count download_events error = %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 backfilled events, got %d", count)
	}

	// 回填行：bytes_served=0, completed=0, source=downloads_import
	var served, completed int
	var source string
	if err := DB.QueryRow("SELECT bytes_served, completed, source FROM download_events WHERE source_id = 1").Scan(&served, &completed, &source); err != nil {
		t.Fatalf("scan backfilled event error = %v", err)
	}
	if served != 0 || completed != 0 || source != "downloads_import" {
		t.Fatalf("backfilled row mismatch: served=%d completed=%d source=%s", served, completed, source)
	}

	// 下载次数/top 聚合从事件表派生
	top, err := GetTopDownloadsFromEvents(10)
	if err != nil {
		t.Fatalf("GetTopDownloadsFromEvents error = %v", err)
	}
	// fcl 占 2 次，hmcl 占 1 次
	var fcl, hmcl int64
	for _, r := range top {
		switch r.Launcher {
		case "fcl":
			fcl = r.Count
		case "hmcl":
			hmcl = r.Count
		}
	}
	if fcl != 2 || hmcl != 1 {
		t.Fatalf("top downloads from events: fcl=%d hmcl=%d, want 2/1", fcl, hmcl)
	}
}

func TestMigrateV4DownloadStatusTables_Idempotent(t *testing.T) {
	d := setupSQLiteDB(t)
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	applyV3SchemaWithDownloads(t, d)

	if err := migrateV4DownloadStatusTables(d); err != nil {
		t.Fatalf("first migrateV4 error = %v", err)
	}

	var before int
	if err := DB.QueryRow("SELECT COUNT(*) FROM download_events").Scan(&before); err != nil {
		t.Fatalf("count before error = %v", err)
	}

	// 重复执行不应产生重复行
	for i := 0; i < 3; i++ {
		if err := migrateV4DownloadStatusTables(d); err != nil {
			t.Fatalf("migrateV4 attempt %d error = %v", i, err)
		}
	}

	var after int
	if err := DB.QueryRow("SELECT COUNT(*) FROM download_events").Scan(&after); err != nil {
		t.Fatalf("count after error = %v", err)
	}
	if after != before {
		t.Fatalf("idempotent check failed: before=%d, after=%d", before, after)
	}
}

func TestDownloadAuthorizationConsumeAndEventRoundtrip(t *testing.T) {
	d := setupSQLiteDB(t)
	prev := DB
	DB = d
	isMySQL = false
	t.Cleanup(func() { DB = prev })

	if err := createTables(); err != nil {
		t.Fatalf("createTables error = %v", err)
	}

	// 签发一条授权
	auth := DownloadAuthorization{
		AuthorizationID: "auth_1",
		TokenHash:       "deadbeef",
		FilePath:        "fcl/1.0.0/a.apk",
		Source:          "homepage",
		Flow:            "prepare",
		ClientIP:        "1.2.3.4",
		SourceKind:      "api",
		Status:          "issued",
		ExpiresAt:       timeNowPlusMinutes(5),
		MaxBytes:        1024,
		RequestID:       "req_1",
	}
	if err := CreateDownloadAuthorization(auth); err != nil {
		t.Fatalf("CreateDownloadAuthorization error = %v", err)
	}

	// 重复 token_hash 应失败
	if err := CreateDownloadAuthorization(auth); err == nil {
		t.Fatal("expected duplicate token_hash error, got nil")
	}

	// Peek（GetByTokenHash）应返回 issued
	loaded, err := GetDownloadAuthorizationByTokenHash("deadbeef")
	if err != nil {
		t.Fatalf("GetDownloadAuthorizationByTokenHash error = %v", err)
	}
	if loaded.Status != "issued" || loaded.FilePath != "fcl/1.0.0/a.apk" {
		t.Fatalf("loaded mismatch: %+v", loaded)
	}

	// 消费：第一次成功，第二次失败（已 consumed）
	consumed, ok, err := ConsumeDownloadAuthorization("deadbeef")
	if err != nil || !ok {
		t.Fatalf("first consume failed: ok=%v err=%v", ok, err)
	}
	if consumed.Status != "consumed" {
		t.Fatalf("consumed status = %s, want consumed", consumed.Status)
	}
	if _, ok2, _ := ConsumeDownloadAuthorization("deadbeef"); ok2 {
		t.Fatal("second consume should fail (already consumed)")
	}

	// 过期授权不应被消费
	expired := DownloadAuthorization{
		AuthorizationID: "auth_2",
		TokenHash:       "cafebabe",
		FilePath:        "fcl/1.0.0/a.apk",
		Status:          "issued",
		ExpiresAt:       timeNowPlusMinutes(-1), // 已过期
	}
	if err := CreateDownloadAuthorization(expired); err != nil {
		t.Fatalf("create expired auth error = %v", err)
	}
	if _, ok3, _ := ConsumeDownloadAuthorization("cafebabe"); ok3 {
		t.Fatal("consuming expired authorization should fail")
	}

	// 写入事件并聚合
	if err := RecordDownloadEvent(DownloadEvent{
		AuthorizationID: "auth_1",
		FilePath:        "fcl/1.0.0/a.apk",
		FileName:        "a.apk",
		Launcher:        "fcl",
		Version:         "1.0.0",
		ClientIP:        "1.2.3.4",
		BytesServed:     512,
		Completed:       true,
		StatusCode:      200,
	}); err != nil {
		t.Fatalf("RecordDownloadEvent error = %v", err)
	}

	totalServed, _ := GetTotalServedFromEvents()
	totalCompleted, _ := GetTotalCompletedFromEvents()
	if totalServed != 512 {
		t.Fatalf("total served = %d, want 512", totalServed)
	}
	if totalCompleted != 512 {
		t.Fatalf("total completed = %d, want 512", totalCompleted)
	}

	ipDaily, _ := GetDailyServedByIPFromEvents("1.2.3.4", todayUTC())
	if ipDaily != 512 {
		t.Fatalf("ip daily served = %d, want 512", ipDaily)
	}

	// 清理过期授权
	n, err := CleanupExpiredAuthorizations()
	if err != nil {
		t.Fatalf("CleanupExpiredAuthorizations error = %v", err)
	}
	if n != 1 {
		t.Fatalf("cleanup expired = %d, want 1", n)
	}
	marked, _ := GetDownloadAuthorizationByTokenHash("cafebabe")
	if marked.Status != "expired" {
		t.Fatalf("expired auth status = %s, want expired", marked.Status)
	}
}

func timeNowPlusMinutes(min int) string {
	return timeNow().Add(time.Duration(min) * time.Minute).UTC().Format(AuthzTimeFormat)
}

func timeNow() time.Time { return time.Now() }

func todayUTC() string { return time.Now().UTC().Format("2006-01-02") }
