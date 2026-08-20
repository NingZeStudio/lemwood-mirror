package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// AuthzTimeFormat 是授权与事件时间字段的统一 UTC 文本格式（与 system_info.start_time 一致）。
// expires_at 用此格式存储，使得字符串字典序与时间顺序一致，可直接做 > / <= 比较。
// 导出供 download_authz 包在签发时以同一格式写入 expires_at。
const AuthzTimeFormat = "2006-01-02 15:04:05"

// DownloadAuthorization 表示一次下载授权的状态表记录。
// token_hash = SHA-256(opaque_token)；明文 token 只在签发响应中返回一次，不落库。
type DownloadAuthorization struct {
	AuthorizationID string
	TokenHash       string
	FilePath        string
	ReturnURL       string
	Source          string
	Flow            string
	ClientIP        string
	SourceKind      string // web | api
	Status          string // issued | consumed | expired
	ExpiresAt       string
	MaxBytes        int64
	RangeLimit      int
	RequestID       string
	FirstTransferAt string // 空串表示 NULL
	CreatedAt       string
	ConsumedAt      string // 空串表示 NULL
}

// DownloadEvent 表示一次实际下载的事件/流量行，是流量统计的状态表来源。
type DownloadEvent struct {
	ID              int64
	AuthorizationID string
	FilePath        string
	FileName        string
	Launcher        string
	Version         string
	ClientIP        string
	Country         string
	BytesServed     int64
	Completed       bool
	StatusCode      int
	Date            string
	CreatedAt       string
}

// DownloadRank 用于下载次数排行（按启动器聚合）。
type DownloadRank struct {
	Launcher string
	Count    int64
}

// EventDailyStat 是按日聚合的事件统计（served=真实发送字节，completed=完整传输字节）。
type EventDailyStat struct {
	Date      string
	Served    int64
	Completed int64
	Count     int64
}

