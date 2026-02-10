package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"lemwood_mirror/internal/auth"
	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"
	"lemwood_mirror/internal/stats"
)

type State struct {
	BasePath    string
	ProjectRoot string
	Config      *config.Config
	// 缓存状态：map[launcher]map[version]infoPath
	mu        sync.RWMutex
	index     map[string]map[string]string
	latest    map[string]string
	infoCache map[string]map[string]interface{} // 缓存 index.json 文件内容

	// 登录限制
	loginAttempts   map[string]int       // IP -> 失败次数
	loginLocks      map[string]time.Time // IP -> 解锁时间
	loginAttemptsMu sync.Mutex
}

func NewState(base string, projectRoot string, cfg *config.Config) *State {
	return &State{
		BasePath:    base,
		ProjectRoot: projectRoot,
		Config:      cfg,
		index:       make(map[string]map[string]string),
		latest:      make(map[string]string),
		infoCache:   make(map[string]map[string]interface{}),

		loginAttempts: make(map[string]int),
		loginLocks:    make(map[string]time.Time),
	}
}

func (s *State) UpdateIndex(launcher string, version string, infoPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index[launcher] == nil {
		s.index[launcher] = make(map[string]string)
	}
	s.index[launcher][version] = infoPath

	// 尝试从磁盘读取并更新缓存
	if content, err := os.ReadFile(infoPath); err == nil {
		var info map[string]interface{}
		if err := json.Unmarshal(content, &info); err == nil {
			s.infoCache[infoPath] = info
		}
	}

	s.latest[launcher] = s.pickLatest(s.index[launcher])
	log.Printf("更新启动器 %s 索引: 版本=%s, 最新版本=%s", launcher, version, s.latest[launcher])
}

// GetLatestVersion 获取启动器的最新版本号
func (s *State) GetLatestVersion(launcher string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest[launcher]
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
		// 检查缓存中的 is_latest 字段，如果为 true 才处理
		s.mu.RLock()
		info, exists := s.infoCache[infoPath]
		s.mu.RUnlock()
		
		// 如果缓存存在且 is_latest 为 true，或者缓存不存在（需要读取文件），则处理
		if !exists || (exists && info["is_latest"] == true) {
			if err := s.clearLatestFlag(infoPath); err != nil {
				log.Printf("清除 %s 的 latest 标记失败: %v", infoPath, err)
				// 继续处理其他文件，不返回错误
			}
		}
	}
	
	return nil
}

// clearLatestFlag 清除单个 index.json 文件的 is_latest 标记
func (s *State) clearLatestFlag(infoPath string) error {
	s.mu.RLock()
	info, exists := s.infoCache[infoPath]
	s.mu.RUnlock()
	
	// 如果缓存不存在，读取文件
	if !exists {
		content, err := os.ReadFile(infoPath)
		if err != nil {
			return fmt.Errorf("读取文件失败: %w", err)
		}
		
		var fileInfo map[string]interface{}
		if err := json.Unmarshal(content, &fileInfo); err != nil {
			return fmt.Errorf("解析 JSON 失败: %w", err)
		}
		
		info = fileInfo
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
		
		// 更新缓存
		s.mu.Lock()
		s.infoCache[infoPath] = info
		s.mu.Unlock()
		
		log.Printf("已清除 %s 的 latest 标记", infoPath)
	}
	
	return nil
}

