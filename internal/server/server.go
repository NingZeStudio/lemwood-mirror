package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"lemwood_mirror/internal/blacklist"
	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"
	"lemwood_mirror/internal/download_authz"
	"lemwood_mirror/internal/netutil"
	"lemwood_mirror/internal/pow"
	"lemwood_mirror/internal/selfupdate"
	"lemwood_mirror/internal/stats"
	"lemwood_mirror/internal/traffic"
	"lemwood_mirror/internal/version"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var errForbiddenPath = errors.New("forbidden")

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

	// 验证码（已移除极验）→ PoW 挑战 + DB 授权
	powMgr          *pow.Manager
	authzMgr        *download_authz.Manager
	selfUpdate      *selfupdate.Manager
	applySelfUpdate func(ctx context.Context) error
	restartProcess  func() error

	// 扫描回调（在 Routes 中使用）
	scanAllFunc         func()
	scanLauncherFunc    func(launcherName string)
	selfUpdateCheckFunc func()
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

	if cfg.PowEnabled {
		s.powMgr = pow.NewManager(pow.Config{
			Secret:     cfg.PowHMACSecret,
			Cost:       cfg.PowCost,
			KeyLength:  cfg.PowKeyLength,
			Difficulty: cfg.PowDifficulty,
			TTL:        parseDuration(cfg.PowChallengeTTL, 2*time.Minute),
		})
	}
	s.authzMgr = download_authz.NewManager(parseDuration(cfg.DownloadTokenTTL, 5*time.Minute))

	return s
}

