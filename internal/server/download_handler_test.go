package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"
	"lemwood_mirror/internal/pow"
	"lemwood_mirror/internal/stats"
	"lemwood_mirror/internal/traffic"
)

const (
	serverTestGB = int64(1024 * 1024 * 1024)
)

func setupDownloadHandlerState(t *testing.T, cfg *config.Config, limitGB int, content string) (*State, http.Handler, string) {
	t.Helper()

	base := t.TempDir()
	filePath := filepath.Join(base, "launcher", "v1", "file.txt")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if db.DB != nil {
		_ = db.DB.Close()
		db.DB = nil
	}

	if err := db.InitDB(base, cfg); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	traffic.InitTracker(limitGB, "banned_ips.txt", "test-contact", base)
	stats.InitWritePool(1, 20)

	state := NewState(base, base, cfg)
	mux := http.NewServeMux()
	state.Routes(mux)

	t.Cleanup(func() {
		stats.CloseWritePool()
		traffic.CloseTracker()
		if db.DB != nil {
			_ = db.DB.Close()
			db.DB = nil
		}
	})

	return state, SecurityMiddleware(mux), "/download/launcher/v1/file.txt"
}

func setupDownloadHandlerTest(t *testing.T, limitGB int, content string) (http.Handler, string) {
	t.Helper()

	cfg := &config.Config{
		PowEnabled:    false,
		AppealContact:  "test-contact",
	}
	_, handler, path := setupDownloadHandlerState(t, cfg, limitGB, content)
	return handler, path
}

