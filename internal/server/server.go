package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"lemwood_mirror/internal/stats"
)

type State struct {
	BasePath string
	// 缓存状态：map[launcher]map[version]infoPath
	mu     sync.RWMutex
	index  map[string]map[string]string
	latest map[string]string
}

func NewState(base string) *State {
	return &State{BasePath: base, index: make(map[string]map[string]string), latest: make(map[string]string)}
}

func (s *State) UpdateIndex(launcher string, version string, infoPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index[launcher] == nil {
		s.index[launcher] = make(map[string]string)
	}
	s.index[launcher][version] = infoPath
	s.latest[launcher] = s.pickLatest(s.index[launcher])
}

func (s *State) RemoveVersion(launcher string, version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index[launcher] == nil {
		return
	}
	delete(s.index[launcher], version)
	s.latest[launcher] = s.pickLatest(s.index[launcher])
}

// ClearLatestFlags 清除指定启动器所有版本的 is_latest 标记
func (s *State) ClearLatestFlags(launcher string) error {
	s.mu.RLock()
	versions, exists := s.index[launcher]
	s.mu.RUnlock()
	
	if !exists {
		return nil // 启动器不存在，无需清除
	}
	
	for _, infoPath := range versions {
		if err := s.clearLatestFlag(infoPath); err != nil {
			log.Printf("清除 %s 的 latest 标记失败: %v", infoPath, err)
			// 继续处理其他文件，不返回错误
		}
	}
	
	return nil
}

// clearLatestFlag 清除单个 index.json 文件的 is_latest 标记
func (s *State) clearLatestFlag(infoPath string) error {
	content, err := os.ReadFile(infoPath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}
	
	var info map[string]interface{}
	if err := json.Unmarshal(content, &info); err != nil {
		return fmt.Errorf("解析 JSON 失败: %w", err)
	}
	
	// 如果存在 is_latest 字段且为 true，则将其设置为 false
	if isLatest, exists := info["is_latest"]; exists && isLatest == true {
		info["is_latest"] = false
		
		// 重新写入文件
		newContent, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化 JSON 失败: %w", err)
		}
		
		if err := os.WriteFile(infoPath, newContent, 0o644); err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}
		
		log.Printf("已清除 %s 的 latest 标记", infoPath)
	}
	
	return nil
}

func (s *State) Routes(mux *http.ServeMux) {
	// 静态 UI
	staticDir := filepath.Join("web", "dist")

	// 安全静态资源处理器
	mux.HandleFunc("/dist/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// 再次检查路径遍历以防万一，尽管中间件会捕获它
		if containsDotDot(path) {
			http.NotFound(w, r)
			return
		}

		relPath := strings.TrimPrefix(path, "/dist/")
		if relPath == "" || relPath == "/" {
			http.NotFound(w, r)
			return
		}

		fullPath := filepath.Join(distDir, relPath)
		cleanPath := filepath.Clean(fullPath)

		// 验证路径是否在 staticDir 内
		absStaticDir, _ := filepath.Abs(staticDir)
		absPath, _ := filepath.Abs(cleanPath)
		if !strings.HasPrefix(absPath, absStaticDir) {
			log.Printf("安全警告：拦截到来自 %s 的路径逃逸尝试，请求路径：%s", r.RemoteAddr, path)
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, cleanPath)
	})

	// 根路径处理器
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}
		if path == "/index.html" {
			// 手动服务内容以避免 http.ServeFile 的 301 重定向
			f, err := os.Open(filepath.Join(staticDir, "index.html"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer f.Close()
			d, _ := f.Stat()
			http.ServeContent(w, r, "index.html", d.ModTime(), f)
			return
		}
		if path == "/404.html" {
			http.ServeFile(w, r, filepath.Join(staticDir, "404.html"))
			return
		}

		log.Printf("安全警告：拦截到来自 %s 的非法根目录访问尝试，请求路径：%s", r.RemoteAddr, path)
		w.WriteHeader(http.StatusNotFound)
		http.ServeFile(w, r, filepath.Join(staticDir, "404.html"))
	})

	// 下载 - 安全处理器
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if containsDotDot(path) {
			http.NotFound(w, r)
			return
		}

		relPath := strings.TrimPrefix(path, "/download/")
		fullPath := filepath.Join(s.BasePath, relPath)
		cleanPath := filepath.Clean(fullPath)

		// 验证路径是否在 BasePath 内
		absBase, _ := filepath.Abs(s.BasePath)
		absPath, _ := filepath.Abs(cleanPath)
		if !strings.HasPrefix(absPath, absBase) {
			log.Printf("安全警告：拦截到来自 %s 的路径逃逸尝试，请求路径：%s", r.RemoteAddr, path)
			http.NotFound(w, r)
			return
		}

		// 记录下载
		parts := strings.Split(filepath.ToSlash(relPath), "/")
		if len(parts) >= 2 {
			launcher := parts[0]
			version := parts[1]
			fileName := filepath.Base(relPath)
			stats.RecordDownload(r, fileName, launcher, version)
		}

		// 检查文件是否存在
		_, err := os.Stat(cleanPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("文件未找到：%s", path)
				http.NotFound(w, r)
				return
			}
			log.Printf("访问文件出错：%s, %v", path, err)
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, cleanPath)
	})

	// API 端点
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/status/", s.handleLauncherStatus)
	mux.HandleFunc("/api/files", s.handleFiles)
	mux.HandleFunc("/api/latest", s.handleLatestAll)
	mux.HandleFunc("/api/latest/", s.handleLatestLauncher)
	mux.HandleFunc("/api/stats", s.handleStats)
}

