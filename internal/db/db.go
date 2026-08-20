package db

import (
	"database/sql"
	"fmt"
	"lemwood_mirror/internal/config"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

var (
	DB         *sql.DB
	isMySQL    bool
	isPostgres bool
)

func IsMySQL() bool {
	return isMySQL
}

func IsPostgres() bool { return isPostgres }

// Rebind converts the project's portable ? placeholders to PostgreSQL $n.
// It is exported for packages issuing read/write queries outside db.go.
func Rebind(query string) string { return rebind(query) }

func rebind(query string) string {
	if !isPostgres {
		return query
	}
	result := make([]byte, 0, len(query)+8)
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			result = append(result, '$')
			result = append(result, fmt.Sprintf("%d", n)...)
			continue
		}
		result = append(result, query[i])
	}
	return string(result)
}

func InitDB(storagePath string, cfg *config.Config) error {
	dbPath := filepath.Join(storagePath, "stats.db")
	mode := strings.ToLower(strings.TrimSpace(cfg.DatabaseMode))
	if mode == "" {
		mode = "auto"
	}
	usePostgres := mode == "pgsql" || (mode == "auto" && cfg.PostgresHost != "")
	useMySQL := mode == "mysql" || (mode == "auto" && cfg.PostgresHost == "" && cfg.MySQLHost != "")

	if usePostgres {
		if cfg.PostgresUser == "" || cfg.PostgresDatabase == "" || cfg.PostgresPort <= 0 {
			return fmt.Errorf("PostgreSQL 配置不完整: 必须提供 host, user, database 和有效的 port")
		}
		isMySQL = false
		isPostgres = true
		sslMode := cfg.PostgresSSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDatabase, sslMode)
		var err error
		DB, err = sql.Open("pgx", dsn)
		if err != nil {
			return fmt.Errorf("打开 PostgreSQL 失败: %w", err)
		}
		if err := DB.Ping(); err != nil {
			return fmt.Errorf("连接 PostgreSQL 失败: %w", err)
		}
		DB.SetMaxOpenConns(2)
		DB.SetMaxIdleConns(1)
		DB.SetConnMaxLifetime(time.Hour)
		DB.SetConnMaxIdleTime(30 * time.Minute)
	} else if useMySQL {
		// 验证 MySQL 配置完整性
		if cfg.MySQLUser == "" || cfg.MySQLDatabase == "" || cfg.MySQLPort <= 0 {
			return fmt.Errorf("MySQL 配置不完整: 必须提供 host, user, database 和有效的 port")
		}

		isMySQL = true
		isPostgres = false
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=30s&readTimeout=30s&writeTimeout=30s",
			cfg.MySQLUser, cfg.MySQLPassword, cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLDatabase)

		var err error
		DB, err = sql.Open("mysql", dsn)
		if err != nil {
			return fmt.Errorf("打开 MySQL 失败: %w", err)
		}

		if err := DB.Ping(); err != nil {
			return fmt.Errorf("连接 MySQL 失败: %w", err)
		}

		DB.SetMaxOpenConns(25)
		DB.SetMaxIdleConns(10)
		DB.SetConnMaxLifetime(time.Hour)
		DB.SetConnMaxIdleTime(30 * time.Minute)

		// 检查是否需要从 SQLite 迁移
		if cfg.MySQLMigration {
			if _, err := os.Stat(dbPath); err == nil {
				log.Println("[数据库] 发现 SQLite 数据库，开始自动迁移到 MySQL...")
				if err := migrateFromSQLite(dbPath); err != nil {
					return fmt.Errorf("自动迁移到 MySQL 失败: %w", err)
				}
				log.Println("[数据库] 迁移成功！")
				// 迁移成功后，将 stats.db 重命名为 stats.db.bak
				if err := os.Rename(dbPath, dbPath+".bak"); err != nil {
					log.Printf("[数据库] 备份原 SQLite 文件失败: %v", err)
				} else {
					log.Printf("[数据库] 已将原 SQLite 文件备份为 %s.bak", dbPath)
				}
			}
		}
	} else {
		// 使用 SQLite
		isMySQL = false
		isPostgres = false
		// 确保目录存在
		if err := os.MkdirAll(storagePath, 0755); err != nil {
			return fmt.Errorf("创建数据库目录失败: %w", err)
		}

		var err error
		DB, err = sql.Open("sqlite", dbPath)
		if err != nil {
			return fmt.Errorf("打开 SQLite 失败: %w", err)
		}

		DB.SetMaxOpenConns(1)
		DB.SetConnMaxIdleTime(5 * time.Minute)

		pragmas := []string{
			"PRAGMA journal_mode=WAL",
			"PRAGMA synchronous=NORMAL",
			"PRAGMA busy_timeout=10000",
			"PRAGMA foreign_keys=ON",
		}

		for _, pragma := range pragmas {
			if _, err := DB.Exec(pragma); err != nil {
				return fmt.Errorf("执行 PRAGMA 失败 (%s): %w", pragma, err)
			}
		}
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	if err := createTables(); err != nil {
		return err
	}
	if mode == "pgsql" {
		if err := migratePostgresFromConfiguredSources(storagePath, cfg); err != nil {
			return fmt.Errorf("pgsql 数据迁移失败: %w", err)
		}
	}
	return nil
}