// unwrapV2Envelope 解包 v2 信封响应，返回 data 字段。
func unwrapV2Envelope(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var env struct {
		Data  map[string]any `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("Unmarshal envelope error = %v, body = %s", err, string(body))
	}
	if env.Error != nil {
		t.Fatalf("v2 error: %s - %s", env.Error.Code, env.Error.Message)
	}
	return env.Data
}

// unwrapV2Error 解包 v2 信封错误响应，返回 error 字段。
func unwrapV2Error(t *testing.T, body []byte) (string, string) {
	t.Helper()
	var env struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("Unmarshal envelope error = %v, body = %s", err, string(body))
	}
	if env.Error == nil {
		t.Fatalf("expected v2 error envelope, got success: %s", string(body))
	}
	return env.Error.Code, env.Error.Message
}

func TestDownloadHandlerRejectsBeforeServingWhenLimitWouldBeExceeded(t *testing.T) {
	handler, path := setupDownloadHandlerTest(t, 1, "hello")
	ip := "127.0.0.1"

	// 预置当日 served 流量到事件表（防刷墙现读 download_events）
	if err := db.RecordDownloadEvent(db.DownloadEvent{ClientIP: ip, BytesServed: serverTestGB}); err != nil {
		t.Fatalf("RecordDownloadEvent() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	trafficBytes, err := db.GetDailyServedByIPFromEventsToday(ip)
	if err != nil {
		t.Fatalf("GetDailyServedByIPFromEventsToday() error = %v", err)
	}
	if trafficBytes != serverTestGB {
		t.Fatalf("daily served = %d, want %d", trafficBytes, serverTestGB)
	}
}

func TestDownloadHandlerDoesNotCountHeadRequests(t *testing.T) {
	handler, path := setupDownloadHandlerTest(t, 1, "hello")
	ip := "127.0.0.1"

	req := httptest.NewRequest(http.MethodHead, path, nil)
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	trafficBytes, err := db.GetDailyServedByIPFromEventsToday(ip)
	if err != nil {
		t.Fatalf("GetDailyServedByIPFromEventsToday() error = %v", err)
	}
	if trafficBytes != 0 {
		t.Fatalf("daily served = %d, want 0", trafficBytes)
	}
}

func TestDownloadHandlerCountsPartialContentBytes(t *testing.T) {
	handler, path := setupDownloadHandlerTest(t, 1, "hello")
	ip := "127.0.0.1"

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":1234"
	req.Header.Set("Range", "bytes=0-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusPartialContent)
	}

	trafficBytes, err := db.GetDailyServedByIPFromEventsToday(ip)
	if err != nil {
		t.Fatalf("GetDailyServedByIPFromEventsToday() error = %v", err)
	}
	if trafficBytes != 2 {
		t.Fatalf("daily served = %d, want 2", trafficBytes)
	}
}

// failAfterWriter 模拟客户端在接收 limit 字节后断开连接（Write 返回错误）。
type failAfterWriter struct {
	header    http.Header
	remaining int
	status    int
}

func newFailAfterWriter(limit int) *failAfterWriter {
	return &failAfterWriter{header: make(http.Header), remaining: limit}
}

func (w *failAfterWriter) Header() http.Header {
	return w.header
}

func (w *failAfterWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errors.New("simulated client disconnect")
	}
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, errors.New("simulated client disconnect")
	}
	w.remaining -= len(p)
	return len(p), nil
}

// 完整 GET：写出字节数达到预估值且状态为 200，应计入完整传输（展示）口径。
func TestDownloadHandlerRecordsCompletedTrafficOnFullDownload(t *testing.T) {
	content := "hello"
	handler, path := setupDownloadHandlerTest(t, 1, content)
	ip := "127.0.0.1"

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	completedBytes, err := db.GetTotalCompletedFromEvents()
	if err != nil {
		t.Fatalf("GetTotalCompletedFromEvents() error = %v", err)
	}
	if completedBytes != int64(len(content)) {
		t.Fatalf("completed traffic = %d, want %d", completedBytes, len(content))
	}
}

// 客户端中止的部分传输：served 口径仍记录已写出字节（防刷墙），
// 但完整传输（展示）口径不应计入。
func TestDownloadHandlerDoesNotRecordCompletedTrafficOnAbort(t *testing.T) {
	handler, path := setupDownloadHandlerTest(t, 1, "hello")
	ip := "127.0.0.1"

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":1234"
	// 5 字节文件只接收前 2 字节即"断开"
	w := newFailAfterWriter(2)

	handler.ServeHTTP(w, req)

	servedBytes, err := db.GetDailyServedByIPFromEventsToday(ip)
	if err != nil {
		t.Fatalf("GetDailyServedByIPFromEventsToday() error = %v", err)
	}
	if servedBytes != 2 {
		t.Fatalf("daily served = %d, want 2", servedBytes)
	}

	completedBytes, err := db.GetTotalCompletedFromEvents()
	if err != nil {
		t.Fatalf("GetTotalCompletedFromEvents() error = %v", err)
	}
	if completedBytes != 0 {
		t.Fatalf("completed traffic = %d, want 0", completedBytes)
	}
}

func TestDownloadPrepareReturnsLandingURL(t *testing.T) {
	cfg := &config.Config{
		PowEnabled:    false,
		AppealContact:  "test-contact",
	}
	_, handler, _ := setupDownloadHandlerState(t, cfg, 1, "hello")

	body := bytes.NewBufferString(`{"file_path":"launcher/v1/file.txt","return_url":"https://example.com/back","source":"homepage"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/downloads/prepare", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	resp := unwrapV2Envelope(t, rec.Body.Bytes())
	if resp["download_token"] == "" || resp["download_token"] == nil {
		t.Fatal("download_token should not be empty")
	}
	if resp["download_url"] == "" || resp["download_url"] == nil {
		t.Fatal("download_url should not be empty")
	}
	if resp["landing_url"] == "" || resp["landing_url"] == nil {
		t.Fatal("landing_url should not be empty")
	}
}

func TestDownloadPrepareWithCDNBaseURL(t *testing.T) {
	cfg := &config.Config{
		PowEnabled:    false,
		AppealContact: "test-contact",
		CDNBaseURL:    "https://cdn.example.com/",
	}
	_, handler, _ := setupDownloadHandlerState(t, cfg, 1, "hello")

	body := bytes.NewBufferString(`{"file_path":"launcher/v1/file.txt","return_url":"https://example.com/back","source":"homepage"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/downloads/prepare", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	resp := unwrapV2Envelope(t, rec.Body.Bytes())

	urls, ok := resp["download_urls"].([]interface{})
	if !ok || len(urls) != 2 {
		t.Fatalf("download_urls = %v, want 2 candidates", resp["download_urls"])
	}
	cdnURL, _ := urls[0].(string)
	relURL, _ := urls[1].(string)
	if !strings.HasPrefix(cdnURL, "https://cdn.example.com/download/launcher/v1/file.txt?token=") {
		t.Fatalf("cdn url = %q, want cdn prefix", cdnURL)
	}
	if !strings.HasPrefix(relURL, "/download/launcher/v1/file.txt?token=") {
		t.Fatalf("fallback url = %q, want relative path", relURL)
	}
	if resp["download_url"] != cdnURL {
		t.Fatalf("download_url = %v, want cdn first %q", resp["download_url"], cdnURL)
	}
	if !strings.Contains(cdnURL, url.QueryEscape(resp["download_token"].(string))) {
		t.Fatal("cdn url should embed the download token")
	}

	// landing 也应携带相同候选列表
	landingReq := httptest.NewRequest(http.MethodGet, resp["landing_url"].(string), nil)
	landingRec := httptest.NewRecorder()
	handler.ServeHTTP(landingRec, landingReq)
	if landingRec.Code != http.StatusOK {
		t.Fatalf("landing status = %d, want 200", landingRec.Code)
	}
	landingResp := unwrapV2Envelope(t, landingRec.Body.Bytes())
	lurls, ok := landingResp["download_urls"].([]interface{})
	if !ok || len(lurls) != 2 {
		t.Fatalf("landing download_urls = %v, want 2 candidates", landingResp["download_urls"])
	}
}

func TestDownloadPrepareWithoutCDNBaseURL(t *testing.T) {
	cfg := &config.Config{
		PowEnabled:    false,
		AppealContact: "test-contact",
	}
	_, handler, _ := setupDownloadHandlerState(t, cfg, 1, "hello")

	body := bytes.NewBufferString(`{"file_path":"launcher/v1/file.txt"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/downloads/prepare", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := unwrapV2Envelope(t, rec.Body.Bytes())
	urls, ok := resp["download_urls"].([]interface{})
	if !ok || len(urls) != 1 {
		t.Fatalf("download_urls = %v, want single relative fallback", resp["download_urls"])
	}
	if resp["download_url"] != urls[0] {
		t.Fatalf("download_url = %v, want equal to only candidate", resp["download_url"])
	}
}

func TestDownloadLandingReturnsContext(t *testing.T) {
	cfg := &config.Config{
		PowEnabled:    false,
		AppealContact:  "test-contact",
	}
	_, handler, _ := setupDownloadHandlerState(t, cfg, 1, "hello")

	body := bytes.NewBufferString(`{"file_path":"launcher/v1/file.txt","return_url":"https://example.com/back","source":"homepage"}`)
	prepareReq := httptest.NewRequest(http.MethodPost, "/api/v2/downloads/prepare", body)
	prepareReq.Header.Set("Content-Type", "application/json")
	prepareRec := httptest.NewRecorder()
	handler.ServeHTTP(prepareRec, prepareReq)

	prepareResp := unwrapV2Envelope(t, prepareRec.Body.Bytes())
	landingURL, ok := prepareResp["landing_url"].(string)
	if !ok || landingURL == "" {
		t.Fatalf("landing_url missing or invalid: %v", prepareResp["landing_url"])
	}

	landingReq := httptest.NewRequest(http.MethodGet, landingURL, nil)
	landingRec := httptest.NewRecorder()
	handler.ServeHTTP(landingRec, landingReq)

	if landingRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", landingRec.Code, http.StatusOK)
	}

	landingResp := unwrapV2Envelope(t, landingRec.Body.Bytes())

	if landingResp["return_url"] != "https://example.com/back" {
		t.Fatalf("return_url = %v, want %q", landingResp["return_url"], "https://example.com/back")
	}
	if landingResp["source"] != "homepage" {
		t.Fatalf("source = %v, want %q", landingResp["source"], "homepage")
	}
	if landingResp["file_name"] != "file.txt" {
		t.Fatalf("file_name = %v, want %q", landingResp["file_name"], "file.txt")
	}
}

func TestDownloadLandingRejectsConsumedToken(t *testing.T) {
	cfg := &config.Config{
		PowEnabled:    false,
		AppealContact:  "test-contact",
	}
	state, handler, _ := setupDownloadHandlerState(t, cfg, 1, "hello")

	body := bytes.NewBufferString(`{"file_path":"launcher/v1/file.txt","return_url":"https://example.com/back","source":"homepage"}`)
	prepareReq := httptest.NewRequest(http.MethodPost, "/api/v2/downloads/prepare", body)
	prepareReq.Header.Set("Content-Type", "application/json")
	prepareRec := httptest.NewRecorder()
	handler.ServeHTTP(prepareRec, prepareReq)

	prepareResp := unwrapV2Envelope(t, prepareRec.Body.Bytes())
	token, ok := prepareResp["download_token"].(string)
	if !ok || token == "" {
		t.Fatalf("download_token missing or invalid: %v", prepareResp["download_token"])
	}

	if _, ok := state.authzMgr.Consume(token); !ok {
		t.Fatal("Consume() should consume token successfully")
	}

	landingURL, _ := prepareResp["landing_url"].(string)
	landingReq := httptest.NewRequest(http.MethodGet, landingURL, nil)
	landingRec := httptest.NewRecorder()
	handler.ServeHTTP(landingRec, landingReq)

	if landingRec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", landingRec.Code, http.StatusForbidden)
	}
}

func TestCLIDownloadWithoutTokenStillRequiresVerificationJSON(t *testing.T) {
	cfg := &config.Config{
		PowEnabled:    true,
		AppealContact: "test-contact",
	}
	_, handler, path := setupDownloadHandlerState(t, cfg, 1, "hello")

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "curl/8.0")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if resp["error"] != "verification_required" {
		t.Fatalf("error = %v, want %q", resp["error"], "verification_required")
	}
}

// powTestCfg 返回启用 PoW 的低难度配置（cost=200/difficulty=10），使测试可在毫秒级求解。
func powTestCfg() *config.Config {
	return &config.Config{
		PowEnabled:     true,
		PowAlgorithm:    "PBKDF2-SHA256",
		PowCost:         200,
		PowKeyLength:    32,
		PowDifficulty:   10,
		PowChallengeTTL: "2m",
		AppealContact:   "test-contact",
	}
}

func TestPowChallengeAuthorizeFlow(t *testing.T) {
	cfg := powTestCfg()
	_, handler, _ := setupDownloadHandlerState(t, cfg, 1, "hello")

	// 1. 创建 PoW 挑战
	chReq := httptest.NewRequest(http.MethodGet, "/api/v2/downloads/challenge?file_path=launcher/v1/file.txt", nil)
	chRec := httptest.NewRecorder()
	handler.ServeHTTP(chRec, chReq)
	if chRec.Code != http.StatusOK {
		t.Fatalf("challenge status = %d, want 200", chRec.Code)
	}
	challenge := unwrapV2Envelope(t, chRec.Body.Bytes())

	// 2. 求解
	params := decodeChallengeParams(t, challenge)
	sol, ok := pow.Solve(params, 0)
	if !ok {
		t.Fatal("pow.Solve failed")
	}

	// 3. 提交解领取授权
	authBody, _ := json.Marshal(map[string]any{
		"challenge": challenge,
		"solution":  map[string]any{"counter": sol.Counter, "derivedKey": sol.DerivedKey},
	})
	authReq := httptest.NewRequest(http.MethodPost, "/api/v2/downloads/authorize", bytes.NewReader(authBody))
	authReq.Header.Set("Content-Type", "application/json")
	authRec := httptest.NewRecorder()
	handler.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusOK {
		t.Fatalf("authorize status = %d, want 200, body=%s", authRec.Code, authRec.Body.String())
	}
	authResp := unwrapV2Envelope(t, authRec.Body.Bytes())
	token, _ := authResp["download_token"].(string)
	if token == "" {
		t.Fatal("authorize: empty download_token")
	}
	downloadURL, _ := authResp["download_url"].(string)
	if downloadURL == "" {
		t.Fatal("authorize: empty download_url")
	}

	// 4. 用 token 下载
	dlReq := httptest.NewRequest(http.MethodGet, downloadURL, nil)
	dlRec := httptest.NewRecorder()
	handler.ServeHTTP(dlRec, dlReq)
	if dlRec.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200", dlRec.Code)
	}
	if dlRec.Body.String() != "hello" {
		t.Fatalf("download body = %q, want %q", dlRec.Body.String(), "hello")
	}

	// 5. token 在有效期内可多次使用（非单次消费）——重放应仍成功
	dlRec2 := httptest.NewRecorder()
	handler.ServeHTTP(dlRec2, dlReq)
	if dlRec2.Code != http.StatusOK {
		t.Fatalf("replay status = %d, want 200 (multi-use within TTL)", dlRec2.Code)
	}
	if dlRec2.Body.String() != "hello" {
		t.Fatalf("replay body = %q, want %q", dlRec2.Body.String(), "hello")
	}
}

