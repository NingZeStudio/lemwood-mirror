package db

import (
	"database/sql"
	"errors"
	"fmt"
	"lemwood_mirror/internal/config"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const postgresCleanMigrationKey = "postgres_clean_migration_v1"

type migrationSource struct {
	db      *sql.DB
	dialect string
	name    string
}

func migratePostgresFromConfiguredSources(storagePath string, cfg *config.Config) error {
	done, err := postgresMigrationDone()
	if err != nil {
		return err
	}
	if done {
		log.Println("[数据库迁移] PostgreSQL 清洗迁移已完成，跳过")
		return nil
	}

	source, err := openPreferredMigrationSource(storagePath, cfg)
	if err != nil {
		return err
	}
	if source == nil {
		log.Println("[数据库迁移] MySQL 不可用且未发现 SQLite，使用空 PostgreSQL 数据库")
		return nil
	}
	defer source.db.Close()

	delay, err := time.ParseDuration(cfg.PostgresMigrationDelay)
	if err != nil || delay < 0 {
		delay = 250 * time.Millisecond
	}
	batch := cfg.PostgresMigrationBatch
	if batch <= 0 {
		batch = 200
	}
	log.Printf("[数据库迁移] 从 %s 清洗迁移到 PostgreSQL: batch=%d delay=%s", source.name, batch, delay)
	if err := cleanMigrateToPostgres(source, batch, delay); err != nil {
		return err
	}
	if err := markPostgresMigrationDone(source.name); err != nil {
		return err
	}
	log.Printf("[数据库迁移] %s -> PostgreSQL 清洗迁移完成", source.name)
	return nil
}

func postgresMigrationDone() (bool, error) {
	var value string
	err := DB.QueryRow(`SELECT value FROM system_info WHERE "key"=$1`, postgresCleanMigrationKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil && value != "", err
}

func markPostgresMigrationDone(source string) error {
	value := source + ":" + time.Now().UTC().Format(time.RFC3339)
	_, err := DB.Exec(`INSERT INTO system_info ("key", value) VALUES ($1, $2)
		ON CONFLICT ("key") DO UPDATE SET value=EXCLUDED.value`, postgresCleanMigrationKey, value)
	return err
}

func openPreferredMigrationSource(storagePath string, cfg *config.Config) (*migrationSource, error) {
	if cfg.MySQLHost != "" && cfg.MySQLUser != "" && cfg.MySQLDatabase != "" && cfg.MySQLPort > 0 {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=UTC&timeout=5s&readTimeout=5m",
			cfg.MySQLUser, cfg.MySQLPassword, cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLDatabase)
		d, err := sql.Open("mysql", dsn)
		if err == nil {
			d.SetMaxOpenConns(1)
			d.SetMaxIdleConns(1)
			if pingErr := d.Ping(); pingErr == nil {
				return &migrationSource{db: d, dialect: "mysql", name: "MySQL"}, nil
			} else {
				log.Printf("[数据库迁移] MySQL 连接失败，检查 SQLite: %v", pingErr)
			}
			d.Close()
		} else {
			log.Printf("[数据库迁移] MySQL 打开失败，检查 SQLite: %v", err)
		}
	} else {
		log.Println("[数据库迁移] MySQL 配置不完整，检查 SQLite")
	}

	sqlitePath := filepath.Join(storagePath, "stats.db")
	if _, err := os.Stat(sqlitePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	d, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 迁移源失败: %w", err)
	}
	d.SetMaxOpenConns(1)
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, fmt.Errorf("连接 SQLite 迁移源失败: %w", err)
	}
	return &migrationSource{db: d, dialect: "sqlite", name: "SQLite"}, nil
}