// parseDuration 解析 Go duration 字符串，失败或非正回退到 fallback。
func parseDuration(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func (s *State) SetSelfUpdateManager(manager *selfupdate.Manager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selfUpdate = manager
}

func (s *State) SetSelfUpdateActions(apply func(ctx context.Context) error, restart func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applySelfUpdate = apply
	s.restartProcess = restart
}

func (s *State) updateInfoCache(path string, info map[string]any) {
	s.mu.Lock()
	s.infoCache[path] = info
	s.mu.Unlock()
}

func (s *State) UpdateIndex(launcher string, version string, infoPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index[launcher] == nil {
		s.index[launcher] = make(map[string]string)
	}
	s.index[launcher][version] = infoPath

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

func (s *State) TrimLauncherVersions(launcher string, keep int) error {
	keep = config.NormalizeMaxVersions(keep)

	s.mu.RLock()
	launcherVersions := s.index[launcher]
	if len(launcherVersions) == 0 {
		s.mu.RUnlock()
		return nil
	}

	versions := make([]string, 0, len(launcherVersions))
	infoPaths := make(map[string]string, len(launcherVersions))
	for version, infoPath := range launcherVersions {
		versions = append(versions, version)
		infoPaths[version] = infoPath
	}
	s.mu.RUnlock()

	if len(versions) <= keep {
		return nil
	}

	sort.Slice(versions, func(i, j int) bool {
		return version.Compare(versions[i], versions[j]) > 0
	})

	var deleted []string
	for _, version := range versions[keep:] {
		infoPath := infoPaths[version]
		if infoPath == "" {
			continue
		}

		versionDir := filepath.Dir(infoPath)
		if err := removePathUnderBase(s.BasePath, versionDir); err != nil {
			return fmt.Errorf("删除版本 %s 目录失败: %w", version, err)
		}

		s.mu.Lock()
		if currentVersions := s.index[launcher]; currentVersions != nil {
			delete(currentVersions, version)
			if len(currentVersions) == 0 {
				delete(s.index, launcher)
				delete(s.latest, launcher)
			} else {
				s.latest[launcher] = s.pickLatest(currentVersions)
			}
		}
		delete(s.infoCache, infoPath)
		s.mu.Unlock()

		deleted = append(deleted, version)
	}

	if len(deleted) > 0 {
		log.Printf("%s: 已清理旧版本 %s", launcher, strings.Join(deleted, ", "))
	}

	return nil
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

// getFrontendThemeDir 返回前端主题目录（相对路径）。
// v1 已移除，始终使用 web/default 主题。
func (s *State) getFrontendThemeDir() string {
	return filepath.Join("web", "default")
}

func (s *State) Routes(mux *http.ServeMux) {
	// 静态 UI — 使用 v2 主题目录
	staticDir := s.getFrontendThemeDir()
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
			http.ServeFile(w, r, indexPath)
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

	// 下载 - PoW/授权安全处理器
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

		// token 从 query 或 Authorization: Bearer 提取（不再支持路径内嵌 token）
		token := r.URL.Query().Get("token")
		if token == "" {
			if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// 无 token 且 PoW 开启：浏览器走 PoW 验证页，非浏览器返回 JSON 引导走 API PoW 链路。
		// PoW 关闭时无 token 直接放行（无门控，等同旧 captcha_enabled=false 行为）。
		if token == "" && s.Config.PowEnabled {
			if isBrowserRequest(r) {
				http.Redirect(w, r, "/verify?file="+url.QueryEscape(relPath), http.StatusFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":              "verification_required",
				"message":            "Download requires PoW authorization",
				"pow":                true,
				"challenge_endpoint": "/api/v2/downloads/challenge",
			})
			return
		}

		// 校验授权（TTL 内可多次使用，不单次消费）：
		// Chrome/Android 会拦截非用户手势触发的自动下载，用户随后会点真实按钮重试，
		// 因此 token 在有效期内可重复用于下载（对齐 MapleMirror 按过期+max_bytes 绑定）。
		var auth db.DownloadAuthorization
		if token != "" {
			validated, ok := s.authzMgr.Peek(token)
			if !ok {
				if isBrowserRequest(r) && s.Config.PowEnabled {
					http.Redirect(w, r, "/verify?file="+url.QueryEscape(relPath), http.StatusFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "invalid_token",
					"message": "Download token is invalid or expired",
				})
				return
			}
			auth = validated

			// 文件绑定校验：授权绑定的路径必须与请求路径一致
			if auth.FilePath != relPath {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "token_mismatch",
					"message": "Download token does not match requested file",
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

		clientIP := netutil.ExtractClientIP(r)

		switch r.Method {
		case http.MethodHead:
			http.ServeFile(w, r, cleanPath)
			return
		case http.MethodGet:
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		estimatedBytes := traffic.EstimateTransferBytes(info.Size(), r.Header.Get("Range"))
		allowed, _, projectedBytes, reason := traffic.ReserveTraffic(clientIP, estimatedBytes)
		if !allowed {
			message := reason
			if message == "" {
				message = "已超过当日下载流量限制"
			}
			if s.Config.AppealContact != "" {
				message = fmt.Sprintf("%s，如有误封请联系 %s", message, s.Config.AppealContact)
			}
			http.Error(w, message, http.StatusForbidden)
			log.Printf("[防刷墙] 拒绝下载请求: ip=%s path=%s projected=%.2fGB reason=%s", clientIP, relPath, traffic.ToGB(projectedBytes), reason)
			return
		}

		counter := &traffic.CountingWriter{}
		countingWriter := &responseWriterCounter{
			ResponseWriter: w,
			counter:        counter,
		}
		// token-based URL，禁止中间缓存共享。
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		// 强制附件下载：无扩展名文件（如 mirror-linux-amd64）浏览器需要 attachment 才弹下载
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(relPath)))

		http.ServeFile(countingWriter, r, cleanPath)

		// 事件状态表记录（流量与下载次数的统一来源）：
		// served 口径 = 实际写出字节（含中止的部分传输，防刷墙用）；
		// completed 口径 = 完整传输（写出达到预估且状态 200/206，展示用）。
		// 注：multi-range 请求 EstimateTransferBytes 返回整个文件大小，completed 多为 false，属预期。
		// 必须先写事件再 FinalizeTraffic，防刷墙 CheckAndBan 才能读到本次字节。
		completed := counter.Total >= estimatedBytes && isSuccessfulDownloadStatus(countingWriter.statusCode)
		if counter.Total > 0 {
			parts := strings.Split(filepath.ToSlash(relPath), "/")
			var launcher, version, fileName string
			if len(parts) >= 2 {
				launcher = parts[0]
				version = parts[1]
			}
			fileName = filepath.Base(relPath)
			if err := db.RecordDownloadEvent(db.DownloadEvent{
				AuthorizationID: auth.AuthorizationID,
				FilePath:        relPath,
				FileName:        fileName,
				Launcher:        launcher,
				Version:         version,
				ClientIP:        clientIP,
				BytesServed:     counter.Total,
				Completed:       completed,
				StatusCode:      countingWriter.statusCode,
			}); err != nil {
				log.Printf("[流量统计] 记录下载事件失败: %v", err)
			}
		}

		if banned, reason, trafficGB, err := traffic.FinalizeTraffic(clientIP, estimatedBytes, counter.Total); err != nil {
			log.Printf("[防刷墙] 记录流量失败: %v", err)
		} else if banned {
			log.Printf("[防刷墙] IP %s 因 %s 被封禁，当日流量: %.2fGB", clientIP, reason, trafficGB)
		}
	})

	// ============================================================
	// v2 API 端点 (/api/v2/) — 唯一 API 版本，始终注册
	// ============================================================
	// 公共查询
	mux.HandleFunc("/api/v2/launchers", s.handleV2Status)
	mux.HandleFunc("/api/v2/launchers/", s.handleV2LauncherStatus)
	mux.HandleFunc("/api/v2/latest", s.handleV2LatestAll)
	mux.HandleFunc("/api/v2/latest/", s.handleV2LatestLauncher)
	mux.HandleFunc("/api/v2/stats", s.handleV2Stats)
	mux.HandleFunc("/api/v2/auth/2fa/status", s.handleV2Auth2FAStatus)

	// 下载：prepare（CLI/API 直发授权）+ PoW 挑战/授权（替代极验浏览器验证）+ landing
	mux.HandleFunc("/api/v2/downloads/prepare", s.handleV2DownloadPrepare)
	mux.HandleFunc("/api/v2/downloads/landing", s.handleV2DownloadLanding)
	if s.Config.PowEnabled {
		mux.HandleFunc("/api/v2/downloads/challenge", s.handleV2DownloadChallenge)
		mux.HandleFunc("/api/v2/downloads/authorize", s.handleV2DownloadAuthorize)
		mux.HandleFunc("/api/v2/pow/config", s.handleV2PowConfig)
	}

	// 认证 + 扫描（v2 admin 中间件，返回信封格式错误）
	mux.Handle("/api/v2/auth/login", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.handleV2Login)))
	mux.Handle("/api/v2/admin/scans", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.v2AdminMiddleware(s.handleV2ScanAll))))
	mux.Handle("/api/v2/admin/scans/launcher", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.v2AdminMiddleware(s.handleV2ScanLauncher))))

	// 管理后台（v2 admin 中间件）
	mux.Handle("/api/v2/admin/config", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.v2AdminMiddleware(s.handleV2AdminConfig))))
	mux.Handle("/api/v2/admin/blacklist", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.v2AdminMiddleware(s.handleV2AdminBlacklist))))
	mux.Handle("/api/v2/admin/files", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.v2AdminMiddleware(s.handleV2AdminFiles))))
	mux.Handle("/api/v2/admin/files/download", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.v2AdminMiddleware(s.handleV2AdminFileDownload))))
	mux.Handle("/api/v2/admin/self-update/status", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.v2AdminMiddleware(s.handleV2AdminSelfUpdateStatus))))
	mux.Handle("/api/v2/admin/self-update/check", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.v2AdminMiddleware(s.handleV2AdminSelfUpdateCheck))))
	mux.Handle("/api/v2/admin/self-update/apply", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.v2AdminMiddleware(s.handleV2AdminSelfUpdateApply))))
	mux.Handle("/api/v2/admin/self-update/restart", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.v2AdminMiddleware(s.handleV2AdminSelfUpdateRestart))))
	mux.Handle("/api/v2/admin/self-update", s.v2AdminSwitchMiddleware(http.HandlerFunc(s.v2AdminMiddleware(s.handleV2SelfUpdateCheckEndpoint))))

	// Admin UI
	mux.Handle("/admin", s.AdminSwitchMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})))
	mux.Handle("/admin/", s.AdminSwitchMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if containsDotDot(path) {
			http.NotFound(w, r)
			return
		}
		relPath := strings.TrimPrefix(path, "/admin/")

		if relPath == "" || relPath == "index.html" {
			http.ServeFile(w, r, filepath.Join(adminStaticDir, "index.html"))
			return
		}

		fullPath := filepath.Join(adminStaticDir, relPath)
		cleanPath := filepath.Clean(fullPath)
		// 路径安全验证：防止路径穿越
		absBase, _ := filepath.Abs(adminStaticDir)
		absPath, _ := filepath.Abs(cleanPath)
		if !strings.HasPrefix(absPath, absBase) {
			http.NotFound(w, r)
			return
		}

		if info, err := os.Stat(cleanPath); err == nil && !info.IsDir() {
			http.ServeFile(w, r, cleanPath)
			return
		}

		// Fallback to index.html for SPA-like behavior in admin
		http.ServeFile(w, r, filepath.Join(adminStaticDir, "index.html"))
	})))
}

