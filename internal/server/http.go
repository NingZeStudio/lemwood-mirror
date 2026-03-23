package server

import (
	"encoding/json"
	"fmt"
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

func StartHTTPWithScan(addr string, s *State, scanFunc func(), launcherScanFunc func(launcherName string)) error {
	mux := http.NewServeMux()
	s.Routes(mux)

	mux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		go scanFunc()
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintln(w, "Scan triggered")
	})

	mux.HandleFunc("/api/scan/launcher", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		var req LauncherScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.Launcher == "" {
			http.Error(w, "launcher is required", http.StatusBadRequest)
			return
		}
		go launcherScanFunc(req.Launcher)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(LauncherScanResponse{
			Status:  "accepted",
			Message: "扫描已触发",
		})
	})

	staticDir := filepath.Join("web", "dist")
	handler := SPAFallbackMiddleware(mux, staticDir)

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