func migrateFromSQLite(sqlitePath string) error {
	sqliteDB, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return fmt.Errorf("打开 SQLite 失败: %w", err)
	}
	defer sqliteDB.Close()

	// 1. 创建表
	if err := createTables(); err != nil {
		return fmt.Errorf("创建 MySQL 表失败: %w", err)
	}

	// 2. 迁移数据
	// 注意：stats_snapshot 是缓存表（id=1, data, updated_at），由统计模块重建，不参与数据迁移。
	// repo 镜像功能已移除，repo_downloads / repo_ip_daily_traffic / daily_repo_traffic 不再迁移。
	tables := []string{"visits", "downloads", "ip_blacklist", "ip_daily_traffic", "daily_traffic", "daily_completed_traffic", "download_authorizations", "download_events", "system_info"}
	for _, table := range tables {
		if err := migrateTable(sqliteDB, DB, table); err != nil {
			return fmt.Errorf("迁移表 %s 失败: %w", table, err)
		}
	}

	return nil
}

func migrateTable(src, dst *sql.DB, tableName string) error {
	rows, err := src.Query(fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") || strings.Contains(strings.ToLower(err.Error()), "doesn't exist") {
			return nil
		}
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	placeholders := make([]string, len(cols))
	escapedCols := make([]string, len(cols))
	for i, col := range cols {
		placeholders[i] = "?"
		if isMySQL {
			escapedCols[i] = "`" + col + "`"
		} else {
			escapedCols[i] = col
		}
	}

	insertCmd := "INSERT INTO"
	if isMySQL {
		insertCmd = "INSERT IGNORE INTO"
	}

	query := fmt.Sprintf("%s %s (%s) VALUES (%s)",
		insertCmd, tableName, strings.Join(escapedCols, ","), strings.Join(placeholders, ","))

	tx, err := dst.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	count := 0
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		// 处理 SQLite 的字节数组/时间字符串到 MySQL 的兼容性，并清洗非法 UTF-8 字符
		for i, val := range values {
			if val == nil {
				continue
			}

			var strVal string
			switch v := val.(type) {
			case []byte:
				strVal = string(v)
			case string:
				strVal = v
			default:
				continue
			}

			// 清洗非法 UTF-8 字符，防止 MySQL 报错 Error 1366
			// \xA0 等非标准空格字符在某些 UA 中很常见，需要处理
			if !utf8.ValidString(strVal) {
				strVal = strings.ToValidUTF8(strVal, "?")
			}
			values[i] = strVal
		}

		if _, err := stmt.Exec(values...); err != nil {
			return err
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("[数据库迁移] 表 %s: 迁移了 %d 条数据", tableName, count)
	return nil
}

func createTables() error {
	if isPostgres {
		for _, query := range postgresTableQueries {
			if _, err := DB.Exec(query); err != nil {
				return fmt.Errorf("创建 PostgreSQL 表/索引失败: %w, query: %s", err, query)
			}
		}
		for _, query := range []string{
			`ALTER TABLE visits ADD COLUMN IF NOT EXISTS visit_count BIGINT NOT NULL DEFAULT 1`,
			`ALTER TABLE download_events ADD COLUMN IF NOT EXISTS event_count BIGINT NOT NULL DEFAULT 1`,
			`ALTER TABLE visits ADD COLUMN IF NOT EXISTS aggregate_key TEXT`,
			`ALTER TABLE download_events ADD COLUMN IF NOT EXISTS aggregate_key TEXT`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_visits_aggregate_key ON visits(aggregate_key)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_dlevents_aggregate_key ON download_events(aggregate_key)`,
		} {
			if _, err := DB.Exec(query); err != nil {
				return fmt.Errorf("补充 PostgreSQL 统计计数列失败: %w, query: %s", err, query)
			}
		}
		startTime := time.Now().UTC().Format("2006-01-02 15:04:05")
		if _, err := DB.Exec(`INSERT INTO system_info ("key", value) VALUES ($1, $2) ON CONFLICT ("key") DO NOTHING`, "start_time", startTime); err != nil {
			return fmt.Errorf("记录系统启动时间失败: %w", err)
		}
		if _, err := DB.Exec(`INSERT INTO system_info ("key", value) VALUES ($1, $2) ON CONFLICT ("key") DO UPDATE SET value = EXCLUDED.value`, "schema_version", fmt.Sprintf("%d", CurrentSchemaVersion)); err != nil {
			return fmt.Errorf("记录 PostgreSQL schema 版本失败: %w", err)
		}
		return nil
	}
	var queries []string
	if isMySQL {
		queries = []string{
			`CREATE TABLE IF NOT EXISTS visits (
                id INT AUTO_INCREMENT PRIMARY KEY,
                ip VARCHAR(255),
                path TEXT,
                user_agent TEXT,
                referer TEXT,
                country VARCHAR(255),
                region VARCHAR(255),
                city VARCHAR(255),
                visit_count BIGINT NOT NULL DEFAULT 1,
                aggregate_key VARCHAR(64),
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS downloads (
                id INT AUTO_INCREMENT PRIMARY KEY,
                file_name VARCHAR(255),
                launcher VARCHAR(255),
                version VARCHAR(255),
                ip VARCHAR(255),
                country VARCHAR(255),
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS ip_blacklist (
                ip VARCHAR(255) PRIMARY KEY,
                reason TEXT,
                source VARCHAR(50) DEFAULT 'manual',
                ban_type VARCHAR(50) DEFAULT 'manual',
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS ip_daily_traffic (
                ip VARCHAR(255),
                date VARCHAR(20),
                bytes_downloaded BIGINT DEFAULT 0,
                PRIMARY KEY (ip, date)
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE INDEX idx_ip_daily_traffic_date ON ip_daily_traffic(date)`,
			`CREATE TABLE IF NOT EXISTS daily_traffic (
                date VARCHAR(20) PRIMARY KEY,
                bytes_downloaded BIGINT DEFAULT 0
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS daily_completed_traffic (
                date VARCHAR(20) PRIMARY KEY,
                bytes_downloaded BIGINT DEFAULT 0
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS download_authorizations (
                authorization_id VARCHAR(64) PRIMARY KEY,
                token_hash VARCHAR(64) NOT NULL,
                file_path TEXT NOT NULL,
                return_url TEXT,
                source VARCHAR(32),
                flow VARCHAR(32),
                client_ip VARCHAR(64),
                source_kind VARCHAR(16),
                status VARCHAR(16) NOT NULL DEFAULT 'issued',
                expires_at DATETIME NOT NULL,
                max_bytes BIGINT,
                range_limit INT,
                request_id VARCHAR(64),
                first_transfer_at DATETIME,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                consumed_at DATETIME,
                UNIQUE KEY uq_dlauthz_token_hash (token_hash)
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS download_events (
                id INT AUTO_INCREMENT PRIMARY KEY,
                authorization_id VARCHAR(64),
                file_path TEXT,
                file_name VARCHAR(255),
                launcher VARCHAR(255),
                version VARCHAR(255),
                client_ip VARCHAR(64),
                country VARCHAR(255),
                bytes_served BIGINT DEFAULT 0,
                completed INT DEFAULT 0,
                event_count BIGINT NOT NULL DEFAULT 1,
                aggregate_key VARCHAR(64),
                status_code INT,
                date VARCHAR(20),
                source VARCHAR(32),
                source_id BIGINT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                UNIQUE KEY uq_dlevents_source_id (source, source_id)
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS system_info (
                ` + "`key`" + ` VARCHAR(255) PRIMARY KEY,
                value TEXT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS stats_snapshot (
                id INT PRIMARY KEY,
                data LONGTEXT,
                updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE INDEX idx_visits_created_at ON visits(created_at)`,
			`CREATE INDEX idx_visits_country ON visits(country)`,
			`CREATE INDEX idx_downloads_created_at ON downloads(created_at)`,
			`CREATE INDEX idx_downloads_file_name ON downloads(file_name)`,
			`CREATE INDEX idx_downloads_launcher_version ON downloads(launcher, version)`,
			`CREATE INDEX idx_dlauthz_status_expires ON download_authorizations(status, expires_at)`,
			`CREATE INDEX idx_dlevents_ip_date ON download_events(client_ip, date)`,
			`CREATE INDEX idx_dlevents_date ON download_events(date)`,
			`CREATE INDEX idx_dlevents_launcher ON download_events(launcher)`,
		}
	} else {
		queries = []string{
			`CREATE TABLE IF NOT EXISTS visits (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                ip TEXT,
                path TEXT,
                user_agent TEXT,
                referer TEXT,
                country TEXT,
                region TEXT,
                city TEXT,
                visit_count INTEGER NOT NULL DEFAULT 1,
                aggregate_key TEXT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            )`,
			`CREATE TABLE IF NOT EXISTS downloads (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                file_name TEXT,
                launcher TEXT,
                version TEXT,
                ip TEXT,
                country TEXT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            )`,
			`CREATE TABLE IF NOT EXISTS ip_blacklist (
                ip TEXT PRIMARY KEY,
                reason TEXT,
                source TEXT DEFAULT 'manual',
                ban_type TEXT DEFAULT 'manual',
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            )`,
			`CREATE TABLE IF NOT EXISTS ip_daily_traffic (
                ip TEXT,
                date TEXT,
                bytes_downloaded INTEGER DEFAULT 0,
                PRIMARY KEY (ip, date)
            )`,
			`CREATE INDEX IF NOT EXISTS idx_ip_daily_traffic_date ON ip_daily_traffic(date)`,
			`CREATE TABLE IF NOT EXISTS daily_traffic (
                date TEXT PRIMARY KEY,
                bytes_downloaded INTEGER DEFAULT 0
            )`,
			`CREATE TABLE IF NOT EXISTS daily_completed_traffic (
                date TEXT PRIMARY KEY,
                bytes_downloaded INTEGER DEFAULT 0
            )`,
			`CREATE TABLE IF NOT EXISTS download_authorizations (
                authorization_id TEXT PRIMARY KEY,
                token_hash TEXT NOT NULL UNIQUE,
                file_path TEXT NOT NULL,
                return_url TEXT,
                source TEXT,
                flow TEXT,
                client_ip TEXT,
                source_kind TEXT,
                status TEXT NOT NULL DEFAULT 'issued',
                expires_at DATETIME NOT NULL,
                max_bytes INTEGER,
                range_limit INTEGER,
                request_id TEXT,
                first_transfer_at DATETIME,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                consumed_at DATETIME
            )`,
			`CREATE TABLE IF NOT EXISTS download_events (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                authorization_id TEXT,
                file_path TEXT,
                file_name TEXT,
                launcher TEXT,
                version TEXT,
                client_ip TEXT,
                country TEXT,
                bytes_served INTEGER DEFAULT 0,
                completed INTEGER DEFAULT 0,
                event_count INTEGER NOT NULL DEFAULT 1,
                aggregate_key TEXT,
                status_code INTEGER,
                date TEXT,
                source TEXT,
                source_id INTEGER,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                UNIQUE(source, source_id)
            )`,
			`CREATE TABLE IF NOT EXISTS system_info (
                key TEXT PRIMARY KEY,
                value TEXT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            )`,
			`CREATE TABLE IF NOT EXISTS stats_snapshot (
                id INTEGER PRIMARY KEY,
                data TEXT,
                updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
            )`,
			`CREATE INDEX IF NOT EXISTS idx_visits_created_at ON visits(created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_visits_country ON visits(country)`,
			`CREATE INDEX IF NOT EXISTS idx_downloads_created_at ON downloads(created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_downloads_file_name ON downloads(file_name)`,
			`CREATE INDEX IF NOT EXISTS idx_downloads_launcher_version ON downloads(launcher, version)`,
			`CREATE INDEX IF NOT EXISTS idx_dlauthz_status_expires ON download_authorizations(status, expires_at)`,
			`CREATE INDEX IF NOT EXISTS idx_dlevents_ip_date ON download_events(client_ip, date)`,
			`CREATE INDEX IF NOT EXISTS idx_dlevents_date ON download_events(date)`,
			`CREATE INDEX IF NOT EXISTS idx_dlevents_launcher ON download_events(launcher)`,
		}
	}

	for _, query := range queries {
		if _, err := DB.Exec(query); err != nil {
			// MySQL 中创建索引如果已存在会报错，而 SQLite 有 IF NOT EXISTS。
			// 这里仅忽略“索引已存在”相关的错误 (Error 1061: Duplicate key name)。
			if isMySQL && strings.Contains(strings.ToUpper(query), "CREATE INDEX") {
				errMsg := err.Error()
				if strings.Contains(errMsg, "Duplicate key") || strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "1061") {
					continue
				}
			}
			return fmt.Errorf("创建表/索引失败: %w, query: %s", err, query)
		}
	}

	// 记录系统首次启动时间。
	// 用 Go 侧 UTC 时间而不是数据库函数（MySQL NOW() 为服务器本地时区、
	// SQLite datetime('now') 为 UTC），保证写入格式/时区与 stats 模块读取解析
	// （按 UTC 解析 "2006-01-02 15:04:05"）始终一致，避免运行天数计算偏差。
	startTime := time.Now().UTC().Format("2006-01-02 15:04:05")
	if isMySQL {
		if _, err := DB.Exec("INSERT IGNORE INTO system_info (`key`, value) VALUES (?, ?)", "start_time", startTime); err != nil {
			return fmt.Errorf("记录系统启动时间失败: %w", err)
		}
	} else {
		if _, err := DB.Exec("INSERT OR IGNORE INTO system_info (key, value) VALUES (?, ?)", "start_time", startTime); err != nil {
			return fmt.Errorf("记录系统启动时间失败: %w", err)
		}
	}

	// 应用版本化数据库迁移（schema_version 追踪在 system_info 表中）
	if err := runMigrations(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	return nil
}

func IsIPBlacklisted(ip string) bool {
	var count int
	err := DB.QueryRow(rebind("SELECT COUNT(*) FROM ip_blacklist WHERE ip = ?"), ip).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func GetIPBlacklistInfo(ip string) (bool, string, error) {
	var createdAtRaw interface{}
	err := DB.QueryRow(rebind("SELECT created_at FROM ip_blacklist WHERE ip = ?"), ip).Scan(&createdAtRaw)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, formatTime(createdAtRaw), nil
}

func AddIPToBlacklist(ip, reason string) error {
	var query string
	if isMySQL {
		query = "INSERT INTO ip_blacklist (ip, reason) VALUES (?, ?) ON DUPLICATE KEY UPDATE reason = VALUES(reason)"
	} else if isPostgres {
		query = `INSERT INTO ip_blacklist (ip, reason) VALUES (?, ?) ON CONFLICT (ip) DO UPDATE SET reason = EXCLUDED.reason`
	} else {
		query = "INSERT OR REPLACE INTO ip_blacklist (ip, reason) VALUES (?, ?)"
	}
	_, err := DB.Exec(rebind(query), ip, reason)
	return err
}

func RemoveIPFromBlacklist(ip string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(rebind("DELETE FROM ip_blacklist WHERE ip = ?"), ip); err != nil {
		return err
	}

	if _, err := tx.Exec(rebind("DELETE FROM ip_daily_traffic WHERE ip = ?"), ip); err != nil {
		return err
	}

	return tx.Commit()
}

func GetIPBlacklist() ([]map[string]string, error) {
	rows, err := DB.Query("SELECT ip, reason, source, ban_type, created_at FROM ip_blacklist ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []map[string]string{}
	for rows.Next() {
		var ip, reason, source, banType string
		var createdAtRaw interface{}
		if err := rows.Scan(&ip, &reason, &source, &banType, &createdAtRaw); err != nil {
			return nil, err
		}

		createdAt := formatTime(createdAtRaw)

		list = append(list, map[string]string{
			"ip":         ip,
			"reason":     reason,
			"source":     source,
			"ban_type":   banType,
			"created_at": createdAt,
		})
	}
	return list, nil
}

func GetLocalIPBlacklist() ([]map[string]string, error) {
	rows, err := DB.Query("SELECT ip, reason, source, ban_type, created_at FROM ip_blacklist WHERE source != 'external' ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []map[string]string{}
	for rows.Next() {
		var ip, reason, source, banType string
		var createdAtRaw interface{}
		if err := rows.Scan(&ip, &reason, &source, &banType, &createdAtRaw); err != nil {
			return nil, err
		}

		createdAt := formatTime(createdAtRaw)

		list = append(list, map[string]string{
			"ip":         ip,
			"reason":     reason,
			"source":     source,
			"ban_type":   banType,
			"created_at": createdAt,
		})
	}
	return list, nil
}

func formatTime(raw interface{}) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	case []byte:
		return string(v)
	case string:
		return v
	}
	return fmt.Sprintf("%v", raw)
}

func AddIPToBlacklistWithSource(ip, reason, source, banType string) error {
	var query string
	if isMySQL {
		query = "INSERT INTO ip_blacklist (ip, reason, source, ban_type) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE reason = VALUES(reason), source = VALUES(source), ban_type = VALUES(ban_type)"
	} else if isPostgres {
		query = `INSERT INTO ip_blacklist (ip, reason, source, ban_type) VALUES (?, ?, ?, ?) ON CONFLICT (ip) DO UPDATE SET reason = EXCLUDED.reason, source = EXCLUDED.source, ban_type = EXCLUDED.ban_type`
	} else {
		query = "INSERT OR REPLACE INTO ip_blacklist (ip, reason, source, ban_type) VALUES (?, ?, ?, ?)"
	}
	_, err := DB.Exec(rebind(query), ip, reason, source, banType)
	return err
}

func RecordTraffic(ip string, bytes int64) error {
	date := time.Now().Format("2006-01-02")
	var query string
	if isMySQL {
		query = `
			INSERT INTO ip_daily_traffic (ip, date, bytes_downloaded) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE bytes_downloaded = bytes_downloaded + VALUES(bytes_downloaded)`
	} else if isPostgres {
		query = `
			INSERT INTO ip_daily_traffic (ip, date, bytes_downloaded) VALUES (?, ?, ?)
			ON CONFLICT (ip, date) DO UPDATE SET bytes_downloaded = ip_daily_traffic.bytes_downloaded + EXCLUDED.bytes_downloaded`
	} else {
		query = `
			INSERT INTO ip_daily_traffic (ip, date, bytes_downloaded) VALUES (?, ?, ?)
			ON CONFLICT(ip, date) DO UPDATE SET bytes_downloaded = bytes_downloaded + ?`
	}

	var err error
	if isMySQL {
		_, err = DB.Exec(rebind(query), ip, date, bytes)
	} else {
		_, err = DB.Exec(rebind(query), ip, date, bytes, bytes)
	}
	if err != nil {
		return err
	}

	// 同时更新无 IP 的每日聚合表（用于历史流量统计，IP 级数据 24 小时后清理）
	return updateDailyTrafficAggregate("daily_traffic", date, bytes)
}

// RecordCompletedTraffic 记录一次完整传输的流量到无 IP 聚合表
// daily_completed_traffic（展示口径，无 IP 维度）。
// 与 RecordTraffic 的 served 口径（含客户端中止的部分传输，用于防刷墙）相互独立。
func RecordCompletedTraffic(bytes int64) error {
	date := time.Now().Format("2006-01-02")
	return updateDailyTrafficAggregate("daily_completed_traffic", date, bytes)
}

// updateDailyTrafficAggregate 更新无 IP 的每日流量聚合表。
func updateDailyTrafficAggregate(table, date string, bytes int64) error {
	var query string
	if isMySQL {
		query = fmt.Sprintf(`
			INSERT INTO %s (date, bytes_downloaded) VALUES (?, ?)
			ON DUPLICATE KEY UPDATE bytes_downloaded = bytes_downloaded + VALUES(bytes_downloaded)`, table)
	} else if isPostgres {
		query = fmt.Sprintf(`
			INSERT INTO %s (date, bytes_downloaded) VALUES (?, ?)
			ON CONFLICT (date) DO UPDATE SET bytes_downloaded = %s.bytes_downloaded + EXCLUDED.bytes_downloaded`, table, table)
	} else {
		query = fmt.Sprintf(`
			INSERT INTO %s (date, bytes_downloaded) VALUES (?, ?)
			ON CONFLICT(date) DO UPDATE SET bytes_downloaded = bytes_downloaded + ?`, table)
	}

	if isMySQL {
		_, err := DB.Exec(rebind(query), date, bytes)
		return err
	}
	_, err := DB.Exec(rebind(query), date, bytes, bytes)
	return err
}

func GetDailyTraffic(ip string) (int64, error) {
	date := time.Now().Format("2006-01-02")
	return GetTrafficOnDate(ip, date)
}

func GetTrafficOnDate(ip string, date string) (int64, error) {
	var bytes int64
	err := DB.QueryRow(rebind("SELECT bytes_downloaded FROM ip_daily_traffic WHERE ip = ? AND date = ?"), ip, date).Scan(&bytes)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return bytes, err
}

// DailyTrafficStat 每日流量统计
type DailyTrafficStat struct {
	Date  string `json:"date"`
	Bytes int64  `json:"bytes"`
}

// GetTotalTraffic 返回普通下载总流量（字节），从无 IP 聚合表查询
func GetTotalTraffic() (int64, error) {
	var bytes int64
	err := DB.QueryRow("SELECT COALESCE(SUM(bytes_downloaded), 0) FROM daily_traffic").Scan(&bytes)
	return bytes, err
}

// GetDailyTrafficStats 返回最近 N 天每日普通下载流量，从无 IP 聚合表查询
func GetDailyTrafficStats(days int) ([]DailyTrafficStat, error) {
	threshold := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := DB.Query(rebind("SELECT date, COALESCE(bytes_downloaded, 0) FROM daily_traffic WHERE date >= ? ORDER BY date"), threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []DailyTrafficStat
	for rows.Next() {
		var stat DailyTrafficStat
		if err := rows.Scan(&stat.Date, &stat.Bytes); err != nil {
			return nil, err
		}
		result = append(result, stat)
	}
	return result, rows.Err()
}

// GetTotalCompletedTraffic 返回完整传输总流量（字节），从 daily_completed_traffic 聚合表查询
func GetTotalCompletedTraffic() (int64, error) {
	var bytes int64
	err := DB.QueryRow("SELECT COALESCE(SUM(bytes_downloaded), 0) FROM daily_completed_traffic").Scan(&bytes)
	return bytes, err
}

// GetDailyCompletedTrafficStats 返回最近 N 天每日完整传输流量，从 daily_completed_traffic 聚合表查询
func GetDailyCompletedTrafficStats(days int) ([]DailyTrafficStat, error) {
	threshold := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := DB.Query(rebind("SELECT date, COALESCE(bytes_downloaded, 0) FROM daily_completed_traffic WHERE date >= ? ORDER BY date"), threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []DailyTrafficStat
	for rows.Next() {
		var stat DailyTrafficStat
		if err := rows.Scan(&stat.Date, &stat.Bytes); err != nil {
			return nil, err
		}
		result = append(result, stat)
	}
	return result, rows.Err()
}

// CleanupOldTrafficRecords 清理 ip_daily_traffic 中
// 超过 24 小时的记录（即 date < 今天），IP 级数据仅保留当日用于防刷墙。
func CleanupOldTrafficRecords() (int64, error) {
	today := time.Now().Format("2006-01-02")
	res, err := DB.Exec(rebind("DELETE FROM ip_daily_traffic WHERE date < ?"), today)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
func AddExternalBlacklist(ips []string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var query string
	if isMySQL {
		query = "INSERT IGNORE INTO ip_blacklist (ip, reason, source, ban_type) VALUES (?, ?, 'external', 'manual')"
	} else if isPostgres {
		query = "INSERT INTO ip_blacklist (ip, reason, source, ban_type) VALUES (?, ?, 'external', 'manual') ON CONFLICT (ip) DO NOTHING"
	} else {
		query = "INSERT OR IGNORE INTO ip_blacklist (ip, reason, source, ban_type) VALUES (?, ?, 'external', 'manual')"
	}

	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" || strings.HasPrefix(ip, "#") {
			continue
		}
		if _, err := stmt.Exec(ip, "外部黑名单"); err != nil {
			log.Printf("添加外部黑名单IP失败: %s, %v", ip, err)
		}
	}

	return tx.Commit()
}
