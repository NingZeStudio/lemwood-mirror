package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"lemwood_mirror/internal/config"
)

func TestPostgresBuiltInMigrationIntegration(t *testing.T) {
	if testing.Short() || getenv("LEMWOOD_MIGRATION_INTEGRATION") != "1" {
		t.Skip("set LEMWOOD_MIGRATION_INTEGRATION=1 to run local MySQL/PostgreSQL integration")
	}
	if DB != nil {
		DB.Close()
		DB = nil
	}
	cfg := &config.Config{
		DatabaseMode: "pgsql",
		MySQLHost:    "127.0.0.1", MySQLPort: 33306, MySQLUser: "lemwood", MySQLPassword: "testpass", MySQLDatabase: "lemwood_source",
		PostgresHost: "127.0.0.1", PostgresPort: 55432, PostgresUser: getenv("USER"), PostgresDatabase: "lemwood_builtin_test", PostgresSSLMode: "disable",
		PostgresMigrationBatch: 2, PostgresMigrationDelay: "0s",
	}
	if err := InitDB(t.TempDir(), cfg); err != nil {
		t.Fatalf("InitDB first run error = %v", err)
	}
	var marker string
	if err := DB.QueryRow(`SELECT value FROM system_info WHERE "key"=$1`, postgresCleanMigrationKey).Scan(&marker); err != nil {
		t.Fatalf("migration marker error = %v", err)
	}
	var visitsBefore int64
	if err := DB.QueryRow("SELECT COALESCE(SUM(visit_count),0) FROM visits").Scan(&visitsBefore); err != nil {
		t.Fatalf("visit count error = %v", err)
	}
	DB.Close()
	DB = nil
	if err := InitDB(t.TempDir(), cfg); err != nil {
		t.Fatalf("InitDB second run error = %v", err)
	}
	defer func() { DB.Close(); DB = nil }()
	var visitsAfter int64
	if err := DB.QueryRow("SELECT COALESCE(SUM(visit_count),0) FROM visits").Scan(&visitsAfter); err != nil {
		t.Fatalf("visit count after restart error = %v", err)
	}
	if visitsAfter != visitsBefore {
		t.Fatalf("migration repeated: visits before=%d after=%d", visitsBefore, visitsAfter)
	}
}

func TestPostgresBuiltInMigrationFallsBackToSQLiteIntegration(t *testing.T) {
	if testing.Short() || getenv("LEMWOOD_MIGRATION_INTEGRATION") != "1" {
		t.Skip("set LEMWOOD_MIGRATION_INTEGRATION=1 to run local PostgreSQL integration")
	}
	storage := t.TempDir()
	source, err := sql.Open("sqlite", filepath.Join(storage, "stats.db"))
	if err != nil {
		t.Fatalf("open sqlite source: %v", err)
	}
	queries := []string{
		`CREATE TABLE visits (id INTEGER PRIMARY KEY, country TEXT, region TEXT, city TEXT, created_at DATETIME)`,
		`CREATE TABLE download_events (id INTEGER PRIMARY KEY, file_path TEXT, file_name TEXT, launcher TEXT, version TEXT, client_ip TEXT, country TEXT, bytes_served INTEGER, completed INTEGER, status_code INTEGER, date TEXT)`,
		`CREATE TABLE ip_daily_traffic (ip TEXT, date TEXT, bytes_downloaded INTEGER, PRIMARY KEY(ip,date))`,
		`CREATE TABLE daily_traffic (date TEXT PRIMARY KEY, bytes_downloaded INTEGER)`,
		`CREATE TABLE daily_completed_traffic (date TEXT PRIMARY KEY, bytes_downloaded INTEGER)`,
		`CREATE TABLE ip_blacklist (ip TEXT PRIMARY KEY, reason TEXT, source TEXT, ban_type TEXT, created_at DATETIME)`,
		`CREATE TABLE system_info (key TEXT PRIMARY KEY, value TEXT)`,
		`INSERT INTO visits VALUES (1,'CN','A','B','2026-08-20 01:00:00'),(2,'CN','A','B','2026-08-20 02:00:00')`,
		`INSERT INTO download_events VALUES (1,'a','a','fcl','1','1.2.3.4','CN',100,1,200,'2026-08-20'),(2,'a','a','fcl','1','1.2.3.4','CN',200,1,200,'2026-08-20')`,
		`INSERT INTO system_info VALUES ('start_time','2026-08-20 00:00:00')`,
	}
	for _, query := range queries {
		if _, err := source.Exec(query); err != nil {
			source.Close()
			t.Fatalf("prepare sqlite source: %v", err)
		}
	}
	source.Close()

	if DB != nil {
		DB.Close()
		DB = nil
	}
	cfg := &config.Config{
		DatabaseMode: "pgsql",
		MySQLHost:    "127.0.0.1", MySQLPort: 1, MySQLUser: "invalid", MySQLDatabase: "invalid",
		PostgresHost: "127.0.0.1", PostgresPort: 55432, PostgresUser: getenv("USER"), PostgresDatabase: "lemwood_builtin_sqlite_test", PostgresSSLMode: "disable",
		PostgresMigrationBatch: 2, PostgresMigrationDelay: "0s",
	}
	if err := InitDB(storage, cfg); err != nil {
		t.Fatalf("InitDB fallback error = %v", err)
	}
	defer func() { DB.Close(); DB = nil }()
	var visits, events, bytes int64
	if err := DB.QueryRow("SELECT COALESCE(SUM(visit_count),0) FROM visits").Scan(&visits); err != nil {
		t.Fatal(err)
	}
	if err := DB.QueryRow("SELECT COALESCE(SUM(event_count),0),COALESCE(SUM(bytes_served),0) FROM download_events").Scan(&events, &bytes); err != nil {
		t.Fatal(err)
	}
	if visits != 2 || events != 2 || bytes != 300 {
		t.Fatalf("fallback aggregates visits/events/bytes=%d/%d/%d", visits, events, bytes)
	}
}

func getenv(name string) string {
	return os.Getenv(name)
}

func TestOpenPreferredMigrationSourceFallsBackToSQLite(t *testing.T) {
	storage := t.TempDir()
	sqlitePath := filepath.Join(storage, "stats.db")
	d, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := d.Exec("CREATE TABLE visits (id INTEGER PRIMARY KEY)"); err != nil {
		d.Close()
		t.Fatalf("create sqlite source: %v", err)
	}
	d.Close()

	source, err := openPreferredMigrationSource(storage, &config.Config{
		MySQLHost: "127.0.0.1", MySQLPort: 1,
		MySQLUser: "invalid", MySQLDatabase: "invalid",
	})
	if err != nil {
		t.Fatalf("openPreferredMigrationSource error = %v", err)
	}
	if source == nil {
		t.Fatal("expected SQLite fallback source")
	}
	defer source.db.Close()
	if source.dialect != "sqlite" {
		t.Fatalf("source dialect = %q, want sqlite", source.dialect)
	}
}

func TestOpenPreferredMigrationSourceReturnsNilWithoutSources(t *testing.T) {
	source, err := openPreferredMigrationSource(t.TempDir(), &config.Config{})
	if err != nil {
		t.Fatalf("openPreferredMigrationSource error = %v", err)
	}
	if source != nil {
		source.db.Close()
		t.Fatal("expected no migration source")
	}
}