func (s *State) AdminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			// 也可以尝试从 Cookie 获取
			if cookie, err := r.Cookie("admin_token"); err == nil {
				token = cookie.Value
			}
		}

		if token == "" || !auth.ValidateToken(token) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (s *State) AdminSwitchMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.Config.AdminEnabled {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "Admin console is disabled", http.StatusForbidden)
			} else {
				http.Error(w, "Admin console is disabled by administrator", http.StatusForbidden)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *State) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取 IP
	ip := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = strings.Split(xff, ",")[0]
	} else if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip = xri
	}
	if strings.Contains(ip, ":") {
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}
	}

	// 检查锁定状态
	s.loginAttemptsMu.Lock()
	if unlockTime, locked := s.loginLocks[ip]; locked {
		if time.Now().Before(unlockTime) {
			s.loginAttemptsMu.Unlock()
			diff := time.Until(unlockTime).Round(time.Second)
			http.Error(w, fmt.Sprintf("账号已被锁定，请在 %v 后重试", diff), http.StatusForbidden)
			return
		}
		// 锁定已过期
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
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 检查配置中的用户名和密码
	if s.Config.AdminUser == "" || s.Config.AdminPassword == "" {
		http.Error(w, "Admin account not configured", http.StatusInternalServerError)
		return
	}

	// 验证用户名密码
	if req.Username != s.Config.AdminUser || !auth.CheckPasswordHash(req.Password, s.Config.AdminPassword) {
		// 记录失败
		s.loginAttemptsMu.Lock()
		s.loginAttempts[ip]++
		attempts := s.loginAttempts[ip]
		if attempts >= s.Config.AdminMaxRetries {
			lockUntil := time.Now().Add(time.Duration(s.Config.AdminLockDuration) * time.Minute)
			s.loginLocks[ip] = lockUntil
			log.Printf("IP %s 登录失败次数达到上限 (%d)，已锁定至 %v", ip, attempts, lockUntil.Format("2006-01-02 15:04:05"))
			s.loginAttemptsMu.Unlock()
			http.Error(w, fmt.Sprintf("登录失败次数过多，账号已锁定 %d 小时", s.Config.AdminLockDuration/60), http.StatusForbidden)
		} else {
			log.Printf("IP %s 登录失败 (%d/%d)", ip, attempts, s.Config.AdminMaxRetries)
			s.loginAttemptsMu.Unlock()
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		}
		return
	}

	// 验证 2FA
	if s.Config.TwoFactorEnabled {
		if req.OTPCode == "" {
			http.Error(w, "需要验证码", http.StatusUnauthorized)
			return
		}
		if !auth.ValidateTOTP(req.OTPCode, s.Config.TwoFactorSecret) {
			// 验证码错误也算作一次失败尝试
			s.loginAttemptsMu.Lock()
			s.loginAttempts[ip]++
			attempts := s.loginAttempts[ip]
			log.Printf("IP %s 2FA 验证失败 (%d/%d)", ip, attempts, s.Config.AdminMaxRetries)
			s.loginAttemptsMu.Unlock()
			http.Error(w, "验证码错误", http.StatusUnauthorized)
			return
		}
	}

	// 登录成功，重置计数
	s.loginAttemptsMu.Lock()
	delete(s.loginAttempts, ip)
	delete(s.loginLocks, ip)
	s.loginAttemptsMu.Unlock()

	token, err := auth.GenerateToken()
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
	})
}

func (s *State) handle2FAStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"enabled": s.Config.TwoFactorEnabled,
	})
}

