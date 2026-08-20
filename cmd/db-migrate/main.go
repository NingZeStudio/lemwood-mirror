// db-migrate copies the mirror's MySQL data into PostgreSQL.
// It is intentionally separate from the server so migration cannot happen
// accidentally during a normal application start.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"

	_ "github.com/go-sql-driver/mysql"
)

type tablePlan struct {
	name        string
	columns     string
	targetCols  string
	keyColumns  []string
	keyExpr     string
	identityCol string
}

var tablePlans = []tablePlan{
	{name: "visits", columns: "id, ip, path, user_agent, referer, country, region, city, created_at", targetCols: "id, ip, path, user_agent, referer, country, region, city, created_at", keyColumns: []string{"id"}, keyExpr: "id", identityCol: "id"},
	{name: "downloads", columns: "id, file_name, launcher, version, ip, country, created_at", targetCols: "id, file_name, launcher, version, ip, country, created_at", keyColumns: []string{"id"}, keyExpr: "id", identityCol: "id"},
	{name: "ip_blacklist", columns: "ip, reason, source, ban_type, created_at", targetCols: "ip, reason, source, ban_type, created_at", keyColumns: []string{"ip"}, keyExpr: "ip"},
	{name: "ip_daily_traffic", columns: "ip, date, bytes_downloaded", targetCols: "ip, date, bytes_downloaded", keyColumns: []string{"ip", "date"}, keyExpr: "(ip, date)"},
	{name: "daily_traffic", columns: "date, bytes_downloaded", targetCols: "date, bytes_downloaded", keyColumns: []string{"date"}, keyExpr: "date"},
	{name: "daily_completed_traffic", columns: "date, bytes_downloaded", targetCols: "date, bytes_downloaded", keyColumns: []string{"date"}, keyExpr: "date"},
	{name: "download_authorizations", columns: "authorization_id, token_hash, file_path, return_url, source, flow, client_ip, source_kind, status, expires_at, max_bytes, range_limit, request_id, first_transfer_at, created_at, consumed_at", targetCols: "authorization_id, token_hash, file_path, return_url, source, flow, client_ip, source_kind, status, expires_at, max_bytes, range_limit, request_id, first_transfer_at, created_at, consumed_at", keyColumns: []string{"authorization_id"}, keyExpr: "authorization_id"},
	{name: "download_events", columns: "id, authorization_id, file_path, file_name, launcher, version, client_ip, country, bytes_served, completed, status_code, date, source, source_id, created_at", targetCols: "id, authorization_id, file_path, file_name, launcher, version, client_ip, country, bytes_served, completed, status_code, date, source, source_id, created_at", keyColumns: []string{"id"}, keyExpr: "id", identityCol: "id"},
	{name: "system_info", columns: "`key`, value, created_at", targetCols: `"key", value, created_at`, keyColumns: []string{"`key`"}, keyExpr: "`key`"},
}