func aggregateKey(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func VisitAggregateKey(date, country, region, city string) string {
	return aggregateKey(date, country, region, city)
}

func DownloadEventAggregateKey(e DownloadEvent) string {
	return aggregateKey(e.Date, e.ClientIP, e.FilePath, e.Launcher, e.Version,
		e.Country, fmt.Sprintf("%d", boolToInt(e.Completed)), fmt.Sprintf("%d", e.StatusCode))
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// CreateDownloadAuthorization 写入一条 issued 授权记录。created_at 用 Go 侧 UTC
// 时间显式写入，避免 MySQL NOW() 的服务器时区差异。
func CreateDownloadAuthorization(a DownloadAuthorization) error {
	_, err := DB.Exec(rebind(`INSERT INTO download_authorizations
		(authorization_id, token_hash, file_path, return_url, source, flow, client_ip, source_kind, status, expires_at, max_bytes, range_limit, request_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		a.AuthorizationID, a.TokenHash, a.FilePath, a.ReturnURL, a.Source, a.Flow,
		a.ClientIP, a.SourceKind, a.Status, a.ExpiresAt, a.MaxBytes, a.RangeLimit,
		a.RequestID, time.Now().UTC().Format(AuthzTimeFormat))
	if err != nil {
		return fmt.Errorf("insert download_authorization: %w", err)
	}
	return nil
}

// GetDownloadAuthorizationByTokenHash 按 token_hash 查询授权记录（任意状态）。
// 调用方负责在加载后自行判断 status 与 expires_at。
// DATETIME 列用 SQL 直接格式化为 UTC 文本（AuthzTimeFormat），避免 go-sql-driver 在
// parseTime=True&loc=Local 下按服务器本地时区解析再回传，导致 UTC 时间被偏移误判过期。
func GetDownloadAuthorizationByTokenHash(tokenHash string) (DownloadAuthorization, error) {
	var a DownloadAuthorization
	var returnURL, flow, clientIP, sourceKind, requestID, firstTransfer, consumedAt sql.NullString
	var maxBytes, rangeLimit sql.NullInt64
	// MySQL DATE_FORMAT / SQLite strftime 均返回 "2006-01-02 15:04:05" 纯文本。
	dt := `DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s')`
	if isPostgres {
		dt = `TO_CHAR(%s, 'YYYY-MM-DD HH24:MI:SS')`
	} else if !isMySQL {
		dt = `strftime('%%Y-%%m-%%d %%H:%%M:%%S', %s)`
	}
	err := DB.QueryRow(rebind(`SELECT authorization_id, token_hash, file_path, return_url, source, flow,
		client_ip, source_kind, status, `+fmt.Sprintf(dt, "expires_at")+`, max_bytes, range_limit, request_id,
		`+fmt.Sprintf(dt, "first_transfer_at")+`, `+fmt.Sprintf(dt, "created_at")+`, `+fmt.Sprintf(dt, "consumed_at")+`
		FROM download_authorizations WHERE token_hash = ?`), tokenHash).Scan(
		&a.AuthorizationID, &a.TokenHash, &a.FilePath, &returnURL, &a.Source, &flow,
		&clientIP, &sourceKind, &a.Status, &a.ExpiresAt, &maxBytes, &rangeLimit, &requestID,
		&firstTransfer, &a.CreatedAt, &consumedAt)
	if err != nil {
		return DownloadAuthorization{}, err
	}
	a.ReturnURL = returnURL.String
	a.Flow = flow.String
	a.ClientIP = clientIP.String
	a.SourceKind = sourceKind.String
	a.RequestID = requestID.String
	a.FirstTransferAt = firstTransfer.String
	a.ConsumedAt = consumedAt.String
	if maxBytes.Valid {
		a.MaxBytes = maxBytes.Int64
	}
	if rangeLimit.Valid {
		a.RangeLimit = int(rangeLimit.Int64)
	}
	return a, nil
}

// ConsumeDownloadAuthorization 原子地把一条 issued 且未过期的授权标记为 consumed。
// 返回更新后的行与 true；若未匹配（不存在/已消费/已过期）返回 false。
// expires_at 与 now 均为 UTC "2006-01-02 15:04:05" 文本，字典序比较等价于时间序。
func ConsumeDownloadAuthorization(tokenHash string) (DownloadAuthorization, bool, error) {
	now := time.Now().UTC().Format(AuthzTimeFormat)
	res, err := DB.Exec(rebind(`UPDATE download_authorizations
		SET status='consumed', consumed_at=?
		WHERE token_hash=? AND status='issued' AND expires_at > ?`),
		now, tokenHash, now)
	if err != nil {
		return DownloadAuthorization{}, false, fmt.Errorf("consume download_authorization: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return DownloadAuthorization{}, false, nil
	}
	a, err := GetDownloadAuthorizationByTokenHash(tokenHash)
	if err != nil {
		return DownloadAuthorization{}, false, fmt.Errorf("reload after consume: %w", err)
	}
	return a, true, nil
}

// CleanupExpiredAuthorizations 把已过期但仍为 issued 的授权标记为 expired。
// 供后台定期清理调用，避免过期令牌长期占据 issued 状态。
func CleanupExpiredAuthorizations() (int64, error) {
	now := time.Now().UTC().Format(AuthzTimeFormat)
	res, err := DB.Exec(rebind(`UPDATE download_authorizations SET status='expired'
		WHERE status='issued' AND expires_at <= ?`), now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// RecordDownloadEvent 写入一次下载的事件/流量行。date 缺省为当日（UTC）。
// 这是流量统计状态表的唯一写入点；防刷墙的 served 字节也由此行承载。
func RecordDownloadEvent(e DownloadEvent) error {
	now := time.Now().UTC()
	if e.Date == "" {
		e.Date = now.Format("2006-01-02")
	}
	completed := boolToInt(e.Completed)
	key := DownloadEventAggregateKey(e)
	query := `INSERT INTO download_events
		(authorization_id, file_path, file_name, launcher, version, client_ip, country,
		 bytes_served, completed, status_code, date, event_count, aggregate_key, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if isMySQL {
		query += ` ON DUPLICATE KEY UPDATE bytes_served=bytes_served+VALUES(bytes_served), event_count=event_count+VALUES(event_count)`
	} else if isPostgres {
		query += ` ON CONFLICT (aggregate_key) DO UPDATE SET bytes_served=download_events.bytes_served+EXCLUDED.bytes_served, event_count=download_events.event_count+EXCLUDED.event_count`
	} else {
		query += ` ON CONFLICT(aggregate_key) DO UPDATE SET bytes_served=bytes_served+excluded.bytes_served, event_count=event_count+excluded.event_count`
	}
	_, err := DB.Exec(rebind(query),
		e.AuthorizationID, e.FilePath, e.FileName, e.Launcher, e.Version, e.ClientIP,
		e.Country, e.BytesServed, completed, e.StatusCode, e.Date, 1, key, now.Format(AuthzTimeFormat))
	if err != nil {
		return fmt.Errorf("upsert download_event aggregate: %w", err)
	}
	return nil
}

// GetDailyServedByIPFromEvents 返回某 IP 某日（UTC date "2006-01-02"）的真实发送字节。
// 防刷墙按 IP 当日 served 口径的查询点。
func GetDailyServedByIPFromEvents(ip, date string) (int64, error) {
	var n int64
	err := DB.QueryRow(rebind(`SELECT COALESCE(SUM(bytes_served), 0) FROM download_events
		WHERE client_ip=? AND date=?`), ip, date).Scan(&n)
	return n, err
}

// GetDailyServedByIPFromEventsToday 返回某 IP 当日（UTC）的真实发送字节，供防刷墙使用。
func GetDailyServedByIPFromEventsToday(ip string) (int64, error) {
	return GetDailyServedByIPFromEvents(ip, time.Now().UTC().Format("2006-01-02"))
}

// GetTotalServedFromEvents 返回所有事件行的真实发送字节总和（新数据口径）。
func GetTotalServedFromEvents() (int64, error) {
	var n int64
	err := DB.QueryRow(rebind(`SELECT COALESCE(SUM(bytes_served), 0) FROM download_events`)).Scan(&n)
	return n, err
}

// GetTotalCompletedFromEvents 返回完整传输字节总和（completed=1 的 bytes_served）。
func GetTotalCompletedFromEvents() (int64, error) {
	var n int64
	err := DB.QueryRow(rebind(`SELECT COALESCE(SUM(bytes_served), 0) FROM download_events
		WHERE completed=1`)).Scan(&n)
	return n, err
}

// GetDailyEventStats 返回最近 days 天的按日事件聚合（served/completed/count）。
func GetDailyEventStats(days int) ([]EventDailyStat, error) {
	threshold := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := DB.Query(rebind(`SELECT date,
		COALESCE(SUM(bytes_served), 0),
		COALESCE(SUM(CASE WHEN completed=1 THEN bytes_served ELSE 0 END), 0),
		COALESCE(SUM(event_count), 0)
		FROM download_events WHERE date >= ? GROUP BY date ORDER BY date`), threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []EventDailyStat
	for rows.Next() {
		var s EventDailyStat
		if err := rows.Scan(&s.Date, &s.Served, &s.Completed, &s.Count); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// GetTopDownloadsFromEvents 返回下载次数排行（按 launcher 聚合，排除空 launcher）。
func GetTopDownloadsFromEvents(limit int) ([]DownloadRank, error) {
	rows, err := DB.Query(rebind(`SELECT launcher, COALESCE(SUM(event_count), 0) AS c FROM download_events
		WHERE launcher != '' GROUP BY launcher ORDER BY c DESC LIMIT ?`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ranks []DownloadRank
	for rows.Next() {
		var r DownloadRank
		if err := rows.Scan(&r.Launcher, &r.Count); err != nil {
			continue
		}
		ranks = append(ranks, r)
	}
	return ranks, rows.Err()
}

// GetTotalDownloadsFromEvents 返回事件表的总下载次数（含历史回填行）。
func GetTotalDownloadsFromEvents() (int64, error) {
	var n int64
	err := DB.QueryRow(rebind(`SELECT COALESCE(SUM(event_count), 0) FROM download_events`)).Scan(&n)
	return n, err
}