func (s *State) handleAdminConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// 返回脱敏后的配置
		cfgCopy := *s.Config
		cfgCopy.AdminPassword = "" // 不返回密码哈希
		json.NewEncoder(w).Encode(cfgCopy)
		return
	}

	if r.Method == http.MethodPost {
		var newCfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// 保持密码不变，除非提供了新密码
		if newCfg.AdminPassword == "" {
			newCfg.AdminPassword = s.Config.AdminPassword
		} else {
			hashed, err := auth.HashPassword(newCfg.AdminPassword)
			if err != nil {
				http.Error(w, "Failed to hash password", http.StatusInternalServerError)
				return
			}
			newCfg.AdminPassword = hashed
		}

		if err := newCfg.Save(s.ProjectRoot); err != nil {
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		s.mu.Lock()
		s.Config = &newCfg
		s.mu.Unlock()

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Config updated")
		return
	}

	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

func (s *State) handleAdminBlacklist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := db.GetIPBlacklist()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		var req struct {
			IP     string `json:"ip"`
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		if err := db.AddIPToBlacklist(req.IP, req.Reason); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	case http.MethodDelete:
		ip := r.URL.Query().Get("ip")
		if ip == "" {
			http.Error(w, "Missing ip parameter", http.StatusBadRequest)
			return
		}
		if err := db.RemoveIPFromBlacklist(ip); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (s *State) handleAdminFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		path := r.URL.Query().Get("path")
		fullPath := filepath.Join(s.BasePath, path)
		
		// 安全检查
		absBase, _ := filepath.Abs(s.BasePath)
		absPath, _ := filepath.Abs(fullPath)
		if !strings.HasPrefix(absPath, absBase) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		entries, err := os.ReadDir(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Directory not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var result []map[string]interface{}
		for _, e := range entries {
			info, _ := e.Info()
			result = append(result, map[string]interface{}{
				"name":     e.Name(),
				"is_dir":   e.IsDir(),
				"size":     info.Size(),
				"mod_time": info.ModTime(),
			})
		}
		json.NewEncoder(w).Encode(result)

	case http.MethodDelete:
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "Missing path", http.StatusBadRequest)
			return
		}
		fullPath := filepath.Join(s.BasePath, path)
		
		// 安全检查
		absBase, _ := filepath.Abs(s.BasePath)
		absPath, _ := filepath.Abs(fullPath)
		if !strings.HasPrefix(absPath, absBase) || absPath == absBase {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		if err := os.RemoveAll(fullPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return

	case http.MethodPost:
		// 文件上传
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "Missing path", http.StatusBadRequest)
			return
		}
		fullPath := filepath.Join(s.BasePath, path)

		// 安全检查
		absBase, _ := filepath.Abs(s.BasePath)
		absPath, _ := filepath.Abs(fullPath)
		if !strings.HasPrefix(absPath, absBase) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// 获取上传的文件
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Failed to get file: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// 确保目录存在
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			http.Error(w, "Failed to create directory", http.StatusInternalServerError)
			return
		}

		// 创建文件（自动替换）
		dst, err := os.Create(fullPath)
		if err != nil {
			http.Error(w, "Failed to create file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "File uploaded")
		return

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (s *State) handleAdminFileDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(s.BasePath, path)

	// 安全检查
	absBase, _ := filepath.Abs(s.BasePath)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absBase) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 检查是否是文件
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// 设置下载响应头
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(fullPath)))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, fullPath)
}

