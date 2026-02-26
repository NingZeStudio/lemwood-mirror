package server

import (
	"fmt"
	"net/http"
	"time"
)

// StartHTTPWithScan 启动带有手动扫描端点的 HTTP 服务器
func StartHTTPWithScan(addr string, s *State, scanFunc func()) error {
	mux := http.NewServeMux()
	s.Routes(mux)

	// 手动扫描端点
	mux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		// 异步触发扫描
		go scanFunc()
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintln(w, "Scan triggered")
	})

	// 应用 SPA fallback 中间件（用于静态资源）
	staticDir := filepath.Join("web", "dist")
	handler := SPAFallbackMiddleware(mux, staticDir)

	// 应用安全中间件
	handler = SecurityMiddleware(handler)

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return srv.ListenAndServe()
}
