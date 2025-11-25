package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v50/github"
)

type ReleaseInfo struct {
	Launcher    string               `json:"launcher"`
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	PublishedAt time.Time            `json:"published_at"`
	Assets      []ReleaseAssetSimple `json:"assets"`
}

type ReleaseAssetSimple struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int    `json:"size"`
}

type Downloader struct {
	httpClient *http.Client
	semaphore  chan struct{}
}

func NewDownloader(timeoutMinutes, concurrentDownloads int) *Downloader {
	if concurrentDownloads <= 0 {
		concurrentDownloads = 3 // Default to 3 if invalid
	}
	return &Downloader{
		httpClient: &http.Client{Timeout: time.Duration(timeoutMinutes) * time.Minute},
		semaphore:  make(chan struct{}, concurrentDownloads),
	}
}

func (d *Downloader) DownloadLatest(ctx context.Context, launcher string, destBase string, proxyURL string, assetProxyURL string, xgetEnabled bool, xgetDomain string, rel *github.RepositoryRelease, serverAddress string, serverPort int, downloadUrlBase string) (string, error) {
	if rel == nil {
		return "", errors.New("nil release")
	}
	version := rel.GetTagName()
	if version == "" {
		version = rel.GetName()
		if version == "" {
			version = fmt.Sprintf("%d", rel.GetID())
		}
	}
	dir := filepath.Join(destBase, launcher, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}

	var info ReleaseInfo
	info.Launcher = launcher
	info.TagName = rel.GetTagName()
	info.Name = rel.GetName()
	info.PublishedAt = rel.GetPublishedAt().Time
	for _, a := range rel.Assets {
		var downloadURL string
		if downloadUrlBase != "" {
			// If downloadUrlBase is provided, use it directly.
			// Ensure downloadUrlBase has scheme, if not default to http://
			baseURL := downloadUrlBase
			if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
				baseURL = "http://" + baseURL
			}
			baseURL = strings.TrimRight(baseURL, "/")
			downloadURL = fmt.Sprintf("%s/download/%s/%s/%s", baseURL, launcher, version, a.GetName())
		} else if serverAddress != "" {
			downloadURL = FormatDownloadURL(serverAddress, serverPort, "", launcher, version, a.GetName())
		} else {
			publicIP, err := getPublicIP()
			if err != nil {
				log.Printf("Could not get public IP: %v. Falling back to GitHub URL for asset %s", err, a.GetName())
				downloadURL = a.GetBrowserDownloadURL()
			} else {
				downloadURL = FormatDownloadURL("", serverPort, publicIP, launcher, version, a.GetName())
			}
		}
		info.Assets = append(info.Assets, ReleaseAssetSimple{
			Name: a.GetName(),
			URL:  downloadURL,
			Size: a.GetSize(),
		})
	}

	indexPath := filepath.Join(dir, "index.json")
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 index.json 失败: %w", err)
	}
	if err := os.WriteFile(indexPath, b, 0o644); err != nil {
		return "", fmt.Errorf("写入 index.json 失败: %w", err)
	}
	log.Printf("已将版本信息写入 %s", indexPath)

	client := d.httpClient
	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			return "", fmt.Errorf("解析代理URL失败: %w", err)
		}
		// Create a new client for proxy, as the default one might be shared
		client = &http.Client{
			Timeout:   d.httpClient.Timeout,
			Transport: &http.Transport{Proxy: http.ProxyURL(proxy)},
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(rel.Assets))

	for _, asset := range rel.Assets {
		wg.Add(1)
		go func(asset *github.ReleaseAsset) {
			defer wg.Done()
			d.semaphore <- struct{}{}
			defer func() { <-d.semaphore }()

			err := d.downloadAsset(ctx, client, asset, dir, assetProxyURL, xgetEnabled, xgetDomain)
			if err != nil {
				errCh <- err
			}
		}(asset)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return "", err
		}
	}

	return indexPath, nil
}

