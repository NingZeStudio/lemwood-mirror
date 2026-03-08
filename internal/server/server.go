package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"lemwood_mirror/internal/auth"
	"lemwood_mirror/internal/blacklist"
	"lemwood_mirror/internal/captcha"
	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"
	"lemwood_mirror/internal/download_token"
	"lemwood_mirror/internal/stats"
	"lemwood_mirror/internal/traffic"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
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

	// 验证码
	captchaValidator  *captcha.Validator
	downloadTokenMgr *download_token.Manager
}

func NewState(base string, projectRoot string, cfg *config.Config) *State {
	s := &State{
		BasePath:    base,
		ProjectRoot: projectRoot,
		Config:      cfg,
		index:       make(map[string]map[string]string),
		latest:      make(map[string]string),
		infoCache:   make(map[string]map[string]interface{}),

		loginAttempts: make(map[string]int),
		loginLocks:    make(map[string]time.Time),
	}

	if cfg.CaptchaAppId != "" && cfg.CaptchaSecretKey != "" {
		s.captchaValidator = captcha.NewValidator(cfg.CaptchaAppId, cfg.CaptchaSecretKey)
	}
	s.downloadTokenMgr = download_token.NewManager()

	return s
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

	// 根路径处理器 - 处理静态文件和 SPA fallback
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		
		// 根路径和 index.html
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

		// 检查是否是静态文件
		relPath := strings.TrimPrefix(path, "/")
		if relPath != "" {
			fullPath := filepath.Join(staticDir, relPath)
			cleanPath := filepath.Clean(fullPath)
			
			// 安全检查
			absBase, _ := filepath.Abs(staticDir)
			absPath, _ := filepath.Abs(cleanPath)
			if strings.HasPrefix(absPath, absBase) {
				if info, err := os.Stat(cleanPath); err == nil && !info.IsDir() {
					http.ServeFile(w, r, cleanPath)
					return
				}
			}
		}

		// SPA fallback: 其他所有情况返回 index.html
		indexPath := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}

		http.NotFound(w, r)
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

		// 验证码检查
		if s.Config.CaptchaEnabled && s.captchaValidator != nil {
			// 尝试从查询参数获取 token
			token := r.URL.Query().Get("token")
			var filePath string

			if token == "" {
				// 尝试从路径中提取 token: /download/(token)/文件路径
				parts := strings.SplitN(relPath, "/", 2)
				if len(parts) == 2 {
					potentialToken := parts[0]
					potentialPath := parts[1]
					
					// 检查这个 token 是否有效
					if _, valid := s.downloadTokenMgr.Peek(potentialToken); valid {
						token = potentialToken
						filePath = potentialPath
					}
				}
			}

			if token == "" {
				// 没有 token，检查是否是浏览器请求
				if isBrowserRequest(r) {
					s.serveVerifyPage(w, r, relPath)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":      "verification_required",
					"message":    "Download requires captcha verification",
					"captcha":    true,
					"app_id":     s.Config.CaptchaAppId,
				})
				return
			}

			tokenPath, valid := s.downloadTokenMgr.Validate(token)
			if !valid {
				if isBrowserRequest(r) {
					s.serveVerifyPage(w, r, relPath)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":      "invalid_token",
					"message":    "Download token is invalid or expired",
					"captcha":    true,
					"app_id":     s.Config.CaptchaAppId,
				})
				return
			}

			// 确定最终的文件路径
			if filePath != "" {
				// 使用从路径中提取的文件路径
				relPath = filePath
			}
			// 否则使用 token 中存储的路径

			if tokenPath != relPath {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":      "token_mismatch",
					"message":    "Download token does not match requested file",
				})
				return
			}
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

		// 获取客户端IP用于流量统计
		clientIP := getClientIPFromRequest(r)

		// 记录下载流量
		fileSize := info.Size()
		if err := traffic.RecordTraffic(clientIP, fileSize); err != nil {
			log.Printf("[防刷墙] 记录流量失败: %v", err)
		}

		// 检查是否需要封禁
		if banned, reason, trafficGB := traffic.CheckAndBan(clientIP); banned {
			log.Printf("[防刷墙] IP %s 因 %s 被封禁，当日流量: %.2fGB", clientIP, reason, trafficGB)
			http.Error(w, fmt.Sprintf("Access Denied: %s。如有误封，请联系 %s", reason, s.Config.AppealContact), http.StatusForbidden)
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
	mux.HandleFunc("/api/captcha/config", s.handleCaptchaConfig)
	mux.HandleFunc("/api/download/verify", s.handleDownloadVerify)

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

		// 检查本地黑名单
		if db.IsIPBlacklisted(ip) {
			log.Printf("[防刷墙] 拒绝来自黑名单 IP 的访问: %s，如有误封请联系 %s", ip, "QQ群 964498276")
			http.Error(w, "Access Denied。如有误封，请联系QQ群 964498276", http.StatusForbidden)
			return
		}

		// 检查外部黑名单
		if blacklist.IsExternalBlacklisted(ip) {
			log.Printf("[防刷墙] 拒绝来自外部黑名单 IP 的访问: %s", ip)
			http.Error(w, "Access Denied。如有误封，请联系QQ群 964498276", http.StatusForbidden)
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
	data, err := stats.GetStats(s.BasePath)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("获取统计数据失败: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *State) handleCaptchaConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled": s.Config.CaptchaEnabled,
		"app_id":  s.Config.CaptchaAppId,
	})
}

