package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"lemwood_mirror/internal/storage"
)

type State struct {
    BasePath string
    // cached status: map[launcher]map[version]infoPath
    mu     sync.RWMutex
    index  map[string]map[string]string
}

func NewState(base string) *State {
	return &State{BasePath: base, index: make(map[string]map[string]string)}
}

func (s *State) UpdateIndex(launcher string, version string, infoPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index[launcher] == nil {
		s.index[launcher] = make(map[string]string)
	}
	s.index[launcher][version] = infoPath
}

func (s *State) Routes(mux *http.ServeMux) {
    // Static UI
    staticDir := filepath.Join("web", "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
    // Downloads
    mux.Handle("/download/", http.StripPrefix("/download/", http.FileServer(http.Dir(s.BasePath))))
    // API endpoints
    mux.HandleFunc("/api/status", s.handleStatus)
    mux.HandleFunc("/api/status/", s.handleLauncherStatus)
    mux.HandleFunc("/api/files", s.handleFiles)
}

// RoutesWithScan adds /api/scan endpoint to trigger a scan callback.
func (s *State) RoutesWithScan(mux *http.ServeMux, scan func()) {
    s.Routes(mux)
    mux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            w.WriteHeader(http.StatusMethodNotAllowed)
            return
        }
        go func(){
            // run scan asynchronously to avoid blocking request
            defer func(){ recover() }()
            scan()
        }()
        w.WriteHeader(http.StatusAccepted)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status":"scan started"})
    })
}

func (s *State) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	indexCopy := make(map[string]map[string]string)
	for k, v := range s.index {
		inner := make(map[string]string)
		for vk, vv := range v { inner[vk] = vv }
		indexCopy[k] = inner
	}
	s.mu.RUnlock()
	// Build response reading info.json content
	resp := make(map[string][]map[string]any)
	for launcher, versions := range indexCopy {
		for version, infoPath := range versions {
			v, err := storage.ReadInfoJSON(infoPath)
			if err != nil {
					log.Printf("read info.json failed for %s %s: %v", launcher, version, err)
					continue
				}
				relPath, err := filepath.Rel(s.BasePath, filepath.Dir(infoPath))
				if err != nil {
					log.Printf("could not get relative path for %s: %v", infoPath, err)
				} else {
					v["download_path"] = filepath.ToSlash(filepath.Join("download", relPath))
				}
			resp[launcher] = append(resp[launcher], v)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *State) handleLauncherStatus(w http.ResponseWriter, r *http.Request) {
	launcherID := filepath.Base(r.URL.Path)
	s.mu.RLock()
	versions, ok := s.index[launcherID]
	s.mu.RUnlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	resp := make([]map[string]any, 0, len(versions))
	for version, infoPath := range versions {
		v, err := storage.ReadInfoJSON(infoPath)
		if err != nil {
			log.Printf("read info.json failed for %s %s: %v", launcherID, version, err)
			continue
		}
		relPath, err := filepath.Rel(s.BasePath, filepath.Dir(infoPath))
		if err != nil {
			log.Printf("could not get relative path for %s: %v", infoPath, err)
		} else {
			v["download_path"] = filepath.ToSlash(filepath.Join("download", relPath))
		}
		resp = append(resp, v)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *State) handleFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" { path = "." }
	n, err := storage.ListTree(s.BasePath, path)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(n)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func StartHTTP(addr string, s *State) error {
    mux := http.NewServeMux()
    s.Routes(mux)
    log.Printf("HTTP server listening on %s", addr)
    return http.ListenAndServe(addr, corsMiddleware(mux))
}

func StartHTTPWithScan(addr string, s *State, scan func()) error {
    mux := http.NewServeMux()
    s.RoutesWithScan(mux, scan)
    log.Printf("HTTP server listening on %s", addr)
    return http.ListenAndServe(addr, corsMiddleware(mux))
}

// Ensure directories exist on startup
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}