func getPublicIP() (string, error) {
	resp, err := http.Get("http://ifconfig.me/ip")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(ip), nil
}

func (d *Downloader) downloadAsset(ctx context.Context, client *http.Client, asset *github.ReleaseAsset, dir, assetProxyURL string, xgetEnabled bool, xgetDomain string) error {
	name := asset.GetName()
	outfile := filepath.Join(dir, name)

	if fileInfo, err := os.Stat(outfile); err == nil {
		if fileInfo.Size() == int64(asset.GetSize()) {
			log.Printf("文件 %s 已存在且大小一致，跳过下载。", name)
			return nil
		}
		log.Printf("文件 %s 已存在但大小不一致 (本地: %d, 远程: %d)，将重新下载。", name, fileInfo.Size(), asset.GetSize())
	}

	downloadURL := asset.GetBrowserDownloadURL()
	if downloadURL != "" && assetProxyURL != "" {
		downloadURL = assetProxyURL + downloadURL
	}
	if downloadURL != "" && xgetEnabled && strings.HasPrefix(downloadURL, "https://github.com/") {
		downloadURL = strings.Replace(downloadURL, "https://github.com/", xgetDomain+"/gh/", 1)
	}
	if downloadURL == "" {
		log.Printf("资源 %s 没有下载链接，跳过", name)
		return nil
	}
	if name == "" {
		name = filepath.Base(downloadURL)
	}
	log.Printf("开始下载 %s 到 %s", downloadURL, outfile)

	partial := outfile + ".partial"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}

	var resp *http.Response
	for i := 0; i < 3; i++ {
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		log.Printf("下载 %s 失败，5秒后重试...", downloadURL)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载资源 %s 失败，状态码: %d", downloadURL, resp.StatusCode)
	}

	f, err := os.Create(partial)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(partial)
	}()

	progressWriter := &progressWriter{
		total:      resp.ContentLength,
		fileName:   name,
		lastUpdate: time.Now(),
	}
	if _, err := io.Copy(f, io.TeeReader(resp.Body, progressWriter)); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(partial, outfile); err != nil {
		return err
	}

	log.Printf("完成下载 %s", outfile)
	return nil
}

type progressWriter struct {
	total      int64
	written    int64
	fileName   string
	lastUpdate time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)
	if time.Since(pw.lastUpdate) > 2*time.Second {
		pw.lastUpdate = time.Now()
		percentage := float64(pw.written) / float64(pw.total) * 100
		log.Printf("下载 %s: %d / %d (%.2f%%)", pw.fileName, pw.written, pw.total, percentage)
	}
	return n, nil
}

func FormatDownloadURL(serverAddress string, serverPort int, publicIP string, launcher, version, assetName string) string {
	var host string
	var scheme string = "http"

	if serverAddress != "" {
		host = serverAddress
		// If serverAddress already has scheme, use it and strip it for host processing if needed,
		// but typically serverAddress in config might be just domain or domain:port.
		// Assuming serverAddress is just the address part from the user request "配置文件中的端口为应用网址和api请求端口，地址也是".
		// If serverAddress contains http/https, we should parse it or just use it.
		// However, the requirement says "下载地址格式必须为地址：端口".

		// Simple heuristic: if serverAddress starts with http:// or https://, use that scheme.
		if strings.HasPrefix(serverAddress, "http://") {
			scheme = "http"
			host = strings.TrimPrefix(serverAddress, "http://")
		} else if strings.HasPrefix(serverAddress, "https://") {
			scheme = "https"
			host = strings.TrimPrefix(serverAddress, "https://")
		}
	} else {
		host = publicIP
	}

	// Format host with port if not 80/443
	if serverPort != 80 && serverPort != 443 {
		host = fmt.Sprintf("%s:%d", host, serverPort)
	}

	return fmt.Sprintf("%s://%s/download/%s/%s/%s", scheme, host, launcher, version, assetName)
}