func (s *State) Routes(mux *http.ServeMux) {
	// 静态 UI
	staticDir := filepath.Join("web", "dist")
	adminStaticDir := filepath.Join("web", "admin")

	// 统一静态资源服务函数
	serveStatic := func(w http.ResponseWriter, r *http.Request, baseDir string, prefix string) {
		path := r.URL.Path
		if containsDotDot(path) {
			http.NotFound(w, r)
			return
		}

		relPath := strings.TrimPrefix(path, prefix)
		if relPath == "" || strings.HasSuffix(relPath, "/") {
			http.NotFound(w, r)
			return
		}

		fullPath := filepath.Join(baseDir, relPath)
		cleanPath := filepath.Clean(fullPath)

		// 验证路径安全性和文件类型
		absBase, _ := filepath.Abs(baseDir)
		absPath, _ := filepath.Abs(cleanPath)
		if !strings.HasPrefix(absPath, absBase) {
			log.Printf("安全警告：拦截到来自 %s 的路径逃逸尝试，请求路径：%s", r.RemoteAddr, path)
			http.NotFound(w, r)
			return
		}

		info, err := os.Stat(cleanPath)
		if err != nil || info.IsDir() {
			// 禁止访问目录
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, cleanPath)
	}

	// 静态资源处理器 - /dist/ 和 /assets/
	mux.HandleFunc("/dist/", func(w http.ResponseWriter, r *http.Request) {
		serveStatic(w, r, staticDir, "/dist/")
	})

	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		// assets 通常在 dist/assets 下
		serveStatic(w, r, filepath.Join(staticDir, "assets"), "/assets/")
	})

	// 根路径处理器
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || path == "/index.html" {
			indexPath := filepath.Join(staticDir, "index.html")
			f, err := os.Open(indexPath)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer f.Close()
			d, _ := f.Stat()
			http.ServeContent(w, r, "index.html", d.ModTime(), f)
			return
		}
		
		// 允许访问根目录下的其他合法文件（如 favicon.svg）
		if !strings.Contains(path, "/") || strings.Count(path, "/") == 1 {
			fileName := strings.TrimPrefix(path, "/")
			// 简单的白名单或排除目录
			if fileName != "" && !strings.Contains(fileName, ".") {
				// 如果没有后缀名且不是已知路由，可能是前端路由，返回 index.html
				http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
				return
			}
			
			fullPath := filepath.Join(staticDir, fileName)
			if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
				http.ServeFile(w, r, fullPath)
				return
			}
		}

		// 默认 404
		notFoundPath := filepath.Join(staticDir, "404.html")
		if _, err := os.Stat(notFoundPath); err == nil {
			w.WriteHeader(http.StatusNotFound)
			http.ServeFile(w, r, notFoundPath)
		} else {
			// 如果没有 404.html，返回 index.html 以支持前端路由 fallback
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		}
	})

	// 下载 - 安全处理器
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if containsDotDot(path) {
			http.NotFound(w, r)
			return
		}

		relPath := strings.TrimPrefix(path, "/download/")
		if relPath == "" || strings.HasSuffix(relPath, "/") {
			// 禁止直接访问 /download/ 根目录或任何子目录列表
			http.NotFound(w, r)
			return
		}

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

		// 检查是否为目录
		info, err := os.Stat(cleanPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			log.Printf("访问文件出错：%s, %v", path, err)
			http.NotFound(w, r)
			return
		}
		if info.IsDir() {
			// 禁止目录列表访问
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

		http.ServeFile(w, r, cleanPath)
	})

	// API 端点
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/status/", s.handleLauncherStatus)
	mux.HandleFunc("/api/files", s.handleFiles)
	mux.HandleFunc("/api/latest", s.handleLatestAll)
	mux.HandleFunc("/api/latest/", s.handleLatestLauncher)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/auth/2fa/status", s.handle2FAStatus)

	// Admin API
	mux.Handle("/api/login", s.AdminSwitchMiddleware(http.HandlerFunc(s.handleLogin)))
	mux.Handle("/api/admin/config", s.AdminSwitchMiddleware(http.HandlerFunc(s.AdminMiddleware(s.handleAdminConfig))))
	mux.Handle("/api/admin/blacklist", s.AdminSwitchMiddleware(http.HandlerFunc(s.AdminMiddleware(s.handleAdminBlacklist))))
	mux.Handle("/api/admin/files", s.AdminSwitchMiddleware(http.HandlerFunc(s.AdminMiddleware(s.handleAdminFiles))))
	mux.Handle("/api/admin/files/download", s.AdminSwitchMiddleware(http.HandlerFunc(s.AdminMiddleware(s.handleAdminFileDownload))))

	// Admin UI
	mux.Handle("/admin/", s.AdminSwitchMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		relPath := strings.TrimPrefix(path, "/admin/")
		
		if relPath == "" || relPath == "index.html" {
			http.ServeFile(w, r, filepath.Join(adminStaticDir, "index.html"))
			return
		}

		fullPath := filepath.Join(adminStaticDir, relPath)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			http.ServeFile(w, r, fullPath)
			return
		}
		
		// Fallback to index.html for SPA-like behavior in admin
		http.ServeFile(w, r, filepath.Join(adminStaticDir, "index.html"))
	})))
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
		// 记录访问
		stats.RecordVisit(r)

		// 获取真实 IP（考虑代理）
		ip := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip = strings.Split(xff, ",")[0]
		} else if xri := r.Header.Get("X-Real-IP"); xri != "" {
			ip = xri
		}
		// 移除端口号
		if strings.Contains(ip, ":") {
			if host, _, err := net.SplitHostPort(ip); err == nil {
				ip = host
			}
		}

		// 检查黑名单
		if db.IsIPBlacklisted(ip) {
			log.Printf("拒绝来自黑名单 IP 的访问: %s", ip)
			http.Error(w, "Access Denied", http.StatusForbidden)
			return
		}

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
		
		// 缓存 index.json 文件内容
		content, err := os.ReadFile(path)
		if err == nil {
			var info map[string]interface{}
			if err := json.Unmarshal(content, &info); err == nil {
				s.mu.Lock()
				s.infoCache[path] = info
				s.mu.Unlock()
			}
		}
		return nil
	})
}

