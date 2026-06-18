package server

import (
	"context"
	"net/http"
	"path/filepath"
	"time"
)

type LauncherScanRequest struct {
	Launcher string `json:"launcher"`
}

type LauncherScanResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// StartHTTPWithScan creates the HTTP server and registers handlers.
// scanFunc, launcherScanFunc, and selfUpdateCheckFunc are stored on State
// and registered inside Routes with AdminMiddleware protection.
func StartHTTPWithScan(addr string, s *State, scanFunc func(), launcherScanFunc func(launcherName string), selfUpdateCheckFunc func(), selfUpdateApplyFunc func(ctx context.Context) error, restartFunc func() error) (*http.Server, error) {
	// Store callbacks on State so Routes can register them with auth middleware
	s.scanAllFunc = scanFunc
	s.scanLauncherFunc = launcherScanFunc
	s.selfUpdateCheckFunc = selfUpdateCheckFunc
	s.SetSelfUpdateActions(selfUpdateApplyFunc, restartFunc)

	mux := http.NewServeMux()
	s.Routes(mux)

	// 主题选择统一委托给 getFrontendThemeDir()，转换为绝对路径供 SPAFallbackMiddleware 使用
	staticDir := filepath.Join(s.ProjectRoot, s.getFrontendThemeDir())
	handler := SPAFallbackMiddleware(mux, staticDir)

	handler = SecurityMiddleware(handler)

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	return srv, srv.ListenAndServe()
}