func TestDownloadWithBearerToken(t *testing.T) {
	cfg := powTestCfg()
	_, handler, _ := setupDownloadHandlerState(t, cfg, 1, "hello")

	// prepare 取一个 token（不依赖 PoW）
	body := bytes.NewBufferString(`{"file_path":"launcher/v1/file.txt","source":"test"}`)
	prepReq := httptest.NewRequest(http.MethodPost, "/api/v2/downloads/prepare", body)
	prepReq.Header.Set("Content-Type", "application/json")
	prepRec := httptest.NewRecorder()
	handler.ServeHTTP(prepRec, prepReq)
	if prepRec.Code != http.StatusOK {
		t.Fatalf("prepare status = %d", prepRec.Code)
	}
	token, _ := unwrapV2Envelope(t, prepRec.Body.Bytes())["download_token"].(string)
	if token == "" {
		t.Fatal("prepare: empty token")
	}

	// 用 Authorization: Bearer 下载（路径不含 token）
	dlReq := httptest.NewRequest(http.MethodGet, "/download/launcher/v1/file.txt", nil)
	dlReq.Header.Set("Authorization", "Bearer "+token)
	dlReq.Header.Set("Accept", "application/octet-stream")
	dlRec := httptest.NewRecorder()
	handler.ServeHTTP(dlRec, dlReq)
	if dlRec.Code != http.StatusOK {
		t.Fatalf("Bearer download status = %d, want 200", dlRec.Code)
	}
	if dlRec.Body.String() != "hello" {
		t.Fatalf("Bearer download body = %q, want hello", dlRec.Body.String())
	}
}

