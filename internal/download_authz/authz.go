// Package download_authz 实现基于 DB 状态表（download_authorizations）的下载授权签发与校验，
// 替代旧的内存 download_token。明文 opaque token 只在签发响应中返回一次；DB 只存 SHA-256 哈希。
//
// token：32 字节随机数的无填充 base64url 表示（固定 43 字符，对齐 PoW实现.md §1.6）。
// 存储：token_hash = SHA-256(token_string) 的 hex；状态机 issued -> consumed -> expired。
// 单次消费：Consume 原子地把 issued 且未过期标记为 consumed（见 db.ConsumeDownloadAuthorization）。
package download_authz

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"lemwood_mirror/internal/db"
)

const defaultTTL = 5 * time.Minute

// Manager 管理下载授权的签发/查询/消费，全部落到 DB 状态表。
type Manager struct {
	ttl time.Duration
}

// IssueRequest 是签发授权的入参。
type IssueRequest struct {
	FilePath   string
	ReturnURL  string
	Source     string
	Flow       string
	ClientIP   string
	SourceKind string // web | api
	MaxBytes   int64
	RangeLimit int
	RequestID  string
}

// NewManager 创建授权管理器。ttl<=0 时用默认 5 分钟。
func NewManager(ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Manager{ttl: ttl}
}

// TTL 返回授权有效期。
func (m *Manager) TTL() time.Duration { return m.ttl }

// Issue 签发一条新授权，返回明文 token（仅此一次）与落库的授权记录。
func (m *Manager) Issue(req IssueRequest) (string, db.DownloadAuthorization, error) {
	token, err := generateToken()
	if err != nil {
		return "", db.DownloadAuthorization{}, err
	}
	tokenHash := hashToken(token)
	expires := time.Now().Add(m.ttl).UTC().Format(db.AuthzTimeFormat)
	auth := db.DownloadAuthorization{
		AuthorizationID: generateAuthorizationID(),
		TokenHash:       tokenHash,
		FilePath:        req.FilePath,
		ReturnURL:       req.ReturnURL,
		Source:          req.Source,
		Flow:            req.Flow,
		ClientIP:        req.ClientIP,
		SourceKind:      req.SourceKind,
		Status:          "issued",
		ExpiresAt:       expires,
		MaxBytes:        req.MaxBytes,
		RangeLimit:      req.RangeLimit,
		RequestID:       req.RequestID,
	}
	if err := db.CreateDownloadAuthorization(auth); err != nil {
		return "", db.DownloadAuthorization{}, fmt.Errorf("download_authz: issue: %w", err)
	}
	return token, auth, nil
}

// Peek 查询 token 对应的未消费授权（不消费）。返回授权记录与是否有效。
func (m *Manager) Peek(token string) (db.DownloadAuthorization, bool) {
	auth, err := db.GetDownloadAuthorizationByTokenHash(hashToken(token))
	if err != nil {
		return db.DownloadAuthorization{}, false
	}
	if auth.Status != "issued" {
		return auth, false
	}
	if isExpired(auth.ExpiresAt) {
		return auth, false
	}
	return auth, true
}

// Consume 原子消费 token 对应的授权（issued 且未过期 → consumed）。
// 返回更新后的授权记录与是否成功。
func (m *Manager) Consume(token string) (db.DownloadAuthorization, bool) {
	auth, ok, err := db.ConsumeDownloadAuthorization(hashToken(token))
	if err != nil || !ok {
		return db.DownloadAuthorization{}, false
	}
	return auth, true
}

// generateToken 生成 32 字节随机数，无填充 base64url 编码（固定 43 字符）。
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("download_authz: read random: %w", err)
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// generateAuthorizationID 生成 16 字节随机 hex 作为授权 ID。
func generateAuthorizationID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("auth_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// hashToken 计算 token 字符串的 SHA-256 hex（即 token_hash）。
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// isExpired 解析 expires_at 并与当前比较。
// modernc.org/sqlite 读取 DATETIME 列时会把文本解析为 time.Time 再以 RFC3339 回传，
// 故此处兼容多种格式（AuthzTimeFormat 为主，RFC3339 等为兜底）。
// 解析失败视为已过期（安全侧失败）。
func isExpired(expiresAt string) bool {
	t, ok := parseAuthzTime(expiresAt)
	if !ok {
		return true
	}
	return time.Now().UTC().After(t)
}

// parseAuthzTime 尝试用多种布局解析时间字符串（UTC）。
func parseAuthzTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		db.AuthzTimeFormat,
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