// pickLatest 选择最新版本
func (s *State) pickLatest(versions map[string]string) string {
	if len(versions) == 0 {
		return ""
	}

	// 收集所有标记为 is_latest 的版本
	var latestFlagged []string
	for v, infoPath := range versions {
		var info map[string]interface{}
		var exists bool

		// 优先从内存缓存获取
		info, exists = s.infoCache[infoPath]
		if !exists {
			// 如果内存中没有，尝试读取磁盘（通常发生在启动初始化时）
			if content, err := os.ReadFile(infoPath); err == nil {
				if err := json.Unmarshal(content, &info); err == nil {
					// 这里不更新 s.infoCache，因为 pickLatest 可能在持有锁的情况下被调用
					// 而 s.infoCache 的更新已经在 UpdateIndex 或 InitFromDisk 中处理
				}
			}
		}

		if info != nil {
			if isLatest, ok := info["is_latest"].(bool); ok && isLatest {
				latestFlagged = append(latestFlagged, v)
			}
		}
	}

	// 如果有多个版本被标记为 latest（虽然理论上不应该），选择其中版本号最高的一个
	if len(latestFlagged) > 0 {
		latest := latestFlagged[0]
		for _, v := range latestFlagged[1:] {
			if compareVersions(v, latest) > 0 {
				latest = v
			}
		}
		return latest
	}

	// 如果没有找到标记为 is_latest 的版本，使用版本比较作为后备方案
	var stableVersions []string
	var unstableVersions []string

	for v := range versions {
		if isStable(v) {
			stableVersions = append(stableVersions, v)
		} else {
			unstableVersions = append(unstableVersions, v)
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

// isStable 检查版本号是否为稳定版
func isStable(v string) bool {
	vLower := strings.ToLower(v)
	keywords := []string{"alpha", "beta", "rc", "snapshot", "pre", "dev"}
	for _, k := range keywords {
		if strings.Contains(vLower, k) {
			return false
		}
	}
	// 额外检查：如果包含横杠，通常也是非稳定版（如 1.2.3-v1）
	// 但有些启动器可能使用横杠作为正常版本号的一部分，所以以关键词优先
	return true
}

// compareVersions 比较版本
func compareVersions(v1, v2 string) int {
	if v1 == v2 {
		return 0
	}

	v1Clean := strings.TrimPrefix(v1, "v")
	v2Clean := strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1Clean, ".")
	parts2 := strings.Split(v2Clean, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 string
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		if p1 == p2 {
			continue
		}

		n1, err1 := parseFirstInt(p1)
		n2, err2 := parseFirstInt(p2)

		if err1 == nil && err2 == nil {
			if n1 > n2 {
				return 1
			}
			if n1 < n2 {
				return -1
			}
			// 如果数字部分相同，比较整个字符串（例如 2.0.0_beta-1 vs 2.0.0_beta-2）
			if p1 > p2 {
				return 1
			}
			if p1 < p2 {
				return -1
			}
		} else {
			// 如果不能解析为数字，按字符串比较
			if p1 > p2 {
				return 1
			}
			if p1 < p2 {
				return -1
			}
		}
	}
	return 0
}

func parseFirstInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
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
             
             // 先从缓存获取 index.json 内容
             if fileInfo, ok := s.infoCache[p]; ok {
                 for k, val := range fileInfo {
                     // 排除 is_latest 字段
                     if k != "is_latest" {
                         info[k] = val
                     }
                 }
             } else {
                 // 缓存不存在时，读取文件并更新缓存
                 if content, err := os.ReadFile(p); err == nil {
                     var fileInfo map[string]any
                     if err := json.Unmarshal(content, &fileInfo); err == nil {
                         s.infoCache[p] = fileInfo // 更新缓存
                         for k, val := range fileInfo {
                             // 排除 is_latest 字段
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
             
             // 先从缓存获取 index.json 内容
             if fileInfo, ok := s.infoCache[p]; ok {
                 for k, val := range fileInfo {
                     // 排除 is_latest 字段
                     if k != "is_latest" {
                         info[k] = val
                     }
                 }
             } else {
                 // 缓存不存在时，读取文件并更新缓存
                 if content, err := os.ReadFile(p); err == nil {
                     var fileInfo map[string]any
                     if err := json.Unmarshal(content, &fileInfo); err == nil {
                         s.infoCache[p] = fileInfo // 更新缓存
                         for k, val := range fileInfo {
                             // 排除 is_latest 字段
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
