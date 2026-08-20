package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// CurrentSchemaVersion 是当前代码所期望的最新 schema 版本。
// 每次新增 Migration 时递增此常量。
const CurrentSchemaVersion = 6

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
	{
		Version:     3,
		Description: "新建完整传输流量聚合表 daily_completed_traffic 并回填历史数据",
		Up:          migrateV3CompletedTraffic,
	},
	{
		Version:     4,
		Description: "新建下载授权与下载事件状态表，并从 downloads 回填历史事件",
		Up:          migrateV4DownloadStatusTables,
	},
	{
		Version:     5,
		Description: "为统计聚合行增加访问/下载计数列",
		Up:          migrateV5StatsCounts,
	},
	{
		Version:     6,
		Description: "增加运行时统计聚合键，后续访问和下载按键累加",
		Up:          migrateV6AggregateKeys,
	},
}

// getSchemaVersion 从 system_info 表读取 schema_version，缺失视为 0。
func getSchemaVersion() (int, error) {
	var version int
	key := "`key`"
	if isPostgres {
		key = `"key"`
	}
	err := DB.QueryRow(rebind("SELECT value FROM system_info WHERE "+key+" = ?"), "schema_version").Scan(&version)
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
	} else if isPostgres {
		query = `INSERT INTO system_info ("key", value) VALUES (?, ?) ON CONFLICT ("key") DO UPDATE SET value = EXCLUDED.value`
	} else {
		query = "INSERT OR REPLACE INTO system_info (key, value) VALUES (?, ?)"
	}
	versionValue := any(version)
	if isPostgres {
		versionValue = fmt.Sprintf("%d", version)
	}
	if _, err := DB.Exec(rebind(query), "schema_version", versionValue); err != nil {
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

func migrateV5StatsCounts(d *sql.DB) error {
	for table, column := range map[string]string{
		"visits":          "visit_count",
		"download_events": "event_count",
	} {
		if !tableExists(d, table) {
			continue
		}
		exists, err := columnExists(d, table, column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		typeName := "INTEGER"
		if isMySQL || isPostgres {
			typeName = "BIGINT"
		}
		if _, err := d.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s NOT NULL DEFAULT 1", table, column, typeName)); err != nil {
			return fmt.Errorf("添加 %s.%s 失败: %w", table, column, err)
		}
	}
	return nil
}

func migrateV6AggregateKeys(d *sql.DB) error {
	for table, column := range map[string]string{
		"visits":          "aggregate_key",
		"download_events": "aggregate_key",
	} {
		if !tableExists(d, table) {
			continue
		}
		exists, err := columnExists(d, table, column)
		if err != nil {
			return err
		}
		if !exists {
			typeName := "TEXT"
			if isMySQL {
				typeName = "VARCHAR(64)"
			}
			if _, err := d.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, typeName)); err != nil {
				return fmt.Errorf("添加 %s.%s 失败: %w", table, column, err)
			}
		}
	}

	indexQueries := make([]string, 0, 2)
	if tableExists(d, "visits") {
		indexQueries = append(indexQueries, "CREATE UNIQUE INDEX uq_visits_aggregate_key ON visits(aggregate_key)")
	}
	if tableExists(d, "download_events") {
		indexQueries = append(indexQueries, "CREATE UNIQUE INDEX uq_dlevents_aggregate_key ON download_events(aggregate_key)")
	}
	if !isMySQL && !isPostgres {
		for i := range indexQueries {
			indexQueries[i] = strings.Replace(indexQueries[i], "CREATE UNIQUE INDEX ", "CREATE UNIQUE INDEX IF NOT EXISTS ", 1)
		}
	} else if isPostgres {
		for i := range indexQueries {
			indexQueries[i] = strings.Replace(indexQueries[i], "CREATE UNIQUE INDEX ", "CREATE UNIQUE INDEX IF NOT EXISTS ", 1)
		}
	}
	for _, query := range indexQueries {
		if _, err := d.Exec(query); err != nil {
			if isMySQL && isDuplicateIndexErr(err) {
				continue
			}
			return fmt.Errorf("创建聚合键索引失败: %w, query: %s", err, query)
		}
	}
	return nil
}