func removePathUnderBase(basePath string, targetPath string) error {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("解析基础路径失败: %w", err)
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("解析目标路径失败: %w", err)
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return fmt.Errorf("校验目标路径失败: %w", err)
	}

	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errForbiddenPath
	}

	if err := os.RemoveAll(absTarget); err != nil {
		return err
	}

	return nil
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
		ip := netutil.ExtractClientIP(r)

		// 检查本地黑名单
		if banned, createdAt, _ := db.GetIPBlacklistInfo(ip); banned {
			log.Printf("[防刷墙] 拒绝来自黑名单 IP 的访问: %s，封禁时间: %s，如有误封请联系 QQ群 %s", ip, createdAt, "1104690837")
			http.Error(w, fmt.Sprintf("Access Denied: Your IP %s was banned at %s. 如有误封，请加 QQ群 1104690837 申诉", ip, createdAt), http.StatusForbidden)
			return
		}

		// 检查外部黑名单
		if blacklist.IsExternalBlacklisted(ip) {
			log.Printf("[防刷墙] 拒绝来自外部黑名单 IP 的访问: %s", ip)
			http.Error(w, fmt.Sprintf("Access Denied: Your IP %s is in the external blacklist. 如有误封，请加 QQ群 1104690837 申诉", ip), http.StatusForbidden)
			return
		}

		// 记录访问（仅在通过黑名单检查后）
		stats.RecordVisit(r)

		path := r.URL.Path
		// 拦截路径遍历尝试
		if containsDotDot(path) {
			log.Printf("安全警告：拦截到来自 %s 的路径遍历尝试，请求路径：%s", r.RemoteAddr, path)
			http.NotFound(w, r)
			return
		}

		// 安全响应头
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		// CORS Headers — 根据 Origin 动态设置，而非通配符 *
		origin := r.Header.Get("Origin")
		if origin != "" {
			// API 端点允许跨域，静态资源同源访问无 Origin 头不受影响
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
		launcher := parts[0]
		version := parts[1]
		s.UpdateIndex(launcher, version, path)
		return nil
	})
}