func (s *State) handleDownloadVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.Config.CaptchaEnabled || s.captchaValidator == nil {
		http.Error(w, "Captcha not enabled", http.StatusBadRequest)
		return
	}

	var req struct {
		LotNumber     string `json:"lot_number"`
		CaptchaOutput string `json:"captcha_output"`
		PassToken     string `json:"pass_token"`
		GenTime       string `json:"gen_time"`
		FilePath      string `json:"file_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if req.LotNumber == "" || req.CaptchaOutput == "" || req.PassToken == "" || req.GenTime == "" || req.FilePath == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

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

	result, err := s.captchaValidator.Verify(req.LotNumber, req.CaptchaOutput, req.PassToken, req.GenTime, ip)
	if err != nil {
		log.Printf("验证码验证失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "verification_failed",
			"message": "Failed to verify captcha",
		})
		return
	}

	if result.Result != "success" {
		log.Printf("验证码验证不成功: result=%s, reason=%s", result.Result, result.Reason)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "verification_failed",
			"message": result.Reason,
		})
		return
	}

	fullPath := filepath.Join(s.BasePath, req.FilePath)
	cleanPath := filepath.Clean(fullPath)
	absBase, _ := filepath.Abs(s.BasePath)
	absPath, _ := filepath.Abs(cleanPath)
	if !strings.HasPrefix(absPath, absBase) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_path",
			"message": "Invalid file path",
		})
		return
	}

	info, err := os.Stat(cleanPath)
	if err != nil || info.IsDir() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "file_not_found",
			"message": "File not found",
		})
		return
	}

	downloadToken := s.downloadTokenMgr.Generate(req.FilePath)

	downloadUrl := fmt.Sprintf("/download/%s/%s", downloadToken, req.FilePath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"download_token": downloadToken,
		"download_url":   downloadUrl,
	})
}

func isBrowserRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	userAgent := r.Header.Get("User-Agent")
	
	if strings.Contains(accept, "text/html") {
		return true
	}
	
	if strings.Contains(userAgent, "Mozilla") ||
	   strings.Contains(userAgent, "Chrome") ||
	   strings.Contains(userAgent, "Safari") ||
	   strings.Contains(userAgent, "Edge") ||
	   strings.Contains(userAgent, "Firefox") {
		if !strings.Contains(accept, "application/json") && accept != "" {
			return true
		}
	}
	
	return false
}

func (s *State) serveVerifyPage(w http.ResponseWriter, r *http.Request, filePath string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>安全验证 - 柠枺镜像</title>
    <script src="https://static.geetest.com/v4/gt4.js"></script>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            background: linear-gradient(135deg, #f5f7fa 0%, #e4e8ec 100%);
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            padding: 20px;
        }
        @media (prefers-color-scheme: dark) {
            body { background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%); }
            .card { background: #1f2937; }
            .header { border-bottom-color: #374151; }
            .desc, .file-path { color: #9ca3af; }
            h1 { color: #f3f4f6; }
            .download-url { background: #374151; color: #e5e7eb; }
        }
        .card {
            background: #ffffff;
            border-radius: 16px;
            box-shadow: 0 20px 40px rgba(0,0,0,0.1);
            max-width: 480px;
            width: 100%;
            overflow: hidden;
        }
        .header {
            text-align: center;
            padding: 32px 24px 24px;
            border-bottom: 1px solid #e5e7eb;
        }
        .header svg {
            width: 48px;
            height: 48px;
            color: #3b82f6;
        }
        h1 { margin: 16px 0 8px; font-size: 24px; color: #111827; }
        .desc { color: #6b7280; font-size: 14px; }
        .content {
            padding: 32px 24px;
            display: flex;
            flex-direction: column;
            align-items: center;
            min-height: 150px;
        }
        .footer {
            padding: 16px 24px;
            border-top: 1px solid #e5e7eb;
            text-align: center;
        }
        @media (prefers-color-scheme: dark) {
            .footer { border-top-color: #374151; }
        }
        .file-path { font-size: 12px; color: #6b7280; word-break: break-all; }
        .loading { display: flex; flex-direction: column; align-items: center; gap: 12px; color: #6b7280; }
        .spinner { width: 32px; height: 32px; border: 3px solid #e5e7eb; border-top-color: #3b82f6; border-radius: 50%; animation: spin 1s linear infinite; }
        @keyframes spin { to { transform: rotate(360deg); } }
        .success { display: flex; flex-direction: column; align-items: center; gap: 12px; color: #22c55e; width: 100%; }
        .success svg { width: 48px; height: 48px; }
        .error { display: flex; flex-direction: column; align-items: center; gap: 12px; color: #ef4444; }
        .error svg { width: 48px; height: 48px; }
        .retry-btn { margin-top: 16px; padding: 10px 24px; background: #3b82f6; color: white; border: none; border-radius: 8px; cursor: pointer; font-size: 14px; }
        .retry-btn:hover { opacity: 0.9; }
        .download-url {
            width: 100%;
            margin-top: 16px;
            padding: 12px;
            background: #f3f4f6;
            border-radius: 8px;
            font-size: 12px;
            word-break: break-all;
            color: #374151;
            text-align: left;
        }
        .btn-group {
            display: flex;
            gap: 8px;
            margin-top: 16px;
            width: 100%;
        }
        .btn-group button {
            flex: 1;
            padding: 10px 16px;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            font-size: 14px;
            transition: opacity 0.2s;
        }
        .btn-group button:hover { opacity: 0.9; }
        .btn-primary { background: #3b82f6; color: white; }
        .btn-secondary { background: #e5e7eb; color: #374151; }
        @media (prefers-color-scheme: dark) {
            .btn-secondary { background: #4b5563; color: #f3f4f6; }
        }
        .copied-tip {
            position: fixed;
            top: 20px;
            left: 50%;
            transform: translateX(-50%);
            background: #22c55e;
            color: white;
            padding: 8px 16px;
            border-radius: 8px;
            font-size: 14px;
            opacity: 0;
            transition: opacity 0.3s;
            z-index: 1000;
        }
        .copied-tip.show { opacity: 1; }
    </style>
</head>
<body>
    <div class="copied-tip" id="copied-tip">已复制到剪贴板</div>
    <div class="card">
        <div class="header">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
                <path d="m9 12 2 2 4-4"/>
            </svg>
            <h1>安全验证</h1>
            <p class="desc">请完成验证后开始下载</p>
        </div>
        <div class="content">
            <div id="loading" class="loading">
                <div class="spinner"></div>
                <span>正在加载验证...</span>
            </div>
            <div id="success" class="success" style="display:none;">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
                    <polyline points="22 4 12 14.01 9 11.01"/>
                </svg>
                <span>验证成功</span>
                <div class="download-url" id="download-url"></div>
                <div class="btn-group">
                    <button class="btn-primary" onclick="startDownload()">直接下载</button>
                    <button class="btn-secondary" onclick="copyUrl()">复制链接</button>
                </div>
            </div>
            <div id="error" class="error" style="display:none;">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="12" cy="12" r="10"/>
                    <line x1="15" y1="9" x2="9" y2="15"/>
                    <line x1="9" y1="9" x2="15" y2="15"/>
                </svg>
                <span id="error-msg">验证失败</span>
                <button class="retry-btn" onclick="showCaptcha()">重新验证</button>
            </div>
        </div>
        <div class="footer">
            <p class="file-path">文件: ` + filepath.Base(filePath) + `</p>
        </div>
    </div>
    <script>
        const filePath = "` + filePath + `";
        const captchaId = "` + s.Config.CaptchaAppId + `";
        let captchaObj = null;
        let downloadUrl = "";
        
        function initCaptcha() {
            document.getElementById('loading').style.display = 'flex';
            document.getElementById('success').style.display = 'none';
            document.getElementById('error').style.display = 'none';
            
            initGeetest4({
                captchaId: captchaId,
                product: 'bind'
            }, function(captcha) {
                captchaObj = captcha;
                
                captcha.onReady(function() {
                    document.getElementById('loading').style.display = 'none';
                    captcha.showCaptcha();
                });
                
                captcha.onSuccess(function() {
                    const result = captcha.getValidate();
                    if (result) {
                        verifyCaptcha(result.lot_number, result.captcha_output, result.pass_token, result.gen_time);
                    }
                });
                
                captcha.onError(function(e) {
                    document.getElementById('loading').style.display = 'none';
                    document.getElementById('error').style.display = 'flex';
                    document.getElementById('error-msg').textContent = '验证加载失败: ' + (e.msg || '未知错误');
                });
                
                captcha.onClose(function() {
                    document.getElementById('loading').style.display = 'none';
                    document.getElementById('error').style.display = 'flex';
                    document.getElementById('error-msg').textContent = '用户取消验证';
                });
            });
        }
        
        function showCaptcha() {
            if (captchaObj) {
                document.getElementById('loading').style.display = 'none';
                document.getElementById('success').style.display = 'none';
                document.getElementById('error').style.display = 'none';
                captchaObj.showCaptcha();
            } else {
                initCaptcha();
            }
        }
        
        function verifyCaptcha(lotNumber, captchaOutput, passToken, genTime) {
            document.getElementById('loading').style.display = 'flex';
            
            fetch('/api/download/verify', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    lot_number: lotNumber,
                    captcha_output: captchaOutput,
                    pass_token: passToken,
                    gen_time: genTime,
                    file_path: filePath
                })
            })
            .then(res => res.json())
            .then(data => {
                if (data.download_url) {
                    downloadUrl = data.download_url;
                    document.getElementById('loading').style.display = 'none';
                    document.getElementById('success').style.display = 'flex';
                    document.getElementById('download-url').textContent = window.location.origin + downloadUrl;
                } else {
                    document.getElementById('loading').style.display = 'none';
                    document.getElementById('error').style.display = 'flex';
                    document.getElementById('error-msg').textContent = data.message || '验证失败';
                }
            })
            .catch(err => {
                document.getElementById('loading').style.display = 'none';
                document.getElementById('error').style.display = 'flex';
                document.getElementById('error-msg').textContent = '请求失败: ' + err.message;
            });
        }
        
        function startDownload() {
            if (downloadUrl) {
                window.location.href = downloadUrl;
            }
        }
        
        function copyUrl() {
            if (downloadUrl) {
                const fullUrl = window.location.origin + downloadUrl;
                navigator.clipboard.writeText(fullUrl).then(function() {
                    const tip = document.getElementById('copied-tip');
                    tip.classList.add('show');
                    setTimeout(function() {
                        tip.classList.remove('show');
                    }, 2000);
                }).catch(function(err) {
                    alert('复制失败: ' + err);
                });
            }
        }
        
        initCaptcha();
    </script>
</body>
</html>`
	
	w.Write([]byte(html))
}