func tableExists(d *sql.DB, table string) bool {
	if isMySQL {
		var n int
		return d.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?", table).Scan(&n) == nil && n > 0
	}
	if isPostgres {
		var n int
		return d.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name=$1", table).Scan(&n) == nil && n > 0
	}
	var n int
	return d.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&n) == nil && n > 0
}

func columnExists(d *sql.DB, table, column string) (bool, error) {
	if isMySQL {
		var n int
		err := d.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?`, table, column).Scan(&n)
		return n > 0, err
	}
	if isPostgres {
		var n int
		err := d.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1 AND column_name=$2`, table, column).Scan(&n)
		return n > 0, err
	}
	rows, err := d.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typeName string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typeName, &notnull, &defaultValue, &pk); err == nil && name == column {
			return true, nil
		}
	}
	return false, rows.Err()
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

// migrateV3CompletedTraffic 创建完整传输流量聚合表 daily_completed_traffic，
// 并从 daily_traffic 回填历史数据。
// 历史数据无法区分完整传输与中止传输，回填以 served 口径作为完整传输的初始近似，
// 之后的增量由下载处理器按完整传输判定精确写入。
// 幂等：CREATE TABLE IF NOT EXISTS + INSERT IGNORE / INSERT OR IGNORE，
// 重复执行不产生重复行或错误。
func migrateV3CompletedTraffic(d *sql.DB) error {
	var createQuery, insertPrefix string
	if isMySQL {
		createQuery = `CREATE TABLE IF NOT EXISTS daily_completed_traffic (
			date VARCHAR(20) PRIMARY KEY,
			bytes_downloaded BIGINT DEFAULT 0
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
		insertPrefix = "INSERT IGNORE INTO"
	} else {
		createQuery = `CREATE TABLE IF NOT EXISTS daily_completed_traffic (
			date TEXT PRIMARY KEY,
			bytes_downloaded INTEGER DEFAULT 0
		)`
		insertPrefix = "INSERT OR IGNORE INTO"
	}

	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(createQuery); err != nil {
		return fmt.Errorf("创建 daily_completed_traffic 失败: %w", err)
	}

	if _, err := tx.Exec(insertPrefix + ` daily_completed_traffic (date, bytes_downloaded)
		SELECT date, bytes_downloaded FROM daily_traffic`); err != nil {
		return fmt.Errorf("回填 daily_completed_traffic 失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交回填事务失败: %w", err)
	}

	log.Println("[数据库迁移] v3: daily_completed_traffic 建表与历史回填完成")
	return nil
}

// migrateV4DownloadStatusTables 建立下载授权（download_authorizations）和下载事件
// （download_events）状态表，并从历史 downloads 表回填事件行用于状态表口径的统计。
//
// 两张表在 createTables() 中随启动幂等创建（含索引），此处用 CREATE TABLE IF NOT EXISTS
// 兜底，随后执行一次幂等回填：downloads 每行生成一条 download_events，bytes_served=0
// （历史下载无字节口径，无法重建），completed=0；历史字节总量仍由冻结的 daily_traffic/
// daily_completed_traffic 基线承载。回填借助 (source, source_id) 唯一索引去重，重复执行
// 不产生重复行。
func migrateV4DownloadStatusTables(d *sql.DB) error {
	var createAuthz, createEvents, backfillInsert, maxIDSQL string
	if isMySQL {
		createAuthz = `CREATE TABLE IF NOT EXISTS download_authorizations (
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
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
		createEvents = `CREATE TABLE IF NOT EXISTS download_events (
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
			status_code INT,
			date VARCHAR(20),
			source VARCHAR(32),
			source_id BIGINT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uq_dlevents_source_id (source, source_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
		backfillInsert = `INSERT IGNORE INTO download_events
			(authorization_id, file_path, file_name, launcher, version, client_ip, country, bytes_served, completed, status_code, date, source, source_id, created_at)
			SELECT '', file_name, file_name, launcher, version, '', country, 0, 0, 200, DATE_FORMAT(created_at, '%Y-%m-%d'), 'downloads_import', id, created_at FROM downloads WHERE id > ? ORDER BY id LIMIT ?`
		maxIDSQL = `SELECT COALESCE(MAX(id), 0) FROM (SELECT id FROM downloads WHERE id > ? ORDER BY id LIMIT ?) t`
	} else {
		createAuthz = `CREATE TABLE IF NOT EXISTS download_authorizations (
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
		)`
		createEvents = `CREATE TABLE IF NOT EXISTS download_events (
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
			status_code INTEGER,
			date TEXT,
			source TEXT,
			source_id INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(source, source_id)
		)`
		backfillInsert = `INSERT OR IGNORE INTO download_events
			(authorization_id, file_path, file_name, launcher, version, client_ip, country, bytes_served, completed, status_code, date, source, source_id, created_at)
			SELECT '', file_name, file_name, launcher, version, '', country, 0, 0, 200, date(created_at), 'downloads_import', id, created_at FROM downloads WHERE id > ? ORDER BY id LIMIT ?`
		maxIDSQL = `SELECT COALESCE(MAX(id), 0) FROM (SELECT id FROM downloads WHERE id > ? ORDER BY id LIMIT ?) t`
	}

	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(createAuthz); err != nil {
		return fmt.Errorf("创建 download_authorizations 失败: %w", err)
	}
	if _, err := tx.Exec(createEvents); err != nil {
		return fmt.Errorf("创建 download_events 失败: %w", err)
	}

	// 索引：唯一索引保证回填去重；查询索引服务于防刷墙（按 IP+日）与统计（按日/启动器）。
	// MySQL 不支持 CREATE INDEX IF NOT EXISTS（报 1064 语法错误），故按方言分别构造：
	// MySQL 用裸 CREATE INDEX 并在下方忽略 1061 重复索引错误以保证幂等；
	// SQLite 用 CREATE INDEX IF NOT EXISTS 原生幂等。
	baseIndexStmts := []string{
		"idx_dlauthz_status_expires ON download_authorizations(status, expires_at)",
		"idx_dlevents_ip_date ON download_events(client_ip, date)",
		"idx_dlevents_date ON download_events(date)",
		"idx_dlevents_launcher ON download_events(launcher)",
	}
	for _, idx := range baseIndexStmts {
		var stmt string
		if isMySQL {
			stmt = "CREATE INDEX " + idx
		} else {
			stmt = "CREATE INDEX IF NOT EXISTS " + idx
		}
		if _, err := tx.Exec(stmt); err != nil {
			if isMySQL && isDuplicateIndexErr(err) {
				continue
			}
			return fmt.Errorf("创建索引失败: %w, query: %s", err, stmt)
		}
	}

	// 回填：downloads 每行生成一条 download_events，bytes_served=0、completed=0，
	// 历史字节总量仍由冻结的 daily_traffic/daily_completed_traffic 基线承载。
	// 大表不能一条 INSERT...SELECT 全量搬（MySQL 单次大查询会超过 readTimeout=30s 触发
	// i/o timeout 且连接失效），改为按主键 id 分批 keyset 分页，保证每条 SQL 都短小。
	// 幂等：回填以 (source, source_id) 唯一索引去重，重复执行不产生重复行。
	if !tableExistsTx(tx, "downloads") {
		log.Println("[数据库迁移] v4: downloads 表不存在，跳过事件回填")
	} else {
		const backfillBatch = 5000
		var lastID int64
		for {
			var maxID int64
			if err := tx.QueryRow(maxIDSQL, lastID, backfillBatch).Scan(&maxID); err != nil {
				return fmt.Errorf("查询回填批次上界失败: %w", err)
			}
			if maxID == 0 {
				break
			}
			if _, err := tx.Exec(backfillInsert, lastID, backfillBatch); err != nil {
				return fmt.Errorf("回填 download_events 失败 (id>%d): %w", lastID, err)
			}
			lastID = maxID
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交 v4 事务失败: %w", err)
	}

	log.Println("[数据库迁移] v4: download_authorizations/download_events 建表与历史回填完成")
	return nil
}

// isDuplicateIndexErr 判断是否为 MySQL "索引已存在" 错误（1061），可安全忽略以保证幂等。
func isDuplicateIndexErr(err error) bool {
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "DUPLICATE") || strings.Contains(msg, "1061") || strings.Contains(msg, "ALREADY EXISTS")
}

// tableExistsTx 在事务内判断表是否存在（SQLite 查 sqlite_master，MySQL 查 information_schema）。
func tableExistsTx(tx *sql.Tx, table string) bool {
	if isMySQL {
		var n int
		err := tx.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table).Scan(&n)
		return err == nil && n > 0
	}
	var name string
	err := tx.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
	return err == nil && name == table
}
