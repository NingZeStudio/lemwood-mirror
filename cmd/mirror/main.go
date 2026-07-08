package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/google/go-github/v50/github"
	"github.com/robfig/cron/v3"
	"lemwood_mirror/internal/assets"
	"lemwood_mirror/internal/auth"
	"lemwood_mirror/internal/blacklist"
	"lemwood_mirror/internal/browser"
	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"
	"lemwood_mirror/internal/downloader"
	gh "lemwood_mirror/internal/github"
	"lemwood_mirror/internal/geoip"
	"lemwood_mirror/internal/gitmirror"
	"lemwood_mirror/internal/selfupdate"
	"lemwood_mirror/internal/server"
	"lemwood_mirror/internal/stats"
	"lemwood_mirror/internal/traffic"
)

var Version = "dev"

type LauncherState struct {
	Name     string
	RepoURL  string
	Version  string
	LastScan time.Time
}

type Scanner struct {
	cfg       *config.Config
	base      string
	s         *server.State
	ghc       *gh.Client
	mu        sync.Mutex
	scanMu    sync.Mutex
	launchers map[string]*LauncherState
}

func NewScanner(cfg *config.Config, base string, s *server.State, ghc *gh.Client) *Scanner {
	launchers := make(map[string]*LauncherState)
	for _, l := range cfg.Launchers {
		ls := &LauncherState{Name: l.Name}
		if v := s.GetLatestVersion(l.Name); v != "" {
			ls.Version = v
			log.Printf("%s: 发现本地版本 %s", l.Name, v)
		}
		launchers[l.Name] = ls
	}
	return &Scanner{
		cfg:       cfg,
		base:      base,
		s:         s,
		ghc:       ghc,
		launchers: launchers,
	}
}

func buildSelfUpdateConfig(cfg *config.Config) selfupdate.Config {
	return selfupdate.Config{
		Enabled:       cfg.SelfUpdateEnabled,
		RepoURL:       cfg.SelfUpdateRepoURL,
		Channel:       cfg.SelfUpdateChannel,
		AutoRestart:   cfg.SelfUpdateAutoRestart,
		ProxyURL:      cfg.ProxyURL,
		AssetProxyURL: cfg.AssetProxyURL,
	}
}

