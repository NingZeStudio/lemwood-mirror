package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB(storagePath string) error {
	dbPath := filepath.Join(storagePath, "stats.db")

	// 确保目录存在
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	// 限制最大连接数为 1，避免 SQLite 锁定
	DB.SetMaxOpenConns(1)

	// 性能优化：启用 WAL 模式
	// WAL 模式允许并发读写，显著提高性能
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

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	return createTables()
}

func createTables() error {
	queries := []string{
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

	for _, query := range queries {
		if _, err := DB.Exec(query); err != nil {
			return fmt.Errorf("创建表失败: %w, query: %s", err, query)
		}
	}

	// 记录系统首次启动时间
	if _, err := DB.Exec("INSERT OR IGNORE INTO system_info (key, value) VALUES (?, datetime('now'))", "start_time"); err != nil {
		return fmt.Errorf("记录系统启动时间失败: %w", err)
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

func AddIPToBlacklist(ip, reason string) error {
	_, err := DB.Exec("INSERT OR REPLACE INTO ip_blacklist (ip, reason) VALUES (?, ?)", ip, reason)
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

func AddIPToBlacklistWithSource(ip, reason, source, banType string) error {
	_, err := DB.Exec("INSERT OR REPLACE INTO ip_blacklist (ip, reason, source, ban_type) VALUES (?, ?, ?, ?)", ip, reason, source, banType)
	return err
}

func RecordTraffic(ip string, bytes int64) error {
	date := time.Now().Format("2006-01-02")
	_, err := DB.Exec(`
		INSERT INTO ip_daily_traffic (ip, date, bytes_downloaded) VALUES (?, ?, ?)
		ON CONFLICT(ip, date) DO UPDATE SET bytes_downloaded = bytes_downloaded + ?`,
		ip, date, bytes, bytes)
	return err
}

func GetDailyTraffic(ip string) (int64, error) {
	date := time.Now().Format("2006-01-02")
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

	stmt, err := tx.Prepare("INSERT OR IGNORE INTO ip_blacklist (ip, reason, source, ban_type) VALUES (?, ?, 'external', 'manual')")
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
