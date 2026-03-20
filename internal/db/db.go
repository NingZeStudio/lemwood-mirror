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
	_ "modernc.org/sqlite"
)

var (
	DB      *sql.DB
	isMySQL bool
)

func InitDB(storagePath string, cfg *config.Config) error {
	dbPath := filepath.Join(storagePath, "stats.db")

	// 检查是否启用 MySQL
	if cfg.MySQLHost != "" {
		// 验证 MySQL 配置完整性
		if cfg.MySQLUser == "" || cfg.MySQLDatabase == "" || cfg.MySQLPort <= 0 {
			return fmt.Errorf("MySQL 配置不完整: 必须提供 host, user, database 和有效的 port")
		}

		isMySQL = true
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

		// MySQL 连接池设置
		DB.SetMaxOpenConns(10)
		DB.SetMaxIdleConns(5)
		DB.SetConnMaxLifetime(time.Hour)

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
		// 确保目录存在
		if err := os.MkdirAll(storagePath, 0755); err != nil {
			return fmt.Errorf("创建数据库目录失败: %w", err)
		}

		var err error
		DB, err = sql.Open("sqlite", dbPath)
		if err != nil {
			return fmt.Errorf("打开 SQLite 失败: %w", err)
		}

		// 限制最大连接数为 1，避免 SQLite 锁定
		DB.SetMaxOpenConns(1)

		// 性能优化：启用 WAL 模式
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

	return createTables()
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
	tables := []string{"visits", "downloads", "ip_blacklist", "ip_daily_traffic", "system_info"}
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
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	placeholders := make([]string, len(cols))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName, strings.Join(cols, ","), strings.Join(placeholders, ","))

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
			`CREATE TABLE IF NOT EXISTS system_info (
                ` + "`key`" + ` VARCHAR(255) PRIMARY KEY,
                value TEXT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE INDEX idx_visits_created_at ON visits(created_at)`,
			`CREATE INDEX idx_downloads_created_at ON downloads(created_at)`,
			`CREATE INDEX idx_downloads_file_name ON downloads(file_name)`,
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
			`CREATE TABLE IF NOT EXISTS system_info (
                key TEXT PRIMARY KEY,
                value TEXT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            )`,
			`CREATE INDEX IF NOT EXISTS idx_visits_created_at ON visits(created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_downloads_created_at ON downloads(created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_downloads_file_name ON downloads(file_name)`,
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

	// 记录系统首次启动时间
	if isMySQL {
		if _, err := DB.Exec("INSERT IGNORE INTO system_info (`key`, value) VALUES (?, NOW())", "start_time"); err != nil {
			return fmt.Errorf("记录系统启动时间失败: %w", err)
		}
	} else {
		if _, err := DB.Exec("INSERT OR IGNORE INTO system_info (key, value) VALUES (?, datetime('now'))", "start_time"); err != nil {
			return fmt.Errorf("记录系统启动时间失败: %w", err)
		}
	}

	// 数据库迁移：为旧表添加新列
	if err := migrateTables(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	return nil
}

func migrateTables() error {
	if isMySQL {
		// MySQL 新建表时已经包含了新列
		return nil
	}
	// 检查 ip_blacklist 表是否有 source 列
	var hasSourceColumn bool
	rows, err := DB.Query("PRAGMA table_info(ip_blacklist)")
	if err != nil {
		return err
	}
	defer rows.Close()

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

	// 如果没有 source 列，添加新列
	if !hasSourceColumn {
		log.Println("数据库迁移: 为 ip_blacklist 表添加 source 和 ban_type 列")
		alterQueries := []string{
			"ALTER TABLE ip_blacklist ADD COLUMN source TEXT DEFAULT 'manual'",
			"ALTER TABLE ip_blacklist ADD COLUMN ban_type TEXT DEFAULT 'manual'",
		}
		for _, q := range alterQueries {
			if _, err := DB.Exec(q); err != nil {
				return fmt.Errorf("添加列失败: %w, query: %s", err, q)
			}
		}
	}

	return nil
}

func IsIPBlacklisted(ip string) bool {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM ip_blacklist WHERE ip = ?", ip).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func GetIPBlacklistInfo(ip string) (bool, string, error) {
	var createdAt string
	err := DB.QueryRow("SELECT created_at FROM ip_blacklist WHERE ip = ?", ip).Scan(&createdAt)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, createdAt, nil
}

func AddIPToBlacklist(ip, reason string) error {
	var query string
	if isMySQL {
		query = "INSERT INTO ip_blacklist (ip, reason) VALUES (?, ?) ON DUPLICATE KEY UPDATE reason = VALUES(reason)"
	} else {
		query = "INSERT OR REPLACE INTO ip_blacklist (ip, reason) VALUES (?, ?)"
	}
	_, err := DB.Exec(query, ip, reason)
	return err
}

func RemoveIPFromBlacklist(ip string) error {
	_, err := DB.Exec("DELETE FROM ip_blacklist WHERE ip = ?", ip)
	return err
}

func GetIPBlacklist() ([]map[string]string, error) {
	rows, err := DB.Query("SELECT ip, reason, source, ban_type, created_at FROM ip_blacklist ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []map[string]string{}
	for rows.Next() {
		var ip, reason, source, banType, createdAt string
		if err := rows.Scan(&ip, &reason, &source, &banType, &createdAt); err != nil {
			return nil, err
		}
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
		var ip, reason, source, banType, createdAt string
		if err := rows.Scan(&ip, &reason, &source, &banType, &createdAt); err != nil {
			return nil, err
		}
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

func AddIPToBlacklistWithSource(ip, reason, source, banType string) error {
	var query string
	if isMySQL {
		query = "INSERT INTO ip_blacklist (ip, reason, source, ban_type) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE reason = VALUES(reason), source = VALUES(source), ban_type = VALUES(ban_type)"
	} else {
		query = "INSERT OR REPLACE INTO ip_blacklist (ip, reason, source, ban_type) VALUES (?, ?, ?, ?)"
	}
	_, err := DB.Exec(query, ip, reason, source, banType)
	return err
}

func RecordTraffic(ip string, bytes int64) error {
	date := time.Now().Format("2006-01-02")
	var query string
	if isMySQL {
		query = `
			INSERT INTO ip_daily_traffic (ip, date, bytes_downloaded) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE bytes_downloaded = bytes_downloaded + VALUES(bytes_downloaded)`
	} else {
		query = `
			INSERT INTO ip_daily_traffic (ip, date, bytes_downloaded) VALUES (?, ?, ?)
			ON CONFLICT(ip, date) DO UPDATE SET bytes_downloaded = bytes_downloaded + ?`
	}
	
	var err error
	if isMySQL {
		_, err = DB.Exec(query, ip, date, bytes)
	} else {
		_, err = DB.Exec(query, ip, date, bytes, bytes)
	}
	return err
}

func GetDailyTraffic(ip string) (int64, error) {
	date := time.Now().Format("2006-01-02")
	return GetTrafficOnDate(ip, date)
}

func GetTrafficOnDate(ip string, date string) (int64, error) {
	var bytes int64
	err := DB.QueryRow("SELECT bytes_downloaded FROM ip_daily_traffic WHERE ip = ? AND date = ?", ip, date).Scan(&bytes)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return bytes, err
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