// containsDotDot 检查路径是否包含 ".." 元素
func containsDotDot(v string) bool {
	if !strings.Contains(v, "..") {
		return false
	}
	for _, ent := range strings.FieldsFunc(v, func(r rune) bool { return r == '/' || r == '\\' }) {
		if ent == ".." {
			return true
		}
	}
	return false
}

func SecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stats.RecordVisit(r)
		path := r.URL.Path
		// 拦截路径遍历尝试
		if containsDotDot(path) {
			log.Printf("安全警告：拦截到来自 %s 的路径遍历尝试，请求路径：%s", r.RemoteAddr, path)
			http.NotFound(w, r)
			return
		}

		// CORS Headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Expose-Headers", "X-Latest-Version, X-Latest-Versions")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *State) InitFromDisk() error {
	base := s.BasePath
	return filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "index.json" {
			return nil
		}
		rel, err := filepath.Rel(base, filepath.Dir(path))
		if err != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 2 {
			return nil
		}
		// 假设目录结构为 launcher/version
		launcher := parts[0]
		version := parts[1]
		s.UpdateIndex(launcher, version, path)
		return nil
	})
}

// pickLatest 选择最新版本
func (s *State) pickLatest(versions map[string]string) string {
	if len(versions) == 0 {
		return ""
	}
	
	// 首先查找标记为 is_latest 的版本
	for v, infoPath := range versions {
		if content, err := os.ReadFile(infoPath); err == nil {
			var info map[string]interface{}
			if err := json.Unmarshal(content, &info); err == nil {
				if isLatest, ok := info["is_latest"].(bool); ok && isLatest {
					return v
				}
			}
		}
	}
	
	// 如果没有找到标记为 is_latest 的版本，使用版本比较作为后备方案
	// 分离稳定版和非稳定版
	var stableVersions []string
	var unstableVersions []string
	
	for v := range versions {
		if strings.Contains(v, "-") {
			unstableVersions = append(unstableVersions, v)
		} else {
			stableVersions = append(stableVersions, v)
		}
	}
	
	// 优先从稳定版中选择最新的
	if len(stableVersions) > 0 {
		latest := stableVersions[0]
		for _, v := range stableVersions[1:] {
			if compareVersions(v, latest) > 0 {
				latest = v
			}
		}
		return latest
	}
	
	// 如果没有稳定版，从非稳定版中选择最新的
	if len(unstableVersions) > 0 {
		latest := unstableVersions[0]
		for _, v := range unstableVersions[1:] {
			if compareVersions(v, latest) > 0 {
				latest = v
			}
		}
		return latest
	}
	
	return ""
}

// compareVersions 比较版本
func compareVersions(v1, v2 string) int {
	v1Clean := strings.TrimPrefix(v1, "v")
	v2Clean := strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1Clean, ".")
	parts2 := strings.Split(v2Clean, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		n1 := 0
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		n2 := 0
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}

func (s *State) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
    
    result := make(map[string][]map[string]any)
    for launcher, versions := range s.index {
        var list []map[string]any
        for v, p := range versions {
             info := map[string]any{
                 "tag_name": v,
             }
             if content, err := os.ReadFile(p); err == nil {
                 var fileInfo map[string]any
                 if err := json.Unmarshal(content, &fileInfo); err == nil {
                     for k, val := range fileInfo {
                         // 排除 is_latest 字段
                         if k != "is_latest" {
                             info[k] = val
                         }
                     }
                 }
             }
             list = append(list, info)
        }
        sort.Slice(list, func(i, j int) bool {
             v1, _ := list[i]["tag_name"].(string)
             v2, _ := list[j]["tag_name"].(string)
             return compareVersions(v1, v2) > 0
        })
        result[launcher] = list
    }
    
	json.NewEncoder(w).Encode(result)
}

func (s *State) handleLauncherStatus(w http.ResponseWriter, r *http.Request) {
	launcher := strings.TrimPrefix(r.URL.Path, "/api/status/")
	s.mu.RLock()
	defer s.mu.RUnlock()
	if versions, ok := s.index[launcher]; ok {
        var list []map[string]any
        for v, p := range versions {
             info := map[string]any{"tag_name": v}
             if content, err := os.ReadFile(p); err == nil {
                 var fileInfo map[string]any
                 if err := json.Unmarshal(content, &fileInfo); err == nil {
                     for k, val := range fileInfo {
                         // 排除 is_latest 字段
                         if k != "is_latest" {
                             info[k] = val
                         }
                     }
                 }
             }
             list = append(list, info)
        }
        sort.Slice(list, func(i, j int) bool {
             v1, _ := list[i]["tag_name"].(string)
             v2, _ := list[j]["tag_name"].(string)
             return compareVersions(v1, v2) > 0
        })
		json.NewEncoder(w).Encode(list)
	} else {
		http.NotFound(w, r)
	}
}

func (s *State) handleFiles(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}

func (s *State) handleLatestAll(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
    
    // 添加 Header X-Latest-Versions
    if b, err := json.Marshal(s.latest); err == nil {
        w.Header().Set("X-Latest-Versions", string(b))
    }
	json.NewEncoder(w).Encode(s.latest)
}

func (s *State) handleLatestLauncher(w http.ResponseWriter, r *http.Request) {
	launcher := strings.TrimPrefix(r.URL.Path, "/api/latest/")
	s.mu.RLock()
	defer s.mu.RUnlock()
	if val, ok := s.latest[launcher]; ok {
        w.Header().Set("X-Latest-Version", val)
		w.Write([]byte(val))
	} else {
		http.NotFound(w, r)
	}
}

func (s *State) handleStats(w http.ResponseWriter, r *http.Request) {
	data, err := stats.GetStats()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("获取统计数据失败: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
