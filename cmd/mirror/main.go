package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"lemwood_mirror/internal/auth"
	"lemwood_mirror/internal/browser"
	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"
	"lemwood_mirror/internal/downloader"
	gh "lemwood_mirror/internal/github"
	"lemwood_mirror/internal/server"
)

type LauncherState struct {
	Name     string
	RepoURL  string
	Version  string
	LastScan time.Time
}

func main() {
	projectRoot, _ := os.Getwd()
	cfg, err := config.LoadConfig(projectRoot)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	base := filepath.Join(projectRoot, cfg.StoragePath)
	if err := server.EnsureDir(base); err != nil {
		log.Fatalf("确保目录存在失败: %v", err)
	}
	if err := db.InitDB(base); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 启动 Token 清理协程
	go auth.CleanupTokens()

	s := server.NewState(base, projectRoot, cfg)
	if err := s.InitFromDisk(); err != nil {
		log.Printf("初始化索引失败: %v", err)
	}
	ghc := gh.NewClient(cfg.GitHubToken)

	var mu sync.Mutex
	var scanMu sync.Mutex
	launchers := make(map[string]*LauncherState)
	for _, l := range cfg.Launchers {
		ls := &LauncherState{Name: l.Name}
		// 从磁盘索引中初始化当前版本
		if v := s.GetLatestVersion(l.Name); v != "" {
			ls.Version = v
			log.Printf("%s: 发现本地版本 %s", l.Name, v)
		}
		launchers[l.Name] = ls
	}

	scan := func() {
		if !scanMu.TryLock() {
			log.Printf("扫描已在进行中，跳过此次执行")
			return
		}
		defer scanMu.Unlock()
		log.Printf("扫描开始")
		wg := sync.WaitGroup{}
		for _, lcfg := range cfg.Launchers {
			lcfg := lcfg
			wg.Add(1)
			go func() {
				defer wg.Done()
				timeout := time.Duration(cfg.DownloadTimeoutMinutes) * time.Minute
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				repoURL, err := browser.ResolveRepoURL(lcfg.SourceURL, lcfg.RepoSelector)
				if err != nil {
					log.Printf("%s: 解析仓库地址失败: %v", lcfg.Name, err)
					return
				}
				log.Printf("%s: 使用仓库 %s", lcfg.Name, repoURL)
				owner, repo, err := gh.ParseOwnerRepo(repoURL)
				if err != nil {
					log.Printf("%s: 解析 owner/repo 失败: %v", lcfg.Name, err)
					return
				}
				rel, resp, err := ghc.LatestRelease(ctx, owner, repo)
				if err != nil {
					log.Printf("%s: 获取最新 release 失败: %v", lcfg.Name, err)
					gh.BackoffIfRateLimited(resp)
					return
				}
				version := rel.GetTagName()
				if version == "" {
					version = rel.GetName()
				}
				
				// 检查是否已经是最新版本，避免重复下载
				mu.Lock()
				ls := launchers[lcfg.Name]
				if ls.Version == version {
					mu.Unlock()
					log.Printf("%s: 版本 %s 已是最新，跳过下载", lcfg.Name, version)
					return
				}
				mu.Unlock()
				
				// 清除该启动器所有旧版本的 latest 标记
				if err := s.ClearLatestFlags(lcfg.Name); err != nil {
					log.Printf("%s: 清除旧版本 latest 标记失败: %v", lcfg.Name, err)
				}
				
				downer := downloader.NewDownloader(cfg.DownloadTimeoutMinutes, cfg.ConcurrentDownloads)
				infoPath, err := downer.DownloadLatest(ctx, lcfg.Name, base, cfg.ProxyURL, cfg.AssetProxyURL, cfg.XgetEnabled, cfg.XgetDomain, rel, cfg.ServerAddress, cfg.ServerPort, cfg.DownloadUrlBase, true)
				if err != nil {
					log.Printf("%s: 下载失败: %v", lcfg.Name, err)
					return
				}
				
				s.UpdateIndex(lcfg.Name, version, infoPath)
				mu.Lock()
				ls.RepoURL = repoURL
				ls.Version = version
				ls.LastScan = time.Now()
				mu.Unlock()
				log.Printf("%s: 已更新至 %s", lcfg.Name, version)
			}()
		}
		wg.Wait()
		log.Printf("扫描完成")
	}

	// 初始扫描
	go scan()

	// 定时任务
	c := cron.New()
	_, err = c.AddFunc(cfg.CheckCron, scan)
	if err != nil {
		log.Fatalf("无效的 cron 表达式 %q: %v", cfg.CheckCron, err)
	}
	c.Start()
	defer c.Stop()

	// 带有手动扫描端点的 HTTP 服务器
	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	log.Printf("正在启动服务器于 %s", addr)
	if err := server.StartHTTPWithScan(addr, s, scan); err != nil {
		log.Fatalf("http 服务器出错: %v", err)
	}
}