func (sc *Scanner) scanLauncher(lcfg config.LauncherConfig) {
	timeout := time.Duration(sc.cfg.DownloadTimeoutMinutes) * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	effectiveMaxVersions := config.NormalizeMaxVersions(lcfg.MaxVersions)

	repoURL, err := browser.ResolveRepoURL(lcfg.SourceURL, lcfg.RepoSelector)
	if err != nil {
		log.Printf("%s: 解析仓库地址失败: %v", lcfg.Name, err)
		return
	}
	log.Printf("%s: 使用仓库 %s", lcfg.Name, repoURL)

	mode, err := config.NormalizeLauncherMode(lcfg.Mode)
	if err != nil {
		log.Printf("%s: 模式配置无效: %v", lcfg.Name, err)
		return
	}

	if config.ShouldSyncClone(string(mode)) {
		if err := gitmirror.Sync(ctx, sc.s.ProjectRoot, lcfg.Name, repoURL); err != nil {
			log.Printf("%s: 同步 Git 镜像失败: %v", lcfg.Name, err)
		} else {
			log.Printf("%s: Git 镜像同步完成", lcfg.Name)
		}
	}

	if !config.ShouldSyncRelease(string(mode)) {
		return
	}

	owner, repo, err := gh.ParseOwnerRepo(repoURL)
	if err != nil {
		log.Printf("%s: 解析 owner/repo 失败: %v", lcfg.Name, err)
		return
	}

	var releases []*github.RepositoryRelease
	var resp *github.Response

	releases, resp, err = sc.ghc.ListReleasesByPolicy(ctx, owner, repo, effectiveMaxVersions, lcfg.IncludePrerelease)

	if err != nil {
		log.Printf("%s: 获取 release 失败: %v", lcfg.Name, err)
		gh.BackoffIfRateLimited(resp)
		return
	}
	if len(releases) == 0 {
		log.Printf("%s: 未找到符合条件的 release", lcfg.Name)
		return
	}

	for i, rel := range releases {
		version := rel.GetTagName()
		if version == "" {
			version = rel.GetName()
		}

		isLatest := (i == 0)

		if isLatest {
			sc.mu.Lock()
			ls := sc.launchers[lcfg.Name]
			currentVersion := ls.Version
			sc.mu.Unlock()

			if currentVersion != version {
				if err := sc.s.ClearLatestFlags(lcfg.Name); err != nil {
					log.Printf("%s: 清除旧版本 latest 标记失败: %v", lcfg.Name, err)
				}
			}
		}

		downer := downloader.NewDownloader(sc.cfg.DownloadTimeoutMinutes, sc.cfg.ConcurrentDownloads)
		infoPath, err := downer.DownloadLatest(ctx, lcfg.Name, sc.base, sc.cfg.ProxyURL, sc.cfg.AssetProxyURL, sc.cfg.XgetEnabled, sc.cfg.XgetDomain, rel, sc.cfg.ServerAddress, sc.cfg.ServerPort, sc.cfg.DownloadUrlBase, isLatest)
		if err != nil {
			log.Printf("%s: 下载/检查失败: %v", lcfg.Name, err)
			continue
		}

		sc.s.UpdateIndex(lcfg.Name, version, infoPath)

		if isLatest {
			sc.mu.Lock()
			ls := sc.launchers[lcfg.Name]
			ls.RepoURL = repoURL
			ls.Version = version
			ls.LastScan = time.Now()
			sc.mu.Unlock()
			log.Printf("%s: 已更新至 %s", lcfg.Name, version)
		}
	}

	if err := sc.s.TrimLauncherVersions(lcfg.Name, effectiveMaxVersions); err != nil {
		log.Printf("%s: 清理旧版本失败: %v", lcfg.Name, err)
	}
}
func (sc *Scanner) ScanAll() {
	if !sc.scanMu.TryLock() {
		log.Printf("扫描已在进行中，跳过此次执行")
		return
	}
	defer sc.scanMu.Unlock()
	log.Printf("扫描开始")

	if sc.cfg.ExternalBlacklistURL != "" {
		log.Printf("[黑名单同步] 开始同步外部黑名单: %s", sc.cfg.ExternalBlacklistURL)
		go func() {
			if err := blacklist.SyncExternalBlacklist(sc.cfg.ExternalBlacklistURL); err != nil {
				log.Printf("[黑名单同步] 同步外部黑名单失败: %v", err)
			}
		}()
	}

	wg := sync.WaitGroup{}
	for _, lcfg := range sc.cfg.Launchers {
		lcfg := lcfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			sc.scanLauncher(lcfg)
		}()
	}
	wg.Wait()
	log.Printf("扫描完成")
}

func (sc *Scanner) ScanLauncher(launcherName string) {
	if !sc.scanMu.TryLock() {
		log.Printf("扫描已在进行中，跳过此次执行")
		return
	}
	defer sc.scanMu.Unlock()

	var lcfg *config.LauncherConfig
	for i := range sc.cfg.Launchers {
		if sc.cfg.Launchers[i].Name == launcherName {
			lcfg = &sc.cfg.Launchers[i]
			break
		}
	}
	if lcfg == nil {
		log.Printf("未找到启动器: %s", launcherName)
		return
	}

	log.Printf("开始扫描启动器: %s", launcherName)
	sc.scanLauncher(*lcfg)
	log.Printf("启动器 %s 扫描完成", launcherName)
}

