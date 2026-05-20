package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"
	"lemwood_mirror/internal/traffic"
)

const (
	serverTestGB = int64(1024 * 1024 * 1024)
)

func setupDownloadHandlerTest(t *testing.T, limitGB int, content string) (http.Handler, string) {
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

	cfg := &config.Config{
		CaptchaEnabled: false,
		AppealContact:  "test-contact",
	}
	if err := db.InitDB(base, cfg); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	traffic.InitTracker(limitGB, "banned_ips.txt", "test-contact", base)

	state := NewState(base, base, cfg)
	mux := http.NewServeMux()
	state.Routes(mux)

	t.Cleanup(func() {
		traffic.CloseTracker()
		if db.DB != nil {
			_ = db.DB.Close()
			db.DB = nil
		}
	})

	return SecurityMiddleware(mux), "/download/launcher/v1/file.txt"
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
