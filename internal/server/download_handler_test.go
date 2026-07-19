package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"
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
		CaptchaEnabled: false,
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

	if err := db.RecordTraffic(ip, serverTestGB); err != nil {
		t.Fatalf("RecordTraffic() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	trafficBytes, err := db.GetDailyTraffic(ip)
	if err != nil {
		t.Fatalf("GetDailyTraffic() error = %v", err)
	}
	if trafficBytes != serverTestGB {
		t.Fatalf("daily traffic = %d, want %d", trafficBytes, serverTestGB)
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

	trafficBytes, err := db.GetDailyTraffic(ip)
	if err != nil {
		t.Fatalf("GetDailyTraffic() error = %v", err)
	}
	if trafficBytes != 0 {
		t.Fatalf("daily traffic = %d, want 0", trafficBytes)
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

	trafficBytes, err := db.GetDailyTraffic(ip)
	if err != nil {
		t.Fatalf("GetDailyTraffic() error = %v", err)
	}
	if trafficBytes != 2 {
		t.Fatalf("daily traffic = %d, want 2", trafficBytes)
	}
}

func TestDownloadPrepareReturnsLandingURL(t *testing.T) {
	cfg := &config.Config{
		CaptchaEnabled: false,
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

func TestDownloadLandingReturnsContext(t *testing.T) {
	cfg := &config.Config{
		CaptchaEnabled: false,
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
		CaptchaEnabled: false,
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

	if _, valid := state.downloadTokenMgr.Validate(token); !valid {
		t.Fatal("Validate() should consume token successfully")
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
		CaptchaEnabled:   true,
		CaptchaAppId:     "test-app",
		CaptchaSecretKey: "test-secret",
		AppealContact:    "test-contact",
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