func (s *State) FixAssetURLs() error {
	if s.Config.DownloadUrlBase == "" {
		return nil
	}

	baseURL := s.Config.DownloadUrlBase
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("解析 download_url_base 失败: %w", err)
	}
	targetDomain := parsedURL.Host
	targetScheme := parsedURL.Scheme

	// 第一阶段：只持有读锁，复制文件路径列表
	s.mu.RLock()
	paths := make([]string, 0, len(s.infoCache))
	for path := range s.infoCache {
		paths = append(paths, path)
	}
	s.mu.RUnlock()

	// 第二阶段：无锁状态下进行文件 IO 和处理
	fixedCount := 0
	for _, infoPath := range paths {
		content, err := os.ReadFile(infoPath)
		if err != nil {
			continue
		}

		var info map[string]interface{}
		if err := json.Unmarshal(content, &info); err != nil {
			continue
		}

		assets, ok := info["assets"].([]interface{})
		if !ok {
			continue
		}

		changed := false
		for _, asset := range assets {
			assetMap, ok := asset.(map[string]interface{})
			if !ok {
				continue
			}

			assetURL, ok := assetMap["url"].(string)
			if !ok {
				continue
			}

			parsedAssetURL, err := url.Parse(assetURL)
			if err != nil {
				continue
			}

			if parsedAssetURL.Host != targetDomain {
				newURL := fmt.Sprintf("%s://%s%s", targetScheme, targetDomain, parsedAssetURL.Path)
				assetMap["url"] = newURL
				changed = true
			}
		}

		if changed {
			newContent, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				continue
			}

			if err := os.WriteFile(infoPath, newContent, 0o644); err != nil {
				log.Printf("修复 %s 的 URL 失败: %v", infoPath, err)
				continue
			}

			// 第三阶段：最小化持有写锁，仅更新缓存
			s.mu.Lock()
			s.infoCache[infoPath] = info
			s.mu.Unlock()

			fixedCount++
		}
	}

	if fixedCount > 0 {
		log.Printf("[URL 统一性检查] 修复了 %d 个 index.json 文件中的下载链接", fixedCount)
	}

	return nil
}

