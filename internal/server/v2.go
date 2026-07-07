package server

import (
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lemwood_mirror/internal/auth"
	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"
	"lemwood_mirror/internal/netutil"
	"lemwood_mirror/internal/selfupdate"
	"lemwood_mirror/internal/stats"
	"lemwood_mirror/internal/traffic"
	"lemwood_mirror/internal/version"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ============================================================
// v2 响应信封类型
// ============================================================

type v2Meta struct {
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
	RequestID string `json:"request_id,omitempty"`
	Cached    bool   `json:"cached,omitempty"`
}

type v2ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type v2Envelope struct {
	Data  any          `json:"data"`
	Error *v2ErrorBody `json:"error"`
	Meta  v2Meta       `json:"meta"`
}

// ============================================================
// v2 辅助函数
// ============================================================

const (
	v2CacheControl  = "public, max-age=300"
	v2GzipThreshold = 1024
)

// generateRequestID 生成 16 字符十六进制请求 ID。
func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b)
}

// computeETag 对数据计算弱 ETag（基于 SHA-256 前 16 字节）。
func computeETag(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf(`W/"%s"`, hex.EncodeToString(h[:16]))
}

// writeV2Success 写入成功信封，支持 ETag 条件请求和 gzip 压缩。
// 可选 statusCode 参数覆盖默认 200（如扫描接口传 202）。
func writeV2Success(w http.ResponseWriter, r *http.Request, data any, cached bool, statusCodes ...int) {
	status := http.StatusOK
	if len(statusCodes) > 0 {
		status = statusCodes[0]
	}

	// ETag 基于数据载荷计算（不含 timestamp/request_id 等每次变化的字段）
	dataBytes, err := json.Marshal(data)
	if err != nil {
		writeV2Error(w, r, http.StatusInternalServerError, "internal_error", "Failed to encode response", nil)
		return
	}
	etag := computeETag(dataBytes)

	// ETag 条件请求（仅对 200 响应有意义）
	if status == http.StatusOK {
		if inm := r.Header.Get("If-None-Match"); inm != "" {
			for _, v := range strings.Split(inm, ",") {
				if strings.TrimSpace(v) == etag {
					w.Header().Set("ETag", etag)
					w.Header().Set("Cache-Control", v2CacheControl)
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}
		}
	}

	meta := v2Meta{
		Version:   "v2",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: generateRequestID(),
		Cached:    cached,
	}
	envelope := v2Envelope{Data: data, Error: nil, Meta: meta}

	body, err := json.Marshal(envelope)
	if err != nil {
		writeV2Error(w, r, http.StatusInternalServerError, "internal_error", "Failed to encode response", nil)
		return
	}

	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", v2CacheControl)
	w.Header().Set("Content-Type", "application/json")

	// gzip 压缩（仅对足够大的响应）
	if acceptsGzip(r) && len(body) > v2GzipThreshold {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		w.WriteHeader(status)
		gz.Write(body)
		return
	}

	w.WriteHeader(status)
	w.Write(body)
}

// writeV2Error 写入错误信封，data 为 null。
func writeV2Error(w http.ResponseWriter, r *http.Request, statusCode int, code, message string, details map[string]any) {
	meta := v2Meta{
		Version:   "v2",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: generateRequestID(),
	}
	envelope := v2Envelope{
		Data:  nil,
		Error: &v2ErrorBody{Code: code, Message: message, Details: details},
		Meta:  meta,
	}
	body, _ := json.Marshal(envelope)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}

// acceptsGzip 检查客户端是否接受 gzip 压缩。
func acceptsGzip(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

// ============================================================
// v2 Admin 中间件（返回信封格式错误）
// ============================================================

// v2AdminSwitchMiddleware 检查 admin 是否启用，未启用时返回 v2 错误信封。
func (s *State) v2AdminSwitchMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.Config.AdminEnabled {
			writeV2Error(w, r, http.StatusForbidden, "admin_disabled", "Admin console is disabled", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// v2AdminMiddleware 验证 token，无效时返回 v2 错误信封。
func (s *State) v2AdminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			if cookie, err := r.Cookie("admin_token"); err == nil {
				token = cookie.Value
			}
		}
		// 兼容 "Bearer <token>" 格式
		token = strings.TrimPrefix(token, "Bearer ")
		if token == "" || !auth.ValidateToken(token) {
			writeV2Error(w, r, http.StatusUnauthorized, "unauthorized", "Unauthorized", nil)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// ============================================================
// v2 公共查询 Handler
// ============================================================

// handleV2Status 返回所有启动器状态（信封包裹）。
func (s *State) handleV2Status(w http.ResponseWriter, r *http.Request) {
	result := s.getLauncherStatusData(r)
	writeV2Success(w, r, result, false)
}

// handleV2LauncherStatus 返回单个启动器版本数组（信封包裹）。
func (s *State) handleV2LauncherStatus(w http.ResponseWriter, r *http.Request) {
	launcher := strings.TrimPrefix(r.URL.Path, "/api/v2/launchers/")

	s.mu.RLock()
	versions, ok := s.index[launcher]
	versionsCopy := make(map[string]string)
	if ok {
		for k, v := range versions {
			versionsCopy[k] = v
		}
	}
	infoCacheCopy := make(map[string]map[string]interface{})
	for k, v := range s.infoCache {
		infoCacheCopy[k] = v
	}
	s.mu.RUnlock()

	if !ok {
		writeV2Error(w, r, http.StatusNotFound, "not_found", fmt.Sprintf("Launcher %q not found", launcher), nil)
		return
	}

	list := buildVersionList(versionsCopy, infoCacheCopy, s)

	// 注入 clone_url
	if s.Config != nil {
		for _, lc := range s.Config.Launchers {
			if lc.Name == launcher && config.ShouldSyncClone(lc.Mode) {
				cloneURL := s.cloneRepoURL(r, lc.Name)
				for i := range list {
					list[i]["clone_url"] = cloneURL
				}
				break
			}
		}
	}

	writeV2Success(w, r, list, false)
}

// handleV2LatestAll 返回所有启动器最新版本 map（信封包裹）。
func (s *State) handleV2LatestAll(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	latestCopy := make(map[string]string, len(s.latest))
	for k, v := range s.latest {
		latestCopy[k] = v
	}
	s.mu.RUnlock()

	// 保留 X-Latest-Versions 头以兼容
	if b, err := json.Marshal(latestCopy); err == nil {
		w.Header().Set("X-Latest-Versions", string(b))
	}
	writeV2Success(w, r, latestCopy, false)
}

// handleV2LatestLauncher 返回指定启动器最新版本号（信封包裹）。
func (s *State) handleV2LatestLauncher(w http.ResponseWriter, r *http.Request) {
	launcher := strings.TrimPrefix(r.URL.Path, "/api/v2/latest/")
	s.mu.RLock()
	defer s.mu.RUnlock()
	if val, ok := s.latest[launcher]; ok {
		// 保留 X-Latest-Version 头以兼容旧客户端
		w.Header().Set("X-Latest-Version", val)
		writeV2Success(w, r, map[string]string{"version": val}, false)
	} else {
		writeV2Error(w, r, http.StatusNotFound, "not_found", fmt.Sprintf("Launcher %q not found", launcher), nil)
	}
}

// handleV2Stats 返回站点统计信息（信封包裹）。
func (s *State) handleV2Stats(w http.ResponseWriter, r *http.Request) {
	data, err := stats.GetStats(s.BasePath)
	if err != nil {
		log.Printf("获取统计数据失败: %v", err)
		writeV2Error(w, r, http.StatusInternalServerError, "internal_error", "Failed to fetch stats", nil)
		return
	}
	writeV2Success(w, r, data, true)
}

// handleV2CaptchaConfig 返回验证码配置（信封包裹）。
func (s *State) handleV2CaptchaConfig(w http.ResponseWriter, r *http.Request) {
	writeV2Success(w, r, map[string]any{
		"enabled": s.Config.CaptchaEnabled,
		"app_id":  s.Config.CaptchaAppId,
	}, false)
}

// handleV2Auth2FAStatus 返回 2FA 状态（信封包裹）。
func (s *State) handleV2Auth2FAStatus(w http.ResponseWriter, r *http.Request) {
	writeV2Success(w, r, map[string]bool{
		"enabled": s.Config.TwoFactorEnabled,
	}, false)
}

// ============================================================
// v2 认证 Handler
// ============================================================

// loginOutcome 表示登录结果，Token 非空表示成功。
type loginOutcome struct {
	Token      string
	StatusCode int
	Code       string
	Message    string
}

// processLogin 执行登录核心逻辑，供 v2 调用（v1 handleLogin 保持独立不变）。
func (s *State) processLogin(r *http.Request) loginOutcome {
	ip := netutil.ExtractClientIP(r)

	// 检查锁定状态
	s.loginAttemptsMu.Lock()
	if unlockTime, locked := s.loginLocks[ip]; locked {
		if time.Now().Before(unlockTime) {
			s.loginAttemptsMu.Unlock()
			diff := time.Until(unlockTime).Round(time.Second)
			return loginOutcome{
				StatusCode: http.StatusForbidden,
				Code:       "account_locked",
				Message:    fmt.Sprintf("账号已被锁定，请在 %v 后重试", diff),
			}
		}
		delete(s.loginLocks, ip)
		delete(s.loginAttempts, ip)
	}
	s.loginAttemptsMu.Unlock()

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		OTPCode  string `json:"otp_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return loginOutcome{StatusCode: http.StatusBadRequest, Code: "bad_request", Message: "Bad Request"}
	}

	if s.Config.AdminUser == "" || s.Config.AdminPassword == "" {
		return loginOutcome{StatusCode: http.StatusInternalServerError, Code: "admin_not_configured", Message: "Admin account not configured"}
	}

	// 验证用户名密码
	if req.Username != s.Config.AdminUser || !auth.CheckPasswordHash(req.Password, s.Config.AdminPassword) {
		s.loginAttemptsMu.Lock()
		s.loginAttempts[ip]++
		attempts := s.loginAttempts[ip]
		if attempts >= s.Config.AdminMaxRetries {
			lockUntil := time.Now().Add(time.Duration(s.Config.AdminLockDuration) * time.Minute)
			s.loginLocks[ip] = lockUntil
			log.Printf("IP %s 登录失败次数达到上限 (%d)，已锁定至 %v", ip, attempts, lockUntil.Format("2006-01-02 15:04:05"))
			s.loginAttemptsMu.Unlock()
			return loginOutcome{
				StatusCode: http.StatusForbidden,
				Code:       "account_locked",
				Message:    fmt.Sprintf("登录失败次数过多，账号已锁定 %d 小时", s.Config.AdminLockDuration/60),
			}
		}
		log.Printf("IP %s 登录失败 (%d/%d)", ip, attempts, s.Config.AdminMaxRetries)
		s.loginAttemptsMu.Unlock()
		return loginOutcome{StatusCode: http.StatusUnauthorized, Code: "invalid_credentials", Message: "Invalid credentials"}
	}

	// 验证 2FA
	if s.Config.TwoFactorEnabled {
		if req.OTPCode == "" {
			return loginOutcome{StatusCode: http.StatusUnauthorized, Code: "otp_required", Message: "需要验证码"}
		}
		if !auth.ValidateTOTP(req.OTPCode, s.Config.TwoFactorSecret) {
			s.loginAttemptsMu.Lock()
			s.loginAttempts[ip]++
			attempts := s.loginAttempts[ip]
			if attempts >= s.Config.AdminMaxRetries {
				lockUntil := time.Now().Add(time.Duration(s.Config.AdminLockDuration) * time.Minute)
				s.loginLocks[ip] = lockUntil
				log.Printf("IP %s 2FA 验证失败次数达到上限 (%d)，已锁定至 %v", ip, attempts, lockUntil.Format("2006-01-02 15:04:05"))
				s.loginAttemptsMu.Unlock()
				return loginOutcome{
					StatusCode: http.StatusForbidden,
					Code:       "account_locked",
					Message:    fmt.Sprintf("登录失败次数过多，账号已锁定 %d 小时", s.Config.AdminLockDuration/60),
				}
			}
			log.Printf("IP %s 2FA 验证失败 (%d/%d)", ip, attempts, s.Config.AdminMaxRetries)
			s.loginAttemptsMu.Unlock()
			return loginOutcome{StatusCode: http.StatusUnauthorized, Code: "invalid_otp", Message: "验证码错误"}
		}
	}

	// 登录成功，重置计数
	s.loginAttemptsMu.Lock()
	delete(s.loginAttempts, ip)
	delete(s.loginLocks, ip)
	s.loginAttemptsMu.Unlock()

	token, err := auth.GenerateToken()
	if err != nil {
		return loginOutcome{StatusCode: http.StatusInternalServerError, Code: "token_generation_failed", Message: "Failed to generate token"}
	}
	return loginOutcome{Token: token}
}

// handleV2Login 管理员登录（信封包裹）。
func (s *State) handleV2Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}
	outcome := s.processLogin(r)
	if outcome.Token != "" {
		writeV2Success(w, r, map[string]string{"token": outcome.Token}, false)
		return
	}
	writeV2Error(w, r, outcome.StatusCode, outcome.Code, outcome.Message, nil)
}

// ============================================================
// v2 下载 Handler
// ============================================================

// handleV2DownloadPrepare 准备下载（无验证码），信封包裹。
func (s *State) handleV2DownloadPrepare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}

	var req downloadPrepareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV2Error(w, r, http.StatusBadRequest, "bad_request", "Bad Request", nil)
		return
	}

	if req.FilePath == "" {
		writeV2Error(w, r, http.StatusBadRequest, "missing_required_parameters", "Missing required parameters", nil)
		return
	}

	if _, _, validationErr := s.validateDownloadFile(req.FilePath); validationErr != nil {
		writeV2Error(w, r, validationErr.StatusCode, validationErr.Code, validationErr.Message, nil)
		return
	}

	resp, err := s.issueDownloadToken(req.FilePath, req.ReturnURL, req.Source, "prepare")
	if err != nil {
		writeV2Error(w, r, http.StatusInternalServerError, "token_generation_failed", "Failed to generate download token", nil)
		return
	}

	// v2 landing_url 指向 v2 端点
	resp.LandingURL = fmt.Sprintf("/api/v2/downloads/landing?token=%s", url.QueryEscape(resp.DownloadToken))
	writeV2Success(w, r, resp, false)
}

// handleV2DownloadLanding 获取下载引导信息，信封包裹。
func (s *State) handleV2DownloadLanding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		writeV2Error(w, r, http.StatusBadRequest, "missing_token", "Missing token", nil)
		return
	}

	entry, valid := s.downloadTokenMgr.Peek(token)
	if !valid {
		writeV2Error(w, r, http.StatusForbidden, "expired_token", "Download token is invalid or expired", nil)
		return
	}

	writeV2Success(w, r, map[string]string{
		"download_url": buildDownloadURL(token, entry.FilePath),
		"return_url":   entry.ReturnURL,
		"source":       entry.Source,
		"file_name":    filepath.Base(entry.FilePath),
		"file_path":    entry.FilePath,
		"flow":         entry.Flow,
	}, false)
}

// handleV2DownloadVerify 验证码验证后生成下载令牌，信封包裹。
func (s *State) handleV2DownloadVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}

	if !s.Config.CaptchaEnabled || s.captchaValidator == nil {
		writeV2Error(w, r, http.StatusBadRequest, "captcha_not_enabled", "Captcha not enabled", nil)
		return
	}

	var req downloadVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV2Error(w, r, http.StatusBadRequest, "bad_request", "Bad Request", nil)
		return
	}

	if req.LotNumber == "" || req.CaptchaOutput == "" || req.PassToken == "" || req.GenTime == "" || req.FilePath == "" {
		writeV2Error(w, r, http.StatusBadRequest, "missing_required_parameters", "Missing required parameters", nil)
		return
	}

	ip := netutil.ExtractClientIP(r)
	result, err := s.captchaValidator.Verify(req.LotNumber, req.CaptchaOutput, req.PassToken, req.GenTime, ip)
	if err != nil {
		log.Printf("验证码验证失败: %v", err)
		writeV2Error(w, r, http.StatusInternalServerError, "verification_failed", "Failed to verify captcha", nil)
		return
	}

	if result.Result != "success" {
		log.Printf("验证码验证不成功: result=%s, reason=%s", result.Result, result.Reason)
		writeV2Error(w, r, http.StatusForbidden, "verification_failed", result.Reason, nil)
		return
	}

	if _, _, validationErr := s.validateDownloadFile(req.FilePath); validationErr != nil {
		writeV2Error(w, r, validationErr.StatusCode, validationErr.Code, validationErr.Message, nil)
		return
	}

	resp, err := s.issueDownloadToken(req.FilePath, req.ReturnURL, req.Source, "verify")
	if err != nil {
		writeV2Error(w, r, http.StatusInternalServerError, "token_generation_failed", "Failed to generate download token", nil)
		return
	}

	resp.LandingURL = fmt.Sprintf("/api/v2/downloads/landing?token=%s", url.QueryEscape(resp.DownloadToken))
	writeV2Success(w, r, resp, false)
}

// ============================================================
// v2 扫描 Handler
// ============================================================

// handleV2ScanAll 触发全量扫描（信封包裹）。
func (s *State) handleV2ScanAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}
	if s.scanAllFunc == nil {
		writeV2Error(w, r, http.StatusNotImplemented, "scan_unavailable", "Scan not available", nil)
		return
	}
	go s.scanAllFunc()
	writeV2Success(w, r, map[string]string{"status": "accepted", "message": "Scan triggered"}, false, http.StatusAccepted)
}

// handleV2ScanLauncher 触发单个启动器扫描（信封包裹）。
func (s *State) handleV2ScanLauncher(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}
	if s.scanLauncherFunc == nil {
		writeV2Error(w, r, http.StatusNotImplemented, "scan_unavailable", "Scan not available", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req LauncherScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV2Error(w, r, http.StatusBadRequest, "bad_request", "Invalid request body", nil)
		return
	}
	if req.Launcher == "" {
		writeV2Error(w, r, http.StatusBadRequest, "missing_required_parameters", "launcher is required", nil)
		return
	}

	go s.scanLauncherFunc(req.Launcher)
	writeV2Success(w, r, LauncherScanResponse{Status: "accepted", Message: "扫描已触发"}, false, http.StatusAccepted)
}

// ============================================================
// v2 内部辅助（数据获取）
// ============================================================

// getLauncherStatusData 获取所有启动器状态数据，供 v2 信封包裹。
func (s *State) getLauncherStatusData(r *http.Request) map[string][]map[string]any {
	s.mu.RLock()
	indexCopy := make(map[string]map[string]string)
	for k, v := range s.index {
		indexCopy[k] = make(map[string]string)
		for vk, vv := range v {
			indexCopy[k][vk] = vv
		}
	}
	infoCacheCopy := make(map[string]map[string]interface{})
	for k, v := range s.infoCache {
		infoCacheCopy[k] = v
	}
	s.mu.RUnlock()

	result := make(map[string][]map[string]any)
	for launcher, versions := range indexCopy {
		list := buildVersionList(versions, infoCacheCopy, s)
		result[launcher] = list
	}

	// 为 clone/all 模式启动器注入 clone_url
	if s.Config != nil {
		for _, lc := range s.Config.Launchers {
			if !config.ShouldSyncClone(lc.Mode) {
				continue
			}
			if list, ok := result[lc.Name]; ok {
				cloneURL := s.cloneRepoURL(r, lc.Name)
				for i := range list {
					list[i]["clone_url"] = cloneURL
				}
			}
		}
	}

	return result
}

// buildVersionList 根据版本映射和缓存构建排序后的版本列表。
func buildVersionList(versions map[string]string, infoCache map[string]map[string]interface{}, s *State) []map[string]any {
	var list []map[string]any
	for v, p := range versions {
		info := map[string]any{
			"tag_name": v,
		}

		if fileInfo, ok := infoCache[p]; ok {
			for k, val := range fileInfo {
				if k != "is_latest" {
					info[k] = val
				}
			}
		} else {
			if content, err := os.ReadFile(p); err == nil {
				var fileInfo map[string]any
				if err := json.Unmarshal(content, &fileInfo); err == nil {
					infoCache[p] = fileInfo
					s.updateInfoCache(p, fileInfo)
					for k, val := range fileInfo {
						if k != "is_latest" {
							info[k] = val
						}
					}
				}
			}
		}

		list = append(list, info)
	}
	sort.Slice(list, func(i, j int) bool {
		v1, _ := list[i]["tag_name"].(string)
		v2, _ := list[j]["tag_name"].(string)
		return version.Compare(v1, v2) > 0
	})
	return list
}

// ============================================================
// v2 管理后台 Handler（补全：config/blacklist/files/self-update）
// ============================================================

// handleV2AdminConfig 管理后台配置读写（信封包裹）。
func (s *State) handleV2AdminConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		cfgCopy := *s.Config
		cfgCopy.AdminPassword = "" // 不返回密码哈希
		writeV2Success(w, r, cfgCopy, false)
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
		var newCfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			writeV2Error(w, r, http.StatusBadRequest, "bad_request", "Bad Request", nil)
			return
		}

		// 保持密码不变，除非提供了新密码
		if newCfg.AdminPassword == "" {
			newCfg.AdminPassword = s.Config.AdminPassword
		} else {
			hashed, err := auth.HashPassword(newCfg.AdminPassword)
			if err != nil {
				writeV2Error(w, r, http.StatusInternalServerError, "hash_failed", "Failed to hash password", nil)
				return
			}
			newCfg.AdminPassword = hashed
		}

		if err := config.NormalizeConfig(&newCfg); err != nil {
			writeV2Error(w, r, http.StatusBadRequest, "invalid_config", fmt.Sprintf("Invalid config: %v", err), nil)
			return
		}

		if err := newCfg.Save(s.ProjectRoot); err != nil {
			writeV2Error(w, r, http.StatusInternalServerError, "save_failed", "Failed to save config", nil)
			return
		}

		s.mu.Lock()
		s.Config = &newCfg
		manager := s.selfUpdate
		s.mu.Unlock()

		if manager != nil {
			manager.UpdateConfig(selfupdate.Config{
				Enabled:       newCfg.SelfUpdateEnabled,
				RepoURL:       newCfg.SelfUpdateRepoURL,
				Channel:       newCfg.SelfUpdateChannel,
				AutoRestart:   newCfg.SelfUpdateAutoRestart,
				ProxyURL:      newCfg.ProxyURL,
				AssetProxyURL: newCfg.AssetProxyURL,
			})
		}

		writeV2Success(w, r, map[string]string{"message": "Config updated"}, false)
		return
	}

	writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
}

// handleV2AdminBlacklist 黑名单管理（信封包裹）。
func (s *State) handleV2AdminBlacklist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := db.GetIPBlacklist()
		if err != nil {
			writeV2Error(w, r, http.StatusInternalServerError, "internal_error", err.Error(), nil)
			return
		}
		writeV2Success(w, r, list, false)
	case http.MethodPost:
		var req struct {
			IP     string `json:"ip"`
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeV2Error(w, r, http.StatusBadRequest, "bad_request", "Bad Request", nil)
			return
		}
		if err := db.AddIPToBlacklist(req.IP, req.Reason); err != nil {
			writeV2Error(w, r, http.StatusInternalServerError, "internal_error", err.Error(), nil)
			return
		}
		traffic.SyncBanRecord()
		writeV2Success(w, r, map[string]string{"message": "added"}, false, http.StatusCreated)
	case http.MethodDelete:
		ip := r.URL.Query().Get("ip")
		if ip == "" {
			writeV2Error(w, r, http.StatusBadRequest, "missing_param", "Missing ip parameter", nil)
			return
		}
		if err := db.RemoveIPFromBlacklist(ip); err != nil {
			writeV2Error(w, r, http.StatusInternalServerError, "internal_error", err.Error(), nil)
			return
		}
		traffic.SyncBanRecord()
		writeV2Success(w, r, map[string]string{"message": "removed"}, false)
	default:
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
	}
}

// handleV2AdminFiles 文件管理（信封包裹，下载除外）。
func (s *State) handleV2AdminFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		path := r.URL.Query().Get("path")
		fullPath := filepath.Join(s.BasePath, path)

		absBase, _ := filepath.Abs(s.BasePath)
		absPath, _ := filepath.Abs(fullPath)
		if !strings.HasPrefix(absPath, absBase) {
			writeV2Error(w, r, http.StatusForbidden, "forbidden", "Forbidden", nil)
			return
		}

		entries, err := os.ReadDir(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				writeV2Error(w, r, http.StatusNotFound, "not_found", "Directory not found", nil)
				return
			}
			writeV2Error(w, r, http.StatusInternalServerError, "internal_error", err.Error(), nil)
			return
		}

		var result []map[string]interface{}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			result = append(result, map[string]interface{}{
				"name":     e.Name(),
				"is_dir":   e.IsDir(),
				"size":     info.Size(),
				"mod_time": info.ModTime(),
			})
		}
		writeV2Success(w, r, result, false)

	case http.MethodDelete:
		path := r.URL.Query().Get("path")
		if path == "" {
			writeV2Error(w, r, http.StatusBadRequest, "missing_param", "Missing path", nil)
			return
		}
		fullPath := filepath.Join(s.BasePath, path)
		if err := removePathUnderBase(s.BasePath, fullPath); err != nil {
			if errors.Is(err, errForbiddenPath) {
				writeV2Error(w, r, http.StatusForbidden, "forbidden", "Forbidden", nil)
				return
			}
			writeV2Error(w, r, http.StatusInternalServerError, "internal_error", err.Error(), nil)
			return
		}
		writeV2Success(w, r, map[string]string{"message": "deleted"}, false)

	case http.MethodPost:
		// 文件上传
		path := r.URL.Query().Get("path")
		if path == "" {
			writeV2Error(w, r, http.StatusBadRequest, "missing_param", "Missing path", nil)
			return
		}
		fullPath := filepath.Join(s.BasePath, path)

		absBase, _ := filepath.Abs(s.BasePath)
		absPath, _ := filepath.Abs(fullPath)
		if !strings.HasPrefix(absPath, absBase) {
			writeV2Error(w, r, http.StatusForbidden, "forbidden", "Forbidden", nil)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			writeV2Error(w, r, http.StatusBadRequest, "bad_request", "Failed to get file: "+err.Error(), nil)
			return
		}
		defer file.Close()

		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			writeV2Error(w, r, http.StatusInternalServerError, "internal_error", "Failed to create directory", nil)
			return
		}

		dst, err := os.Create(fullPath)
		if err != nil {
			writeV2Error(w, r, http.StatusInternalServerError, "internal_error", "Failed to create file: "+err.Error(), nil)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			writeV2Error(w, r, http.StatusInternalServerError, "internal_error", "Failed to save file", nil)
			return
		}

		writeV2Success(w, r, map[string]string{"message": "File uploaded"}, false)

	default:
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
	}
}

// handleV2AdminFileDownload 文件下载（二进制流，不走信封）。
func (s *State) handleV2AdminFileDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		writeV2Error(w, r, http.StatusBadRequest, "missing_param", "Missing path", nil)
		return
	}
	fullPath := filepath.Join(s.BasePath, path)

	absBase, _ := filepath.Abs(s.BasePath)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absBase) {
		writeV2Error(w, r, http.StatusForbidden, "forbidden", "Forbidden", nil)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		writeV2Error(w, r, http.StatusNotFound, "not_found", "File not found", nil)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(fullPath)))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, fullPath)
}

// handleV2AdminSelfUpdateStatus 自更新状态（信封包裹）。
func (s *State) handleV2AdminSelfUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}
	if s.selfUpdate == nil {
		writeV2Error(w, r, http.StatusNotImplemented, "not_configured", "Self update not configured", nil)
		return
	}
	writeV2Success(w, r, s.selfUpdate.Status(), false)
}

// handleV2AdminSelfUpdateCheck 检查更新（信封包裹）。
func (s *State) handleV2AdminSelfUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}
	if s.selfUpdate == nil {
		writeV2Error(w, r, http.StatusNotImplemented, "not_configured", "Self update not configured", nil)
		return
	}
	status, err := s.selfUpdate.Check(r.Context())
	if err != nil {
		writeV2Error(w, r, http.StatusBadGateway, "check_failed", err.Error(), nil)
		return
	}
	writeV2Success(w, r, status, false)
}

// handleV2AdminSelfUpdateApply 应用更新（信封包裹）。
func (s *State) handleV2AdminSelfUpdateApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}
	if s.selfUpdate == nil || s.applySelfUpdate == nil {
		writeV2Error(w, r, http.StatusNotImplemented, "not_configured", "Self update apply is not available", nil)
		return
	}
	if err := s.applySelfUpdate(r.Context()); err != nil {
		status := s.selfUpdate.SetApplyError(err)
		writeV2Error(w, r, http.StatusBadGateway, "apply_failed", err.Error(), map[string]any{"status": status})
		return
	}
	writeV2Success(w, r, s.selfUpdate.Status(), false)
}

// handleV2AdminSelfUpdateRestart 重启进程（信封包裹）。
func (s *State) handleV2AdminSelfUpdateRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}
	if s.restartProcess == nil {
		writeV2Error(w, r, http.StatusNotImplemented, "not_configured", "Restart is not available", nil)
		return
	}
	if err := s.restartProcess(); err != nil {
		writeV2Error(w, r, http.StatusInternalServerError, "restart_failed", err.Error(), nil)
		return
	}
	writeV2Success(w, r, map[string]string{
		"status":  "accepted",
		"message": "重启请求已触发",
	}, false)
}

// handleV2SelfUpdateCheckEndpoint 触发自更新检查（信封包裹）。
func (s *State) handleV2SelfUpdateCheckEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeV2Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
		return
	}
	if s.selfUpdateCheckFunc == nil {
		writeV2Error(w, r, http.StatusNotImplemented, "not_configured", "Self update check not available", nil)
		return
	}
	go s.selfUpdateCheckFunc()
	writeV2Success(w, r, map[string]string{"status": "accepted", "message": "Self update check triggered"}, false, http.StatusAccepted)
}