func main() {
	projectRoot, _ := os.Getwd()
	if err := assets.SyncEmbedded(projectRoot); err != nil {
		log.Fatalf("释放前端资源失败: %v", err)
	}
	cfg, err := config.LoadConfig(projectRoot)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	base := filepath.Join(projectRoot, cfg.StoragePath)
	if err := server.EnsureDir(base); err != nil {
		log.Fatalf("确保目录存在失败: %v", err)
	}
	if err := db.InitDB(base, cfg); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	traffic.InitTracker(cfg.TrafficLimitGB, cfg.BanRecordFile, cfg.AppealContact, base)
	traffic.InitRepoTracker(cfg.TrafficLimitGB, cfg.BanRecordFile, cfg.AppealContact, base)
	if cfg.TrafficLimitGB > 0 {
		log.Printf("防刷墙已启用: 单IP每日流量限制 %dGB", cfg.TrafficLimitGB)
		if err := traffic.SyncBanRecordNow(); err != nil {
			log.Printf("[防刷墙] 启动同步封禁记录文件失败: %v", err)
		}
	} else {
		log.Println("防刷墙已禁用，仅使用外部黑名单")
	}

	go auth.CleanupTokens()

	// 加载 ip2region 离线 IP 归属地数据库（缺失时自动下载 v4/v6 两份）。
	// 失败不影响主流程，统计会降级为空地域字段。
	v4XdbPath := filepath.Join(base, "ip2region_v4.xdb")
	v6XdbPath := filepath.Join(base, "ip2region_v6.xdb")
	geoip.Init(v4XdbPath, cfg.IP2RegionV4XdbURL, v6XdbPath, cfg.IP2RegionV6XdbURL)

	stats.InitWritePool(4, 1000)

	s := server.NewState(base, projectRoot, cfg)
	if err := s.InitFromDisk(); err != nil {
		log.Printf("初始化索引失败: %v", err)
	}

	if err := s.FixAssetURLs(); err != nil {
		log.Printf("修复资产 URL 失败: %v", err)
	}

	for _, lcfg := range cfg.Launchers {
		keep := config.NormalizeMaxVersions(lcfg.MaxVersions)
		if err := s.TrimLauncherVersions(lcfg.Name, keep); err != nil {
			log.Printf("%s: 启动时清理旧版本失败: %v", lcfg.Name, err)
		}
	}

	ghc := gh.NewClient(cfg.GitHubToken, cfg.ProxyURL)
	selfUpdateManager := selfupdate.NewManager(ghc, Version, os.Args[0], buildSelfUpdateConfig(cfg))
	s.SetSelfUpdateManager(selfUpdateManager)

	scanner := NewScanner(cfg, base, s, ghc)
	go scanner.ScanAll()

	if cfg.SelfUpdateEnabled {
		go func() {
			if _, err := selfUpdateManager.Check(context.Background()); err != nil {
				log.Printf("自更新检查失败: %v", err)
			}
		}()
	}

	c := cron.New()
	_, err = c.AddFunc(cfg.CheckCron, scanner.ScanAll)
	if err != nil {
		log.Fatalf("无效的 cron 表达式 %q: %v", cfg.CheckCron, err)
	}

	// 预热 + 定时刷新统计快照，避免 /api/stats 每次跑聚合查询
	go func() {
		if err := stats.RefreshSnapshot(); err != nil {
			log.Printf("[Stats] 启动预热快照失败: %v", err)
		}
	}()
	if _, err := c.AddFunc("@every 10m", func() {
		if err := stats.RefreshSnapshot(); err != nil {
			log.Printf("[Stats] 定时刷新快照失败: %v", err)
		}
	}); err != nil {
		log.Fatalf("无效的统计快照 cron 表达式: %v", err)
	}

	if cfg.SelfUpdateEnabled && cfg.SelfUpdateCheckCron != "" {
		_, err = c.AddFunc(cfg.SelfUpdateCheckCron, func() {
			if _, checkErr := selfUpdateManager.Check(context.Background()); checkErr != nil {
				log.Printf("定时自更新检查失败: %v", checkErr)
			}
		})
		if err != nil {
			log.Fatalf("无效的 self update cron 表达式 %q: %v", cfg.SelfUpdateCheckCron, err)
		}
	}
	c.Start()
	defer c.Stop()

	applySelfUpdate := func(ctx context.Context) error {
		_, err := selfUpdateManager.Check(ctx)
		if err != nil {
			return fmt.Errorf("检查更新失败: %w", err)
		}
		_, err = selfUpdateManager.Apply(ctx)
		return err
	}

	doRestart := func() error {
		bin := os.Args[0]
		if bin == "" {
			bin = selfUpdateManager.BinaryPath()
		}
		return restartProcess(bin, os.Args, os.Environ())
	}

	selfUpdateManager.SetOnRestart(doRestart)

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	log.Printf("正在启动服务器于 %s", addr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// 使用 channel 跨 goroutine 安全地传递 srv，避免数据竞争
	srvCh := make(chan *http.Server, 1)
	go func() {
		srv, err := server.StartHTTPWithScan(addr, s, scanner.ScanAll, scanner.ScanLauncher, func() {
			if _, checkErr := selfUpdateManager.Check(context.Background()); checkErr != nil {
				log.Printf("手动自更新检查失败: %v", checkErr)
			}
		}, applySelfUpdate, doRestart)
		if err != nil {
			log.Printf("http 服务器出错: %v", err)
		}
		srvCh <- srv
	}()

	<-stop
	log.Println("正在关闭服务...")
	select {
	case srv := <-srvCh:
		if srv != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Printf("HTTP 服务关闭出错: %v", err)
			}
		}
	default:
		log.Println("HTTP 服务器尚未启动，跳过 Shutdown")
	}
	stats.CloseWritePool()
	traffic.CloseTracker()
	log.Println("服务已正常退出")
}