// pickLatest 选择最新版本
func (s *State) pickLatest(versions map[string]string) string {
	if len(versions) == 0 {
		return ""
	}

	var latestFlagged []string
	for v, infoPath := range versions {
		info, exists := s.infoCache[infoPath]
		if !exists {
			continue
		}

		if isLatest, ok := info["is_latest"].(bool); ok && isLatest {
			latestFlagged = append(latestFlagged, v)
		}
	}

	if len(latestFlagged) > 0 {
		latest := latestFlagged[0]
		for _, v := range latestFlagged[1:] {
			if version.Compare(v, latest) > 0 {
				latest = v
			}
		}
		return latest
	}

	var stableVersions []string
	var unstableVersions []string

	for v := range versions {
		if version.IsStable(v) {
			stableVersions = append(stableVersions, v)
		} else {
			unstableVersions = append(unstableVersions, v)
		}
	}

	if len(stableVersions) > 0 {
		latest := stableVersions[0]
		for _, v := range stableVersions[1:] {
			if version.Compare(v, latest) > 0 {
				latest = v
			}
		}
		return latest
	}

	if len(unstableVersions) > 0 {
		latest := unstableVersions[0]
		for _, v := range unstableVersions[1:] {
			if version.Compare(v, latest) > 0 {
				latest = v
			}
		}
		return latest
	}

	return ""
}

type downloadValidationError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *downloadValidationError) Error() string {
	return e.Message
}

type downloadPrepareRequest struct {
	FilePath  string `json:"file_path"`
	ReturnURL string `json:"return_url"`
	Source    string `json:"source"`
}

type downloadTokenResponse struct {
	DownloadToken string `json:"download_token"`
	DownloadURL   string `json:"download_url"`
	LandingURL    string `json:"landing_url"`
}

func writeJSONError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}

func (s *State) validateDownloadFile(filePath string) (string, os.FileInfo, *downloadValidationError) {
	fullPath := filepath.Join(s.BasePath, filePath)
	cleanPath := filepath.Clean(fullPath)
	absBase, _ := filepath.Abs(s.BasePath)
	absPath, _ := filepath.Abs(cleanPath)
	if !strings.HasPrefix(absPath, absBase) {
		return "", nil, &downloadValidationError{
			StatusCode: http.StatusForbidden,
			Code:       "invalid_path",
			Message:    "Invalid file path",
		}
	}

	info, err := os.Stat(cleanPath)
	if err != nil || info.IsDir() {
		return "", nil, &downloadValidationError{
			StatusCode: http.StatusNotFound,
			Code:       "file_not_found",
			Message:    "File not found",
		}
	}

	return cleanPath, info, nil
}

// issueAuthz 签发一条 DB 授权并返回响应。明文 token 仅在响应中返回一次。
// 调用方需提供 clientIP（用于额度/流量归因）、sourceKind（web|api）与 maxBytes（文件大小，用于流量上限）。
func (s *State) issueAuthz(filePath, returnURL, source, flow, clientIP, sourceKind string, maxBytes int64) (downloadTokenResponse, error) {
	token, _, err := s.authzMgr.Issue(download_authz.IssueRequest{
		FilePath:   filePath,
		ReturnURL:  returnURL,
		Source:     source,
		Flow:       flow,
		ClientIP:   clientIP,
		SourceKind: sourceKind,
		MaxBytes:   maxBytes,
		RequestID:  generateRequestID(),
	})
	if err != nil {
		return downloadTokenResponse{}, err
	}
	downloadURL := buildDownloadURL(token, filePath)
	return downloadTokenResponse{
		DownloadToken: token,
		DownloadURL:   downloadURL,
		LandingURL:    fmt.Sprintf("/api/v2/downloads/landing?token=%s", url.QueryEscape(token)),
	}, nil
}

func buildDownloadURL(token, filePath string) string {
	return fmt.Sprintf("/download/%s?token=%s", filePath, url.QueryEscape(token))
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

// responseWriterCounter 包装 http.ResponseWriter 以统计实际写入的字节数
type responseWriterCounter struct {
	http.ResponseWriter
	counter     *traffic.CountingWriter
	statusCode  int
	wroteHeader bool
}

func (rw *responseWriterCounter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriterCounter) Write(p []byte) (int, error) {
	if !rw.wroteHeader {
		rw.statusCode = http.StatusOK
		rw.wroteHeader = true
	}
	n, err := rw.ResponseWriter.Write(p)
	rw.counter.Total += int64(n)
	return n, err
}

func isSuccessfulDownloadStatus(statusCode int) bool {
	return statusCode == http.StatusOK || statusCode == http.StatusPartialContent
}