func main() {
	var sourceHost, sourceUser, sourcePassword, sourceDatabase string
	var sourcePort, batch int
	var targetHost, targetUser, targetPassword, targetDatabase, targetSSLMode string
	var targetPort int
	var sleep time.Duration
	var verifyCounts, clean bool

	flag.StringVar(&sourceHost, "source-host", envOr("MYSQL_HOST", ""), "MySQL source host")
	flag.IntVar(&sourcePort, "source-port", envInt("MYSQL_PORT", 3306), "MySQL source port")
	flag.StringVar(&sourceUser, "source-user", envOr("MYSQL_USER", ""), "MySQL source user")
	flag.StringVar(&sourcePassword, "source-password", envOr("MYSQL_PASSWORD", ""), "MySQL source password")
	flag.StringVar(&sourceDatabase, "source-database", envOr("MYSQL_DATABASE", ""), "MySQL source database")
	flag.StringVar(&targetHost, "target-host", envOr("PGHOST", "127.0.0.1"), "PostgreSQL target host")
	flag.IntVar(&targetPort, "target-port", envInt("PGPORT", 5432), "PostgreSQL target port")
	flag.StringVar(&targetUser, "target-user", envOr("PGUSER", ""), "PostgreSQL target user")
	flag.StringVar(&targetPassword, "target-password", envOr("PGPASSWORD", ""), "PostgreSQL target password")
	flag.StringVar(&targetDatabase, "target-database", envOr("PGDATABASE", ""), "PostgreSQL target database")
	flag.StringVar(&targetSSLMode, "target-sslmode", envOr("PGSSLMODE", "disable"), "PostgreSQL SSL mode")
	flag.IntVar(&batch, "batch", 200, "Rows per transaction; keep small on poor IO servers")
	flag.DurationVar(&sleep, "sleep", 250*time.Millisecond, "Pause between batches")
	flag.BoolVar(&verifyCounts, "verify-counts", false, "Run full COUNT(*) checks after each table")
	flag.BoolVar(&clean, "clean", false, "Migrate compact statistics aggregates only; skip raw downloads/auth/events")
	flag.Parse()

	if sourceHost == "" || sourceUser == "" || sourceDatabase == "" || targetUser == "" || targetDatabase == "" {
		log.Fatal("source and target host/user/database are required; use flags or MYSQL_*/PG* environment variables")
	}
	if batch <= 0 {
		log.Fatal("-batch must be greater than zero")
	}
	if sleep < 0 {
		log.Fatal("-sleep cannot be negative")
	}

	source, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=UTC&readTimeout=5m&writeTimeout=5m", sourceUser, sourcePassword, sourceHost, sourcePort, sourceDatabase))
	if err != nil {
		log.Fatalf("open MySQL source: %v", err)
	}
	defer source.Close()
	source.SetMaxOpenConns(1)
	source.SetMaxIdleConns(1)
	if err := source.Ping(); err != nil {
		log.Fatalf("ping MySQL source: %v", err)
	}

	targetCfg := &config.Config{
		PostgresHost:     targetHost,
		PostgresPort:     targetPort,
		PostgresUser:     targetUser,
		PostgresPassword: targetPassword,
		PostgresDatabase: targetDatabase,
		PostgresSSLMode:  targetSSLMode,
	}
	if err := db.InitDB(".", targetCfg); err != nil {
		log.Fatalf("initialize PostgreSQL target: %v", err)
	}
	defer db.DB.Close()
	db.DB.SetMaxOpenConns(1)
	db.DB.SetMaxIdleConns(1)

	log.Printf("starting low-IO migration: batch=%d sleep=%s; source is read-only", batch, sleep)
	if clean {
		if err := migrateClean(source, db.DB, batch, sleep); err != nil {
			log.Fatalf("clean migration: %v", err)
		}
		log.Println("clean migration completed; raw request/download details were skipped")
		return
	}
	for _, plan := range tablePlans {
		migrated, err := migrateTable(source, db.DB, plan, batch, sleep)
		if err != nil {
			log.Fatalf("migrate %s: %v", plan.name, err)
		}
		log.Printf("migrated %s: %d rows", plan.name, migrated)
		if verifyCounts {
			if err := verifyTableCounts(source, db.DB, plan.name); err != nil {
				log.Fatalf("verify %s: %v", plan.name, err)
			}
		}
		if plan.identityCol != "" {
			if err := resetIdentity(db.DB, plan.name, plan.identityCol); err != nil {
				log.Fatalf("reset %s identity: %v", plan.name, err)
			}
		}
	}
	log.Println("migration completed; source database was not modified")
}