func TestServePowPageForBrowserDirectHit(t *testing.T) {
	cfg := powTestCfg()
	_, handler, path := setupDownloadHandlerState(t, cfg, 1, "hello")

	// 浏览器直连（无 token）→ 302 重定向到前端验证页 /verify?file=...
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("browser direct status = %d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/verify?file=") || !strings.Contains(loc, "file.txt") {
		t.Fatalf("redirect Location = %q, want /verify?file=...", loc)
	}

	// CLI 直连（无 token）→ 403 JSON verification_required
	cliReq := httptest.NewRequest(http.MethodGet, path, nil)
	cliReq.Header.Set("Accept", "application/octet-stream")
	cliReq.Header.Set("User-Agent", "curl/8.0")
	cliRec := httptest.NewRecorder()
	handler.ServeHTTP(cliRec, cliReq)
	if cliRec.Code != http.StatusForbidden {
		t.Fatalf("cli direct status = %d, want 403", cliRec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(cliRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("cli direct unmarshal: %v", err)
	}
	if resp["error"] != "verification_required" {
		t.Fatalf("cli direct error = %v, want verification_required", resp["error"])
	}
}

// decodeChallengeParams 把信封中的 challenge map 解析成 pow.ChallengeParameters。
func decodeChallengeParams(t *testing.T, challenge map[string]any) pow.ChallengeParameters {
	t.Helper()
	raw, err := json.Marshal(challenge["parameters"])
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	var p pow.ChallengeParameters
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	return p
}