func cleanMigrateToPostgres(source *migrationSource, batch int, delay time.Duration) error {
	for _, table := range []string{
		"visits", "downloads", "download_authorizations", "download_events", "ip_blacklist",
		"ip_daily_traffic", "daily_traffic", "daily_completed_traffic", "stats_snapshot",
	} {
		if _, err := DB.Exec("TRUNCATE TABLE " + table + " RESTART IDENTITY CASCADE"); err != nil {
			return fmt.Errorf("清空 PostgreSQL.%s 失败: %w", table, err)
		}
	}

	if err := migrateVisitsAggregate(source, batch, delay); err != nil {
		return err
	}
	if err := migrateDownloadEventsAggregate(source, batch, delay); err != nil {
		return err
	}
	for _, table := range []string{"ip_blacklist", "ip_daily_traffic", "daily_traffic", "daily_completed_traffic"} {
		if err := copyCompactTable(source, table, batch, delay); err != nil {
			return err
		}
	}
	return copyStartTime(source)
}

func migrateVisitsAggregate(source *migrationSource, batch int, delay time.Duration) error {
	if !sourceTableExists(source, "visits") {
		return nil
	}
	countExpr := "COUNT(*)"
	if sourceColumnExistsDB(source, "visits", "visit_count") {
		countExpr = "SUM(visit_count)"
	}
	dateExpr := "DATE_FORMAT(created_at, '%Y-%m-%d')"
	if source.dialect == "sqlite" {
		dateExpr = "date(created_at)"
	}
	query := fmt.Sprintf(`SELECT %s, COALESCE(country,''), COALESCE(region,''), COALESCE(city,''), %s
		FROM visits GROUP BY %s, country, region, city ORDER BY 1,2,3,4`, dateExpr, countExpr, dateExpr)
	return streamAggregate(source.db, query, batch, delay, func(row []any) (string, []any) {
		date, country, region, city := valueString(row[0]), valueString(row[1]), valueString(row[2]), valueString(row[3])
		createdAt := date + " 00:00:00"
		return `INSERT INTO visits (ip,path,user_agent,referer,country,region,city,visit_count,aggregate_key,created_at)
			VALUES ('','','','',$1,$2,$3,$4,$5,$6) ON CONFLICT (aggregate_key) DO UPDATE SET visit_count=EXCLUDED.visit_count`,
			[]any{country, region, city, valueInt64(row[4]), VisitAggregateKey(date, country, region, city), createdAt}
	})
}

func migrateDownloadEventsAggregate(source *migrationSource, batch int, delay time.Duration) error {
	if !sourceTableExists(source, "download_events") {
		return nil
	}
	countExpr := "COUNT(*)"
	if sourceColumnExistsDB(source, "download_events", "event_count") {
		countExpr = "SUM(event_count)"
	}
	query := `SELECT COALESCE(file_path,''),COALESCE(file_name,''),COALESCE(launcher,''),COALESCE(version,''),
		COALESCE(client_ip,''),COALESCE(country,''),COALESCE(SUM(bytes_served),0),completed,COALESCE(status_code,0),date,` + countExpr + `
		FROM download_events GROUP BY file_path,file_name,launcher,version,client_ip,country,completed,status_code,date
		ORDER BY date,client_ip,file_path`
	return streamAggregate(source.db, query, batch, delay, func(row []any) (string, []any) {
		e := DownloadEvent{
			FilePath: valueString(row[0]), FileName: valueString(row[1]), Launcher: valueString(row[2]),
			Version: valueString(row[3]), ClientIP: valueString(row[4]), Country: valueString(row[5]),
			Completed: valueInt64(row[7]) == 1, StatusCode: int(valueInt64(row[8])), Date: valueString(row[9]),
		}
		return `INSERT INTO download_events (authorization_id,file_path,file_name,launcher,version,client_ip,country,
			bytes_served,completed,status_code,date,event_count,aggregate_key,created_at)
			VALUES ('',$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (aggregate_key) DO UPDATE SET bytes_served=EXCLUDED.bytes_served,event_count=EXCLUDED.event_count`,
			[]any{e.FilePath, e.FileName, e.Launcher, e.Version, e.ClientIP, e.Country,
				valueInt64(row[6]), valueInt64(row[7]), valueInt64(row[8]), e.Date, valueInt64(row[10]),
				DownloadEventAggregateKey(e), e.Date + " 00:00:00"}
	})
}