// migrateClean keeps only the data needed by the statistics and traffic-limit
// paths. The target is reset first so rerunning the command is deterministic.
func migrateClean(source, target *sql.DB, batch int, pause time.Duration) error {
	for _, table := range []string{"visits", "download_events", "ip_daily_traffic", "daily_traffic", "daily_completed_traffic", "system_info"} {
		if _, err := target.Exec("TRUNCATE TABLE " + table + " RESTART IDENTITY CASCADE"); err != nil {
			return fmt.Errorf("clear target %s: %w", table, err)
		}
	}

	visitCountColumn, err := sourceColumnExists(source, "visits", "visit_count")
	if err != nil {
		return err
	}
	visitCount := "COUNT(*)"
	if visitCountColumn {
		visitCount = "SUM(visit_count)"
	}
	if err := aggregateTable(source, target,
		"SELECT DATE_FORMAT(created_at, '%Y-%m-%d 00:00:00'), country, region, city, "+visitCount+" FROM visits GROUP BY DATE_FORMAT(created_at, '%Y-%m-%d'), country, region, city ORDER BY 1, 2, 3, 4",
		`INSERT INTO visits (ip, path, user_agent, referer, country, region, city, created_at, visit_count, aggregate_key) VALUES (?, '', '', '', ?, ?, ?, ?, ?, ?)`,
		func(values []any) []any {
			date := fmt.Sprint(values[0])
			return []any{"", values[1], values[2], values[3], values[0], values[4], db.VisitAggregateKey(date[:10], fmt.Sprint(values[1]), fmt.Sprint(values[2]), fmt.Sprint(values[3]))}
		}, batch, pause); err != nil {
		return fmt.Errorf("aggregate visits: %w", err)
	}

	eventCountColumn, err := sourceColumnExists(source, "download_events", "event_count")
	if err != nil {
		return err
	}
	eventCount := "COUNT(*)"
	if eventCountColumn {
		eventCount = "SUM(event_count)"
	}
	if err := aggregateTable(source, target,
		"SELECT file_path, file_name, launcher, version, client_ip, country, SUM(bytes_served), completed, status_code, date, "+eventCount+" FROM download_events GROUP BY file_path, file_name, launcher, version, client_ip, country, completed, status_code, date ORDER BY date, client_ip, file_path",
		`INSERT INTO download_events (authorization_id, file_path, file_name, launcher, version, client_ip, country, bytes_served, completed, status_code, date, event_count, aggregate_key, created_at) VALUES ('', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		func(values []any) []any {
			event := db.DownloadEvent{
				FilePath: fmt.Sprint(values[0]), FileName: fmt.Sprint(values[1]),
				Launcher: fmt.Sprint(values[2]), Version: fmt.Sprint(values[3]),
				ClientIP: fmt.Sprint(values[4]), Country: fmt.Sprint(values[5]),
				Completed: fmt.Sprint(values[7]) == "1", StatusCode: intValue(values[8]), Date: fmt.Sprint(values[9]),
			}
			return []any{values[0], values[1], values[2], values[3], values[4], values[5], values[6], values[7], values[8], values[9], values[10], db.DownloadEventAggregateKey(event), values[9]}
		}, batch, pause); err != nil {
		return fmt.Errorf("aggregate download_events: %w", err)
	}

	for _, plan := range []tablePlan{
		{name: "ip_daily_traffic", columns: "ip, date, bytes_downloaded", targetCols: "ip, date, bytes_downloaded", keyColumns: []string{"ip", "date"}, keyExpr: "(ip, date)"},
		{name: "daily_traffic", columns: "date, bytes_downloaded", targetCols: "date, bytes_downloaded", keyColumns: []string{"date"}, keyExpr: "date"},
		{name: "daily_completed_traffic", columns: "date, bytes_downloaded", targetCols: "date, bytes_downloaded", keyColumns: []string{"date"}, keyExpr: "date"},
		{name: "system_info", columns: "`key`, value, created_at", targetCols: `"key", value, created_at`, keyColumns: []string{"`key`"}, keyExpr: "`key`"},
	} {
		if _, err := migrateTable(source, target, plan, batch, pause); err != nil {
			return fmt.Errorf("copy %s: %w", plan.name, err)
		}
	}
	return nil
}

func intValue(value any) int {
	var result int
	if _, err := fmt.Sscanf(fmt.Sprint(value), "%d", &result); err != nil {
		return 0
	}
	return result
}

func aggregateTable(source, target *sql.DB, sourceQuery, targetQuery string, convert func([]any) []any, batch int, pause time.Duration) error {
	rows, err := source.Query(sourceQuery)
	if err != nil {
		return err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	values := make([][]any, 0, batch)
	flush := func() error {
		if len(values) == 0 {
			return nil
		}
		tx, err := target.Begin()
		if err != nil {
			return err
		}
		stmt, err := tx.Prepare(db.Rebind(targetQuery + " ON CONFLICT DO NOTHING"))
		if err != nil {
			tx.Rollback()
			return err
		}
		for _, row := range values {
			if _, err := stmt.Exec(row...); err != nil {
				stmt.Close()
				tx.Rollback()
				return err
			}
		}
		stmt.Close()
		if err := tx.Commit(); err != nil {
			return err
		}
		values = values[:0]
		if pause > 0 {
			time.Sleep(pause)
		}
		return nil
	}
	for rows.Next() {
		row := make([]any, len(columns))
		pointers := make([]any, len(row))
		for i := range row {
			pointers[i] = &row[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return err
		}
		values = append(values, convert(normalizeValues(row)))
		if len(values) >= batch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return flush()
}

func sourceColumnExists(source *sql.DB, table, column string) (bool, error) {
	var count int
	err := source.QueryRow("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", table, column).Scan(&count)
	return count > 0, err
}

func migrateTable(source, target *sql.DB, plan tablePlan, batch int, pause time.Duration) (int64, error) {
	last, err := resumeKey(target, plan)
	if err != nil {
		return 0, err
	}
	var total int64
	for {
		query := "SELECT " + plan.columns + " FROM " + plan.name
		args := make([]any, 0, len(plan.keyColumns)+1)
		if len(last) > 0 {
			if len(plan.keyColumns) == 1 {
				query += " WHERE " + plan.keyExpr + " > ?"
			} else {
				query += " WHERE (" + strings.Join(plan.keyColumns, ", ") + ") > (" + strings.TrimRight(strings.Repeat("?, ", len(plan.keyColumns)), ", ") + ")"
			}
			args = append(args, last...)
		}
		query += " ORDER BY " + strings.Join(plan.keyColumns, ", ") + " LIMIT ?"
		args = append(args, batch)
		rows, err := source.Query(query, args...)
		if err != nil {
			if isMissingTable(err) {
				return total, nil
			}
			return total, err
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return total, err
		}
		values := make([][]any, 0, batch)
		for rows.Next() {
			row := make([]any, len(columns))
			pointers := make([]any, len(row))
			for i := range row {
				pointers[i] = &row[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				rows.Close()
				return total, err
			}
			values = append(values, normalizeValues(row))
		}
		rowErr := rows.Err()
		rows.Close()
		if rowErr != nil {
			return total, rowErr
		}
		if len(values) == 0 {
			return total, nil
		}

		tx, err := target.Begin()
		if err != nil {
			return total, err
		}
		placeholders := make([]string, len(columns))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		insert := "INSERT INTO " + plan.name + " (" + plan.targetCols + ") VALUES (" + strings.Join(placeholders, ", ") + ") ON CONFLICT DO NOTHING"
		stmt, err := tx.Prepare(db.Rebind(insert))
		if err != nil {
			tx.Rollback()
			return total, err
		}
		for _, row := range values {
			if _, err := stmt.Exec(row...); err != nil {
				stmt.Close()
				tx.Rollback()
				return total, err
			}
		}
		stmt.Close()
		if err := tx.Commit(); err != nil {
			return total, err
		}

		last = make([]any, len(plan.keyColumns))
		for i := range last {
			last[i] = values[len(values)-1][i]
		}
		total += int64(len(values))
		if pause > 0 {
			time.Sleep(pause)
		}
	}
}

func verifyTableCounts(source, target *sql.DB, table string) error {
	var sourceCount, targetCount int64
	if err := source.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&sourceCount); err != nil {
		if isMissingTable(err) {
			return nil
		}
		return err
	}
	if err := target.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&targetCount); err != nil {
		return err
	}
	if sourceCount != targetCount {
		return fmt.Errorf("row count mismatch: source=%d target=%d", sourceCount, targetCount)
	}
	return nil
}

func resetIdentity(target *sql.DB, table, column string) error {
	_, err := target.Exec("SELECT setval(pg_get_serial_sequence($1, $2), COALESCE(MAX("+column+"), 1), MAX("+column+") IS NOT NULL) FROM "+table, table, column)
	return err
}

func normalizeValues(values []any) []any {
	for i, value := range values {
		if raw, ok := value.([]byte); ok {
			values[i] = strings.ReplaceAll(string(raw), "\x00", "")
		} else if text, ok := value.(string); ok {
			values[i] = strings.ReplaceAll(text, "\x00", "")
		}
	}
	return values
}

// resumeKey assumes tables are copied in key order and each committed batch
// is a prefix. This avoids re-reading millions of already copied rows after a
// transient failure. Composite-key tables deliberately restart from zero and
// rely on ON CONFLICT DO NOTHING because a safe MAX tuple is dialect-specific.
func resumeKey(target *sql.DB, plan tablePlan) ([]any, error) {
	if len(plan.keyColumns) != 1 || plan.identityCol == "" {
		return nil, nil
	}
	var key any
	query := "SELECT MAX(" + plan.keyColumns[0] + ") FROM " + plan.name
	if err := target.QueryRow(query).Scan(&key); err != nil {
		return nil, fmt.Errorf("read resume key for %s: %w", plan.name, err)
	}
	if key == nil {
		return nil, nil
	}
	return []any{key}, nil
}

func isMissingTable(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "doesn't exist") || strings.Contains(message, "does not exist") || strings.Contains(message, "no such table")
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
