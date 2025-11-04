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

	"lemwood_mirror/internal/browser"
	"lemwood_mirror/internal/config"
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
	s := server.NewState(base)
	ghc := gh.NewClient(cfg.GitHubToken)

	var mu sync.Mutex
	var scanMu sync.Mutex
	launchers := make(map[string]*LauncherState)
	for _, l := range cfg.Launchers {
		launchers[l.Name] = &LauncherState{Name: l.Name}
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
				downer := downloader.NewDownloader(cfg.DownloadTimeoutMinutes, cfg.ConcurrentDownloads)
				infoPath, err := downer.DownloadLatest(ctx, lcfg.Name, base, cfg.ProxyURL, cfg.AssetProxyURL, cfg.XgetEnabled, cfg.XgetDomain, rel, cfg.ServerAddress, cfg.ServerPort)
				if err != nil {
					log.Printf("%s: 下载失败: %v", lcfg.Name, err)
					return
				}
				version := rel.GetTagName()
				if version == "" {
					version = rel.GetName()
				}
				s.UpdateIndex(lcfg.Name, version, infoPath)
				mu.Lock()
				ls := launchers[lcfg.Name]
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

	// initial scan
	go scan()

	// cron schedule
	c := cron.New()
	_, err = c.AddFunc(cfg.CheckCron, scan)
	if err != nil {
		log.Fatalf("无效的 cron 表达式 %q: %v", cfg.CheckCron, err)
	}
	c.Start()
	defer c.Stop()

	// http server with manual scan endpoint
	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	log.Printf("Starting server on %s", addr)
	if err := server.StartHTTPWithScan(addr, s, scan); err != nil {
		log.Fatalf("http 服务器出错: %v", err)
	}
}