func streamAggregate(source *sql.DB, query string, batch int, delay time.Duration, convert func([]any) (string, []any)) error {
	rows, err := source.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	count := 0
	commit := func() error {
		if err := tx.Commit(); err != nil {
			return err
		}
		if delay > 0 {
			time.Sleep(delay)
		}
		tx, err = DB.Begin()
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(values))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return err
		}
		for i, value := range values {
			if raw, ok := value.([]byte); ok {
				values[i] = strings.ReplaceAll(string(raw), "\x00", "")
			}
		}
		query, args := convert(values)
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
		count++
		if count%batch == 0 {
			if err := commit(); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return tx.Commit()
}

func copyCompactTable(source *migrationSource, table string, batch int, delay time.Duration) error {
	if !sourceTableExists(source, table) {
		return nil
	}
	columns := map[string][]string{
		"ip_blacklist":            {"ip", "reason", "source", "ban_type", "created_at"},
		"ip_daily_traffic":        {"ip", "date", "bytes_downloaded"},
		"daily_traffic":           {"date", "bytes_downloaded"},
		"daily_completed_traffic": {"date", "bytes_downloaded"},
	}[table]
	selectQuery := "SELECT " + strings.Join(columns, ",") + " FROM " + table
	if table == "ip_daily_traffic" {
		// The limiter only needs today's IP budget. Historical IP rows are not
		// statistics and would recreate the very table growth this mode avoids.
		selectQuery += " WHERE date='" + time.Now().UTC().Format("2006-01-02") + "'"
	}
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	insertQuery := "INSERT INTO " + table + " (" + strings.Join(columns, ",") + ") VALUES (" + strings.Join(placeholders, ",") + ") ON CONFLICT DO NOTHING"
	return streamAggregate(source.db, selectQuery, batch, delay, func(row []any) (string, []any) {
		switch table {
		case "ip_daily_traffic":
			row = []any{valueString(row[0]), valueString(row[1]), valueInt64(row[2])}
		case "daily_traffic", "daily_completed_traffic":
			row = []any{valueString(row[0]), valueInt64(row[1])}
		case "ip_blacklist":
			row = []any{valueString(row[0]), valueString(row[1]), valueString(row[2]), valueString(row[3]), valueString(row[4])}
		}
		return insertQuery, row
	})
}

func copyStartTime(source *migrationSource) error {
	if !sourceTableExists(source, "system_info") {
		return nil
	}
	key := "`key`"
	if source.dialect == "sqlite" {
		key = "key"
	}
	var value any
	if err := source.db.QueryRow("SELECT value FROM system_info WHERE "+key+"=?", "start_time").Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	_, err := DB.Exec(`INSERT INTO system_info ("key",value) VALUES ($1,$2)
		ON CONFLICT ("key") DO UPDATE SET value=EXCLUDED.value`, "start_time", valueString(value))
	return err
}

func sourceTableExists(source *migrationSource, table string) bool {
	var count int
	if source.dialect == "mysql" {
		return source.db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?", table).Scan(&count) == nil && count > 0
	}
	return source.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count) == nil && count > 0
}

func sourceColumnExistsDB(source *migrationSource, table, column string) bool {
	if source.dialect == "mysql" {
		var count int
		return source.db.QueryRow("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", table, column).Scan(&count) == nil && count > 0
	}
	rows, err := source.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typeName string
		var defaultValue any
		if rows.Scan(&cid, &name, &typeName, &notnull, &defaultValue, &pk) == nil && name == column {
			return true
		}
	}
	return false
}

func valueString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case []byte:
		return strings.ReplaceAll(string(v), "\x00", "")
	case time.Time:
		return v.UTC().Format(AuthzTimeFormat)
	default:
		return strings.ReplaceAll(fmt.Sprint(v), "\x00", "")
	}
}

func valueInt64(value any) int64 {
	var result int64
	fmt.Sscan(valueString(value), &result)
	return result
}
