package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	gh "lemwood_mirror/internal/github"
	"lemwood_mirror/internal/version"
)

type Channel string

const (
	ChannelNotify  Channel = "notify"
	ChannelRelease Channel = "release"
	ChannelPreview Channel = "preview"
)

type TagInfo struct {
	Name      string    `json:"name"`
	Stable    bool      `json:"stable"`
	Published time.Time `json:"published"`
}

type Status struct {
	Enabled           bool      `json:"enabled"`
	RepoURL           string    `json:"repo_url"`
	Channel           string    `json:"channel"`
	CurrentVersion    string    `json:"current_version"`
	LatestVersion     string    `json:"latest_version"`
	HasUpdate         bool      `json:"has_update"`
	CanApply          bool      `json:"can_apply"`
	PendingRestart    bool      `json:"pending_restart"`
	LastCheckedAt     time.Time `json:"last_checked_at"`
	LastAppliedAt     time.Time `json:"last_applied_at"`
	LastCheckError    string    `json:"last_check_error,omitempty"`
	LastApplyError    string    `json:"last_apply_error,omitempty"`
	LastApplyMessage  string    `json:"last_apply_message,omitempty"`
	AvailableVersions []TagInfo `json:"available_versions,omitempty"`
}

type Config struct {
	Enabled       bool
	RepoURL       string
	Channel       string
	AutoRestart   bool
	ProxyURL      string
	AssetProxyURL string
}

type Manager struct {
	client         *gh.Client
	currentVersion string
	binaryPath     string
	mu             sync.RWMutex
	status         Status
	applyMu        sync.Mutex
	httpClient     *http.Client
	autoRestart    bool
	onRestart      func() error
}

func NewManager(client *gh.Client, currentVersion, binaryPath string, cfg Config) *Manager {
	m := &Manager{
		client:         client,
		currentVersion: normalizeVersion(currentVersion),
		binaryPath:     binaryPath,
		status: Status{
			Enabled:        cfg.Enabled,
			RepoURL:        cfg.RepoURL,
			Channel:        normalizeChannel(cfg.Channel),
			CurrentVersion: normalizeVersion(currentVersion),
		},
		httpClient:  buildHTTPClient(cfg.ProxyURL, cfg.AssetProxyURL),
		autoRestart: cfg.AutoRestart,
	}
	return m
}

func buildHTTPClient(proxyURL, assetProxyURL string) *http.Client {
	proxy := assetProxyURL
	if proxy == "" {
		proxy = proxyURL
	}
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	}
	if proxy != "" {
		transport.Proxy = func(_ *http.Request) (*url.URL, error) {
			return url.Parse(proxy)
		}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Minute,
	}
}

func (m *Manager) UpdateConfig(cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Enabled = cfg.Enabled
	m.status.RepoURL = cfg.RepoURL
	m.status.Channel = normalizeChannel(cfg.Channel)
	m.httpClient = buildHTTPClient(cfg.ProxyURL, cfg.AssetProxyURL)
	m.autoRestart = cfg.AutoRestart
}

func (m *Manager) SetOnRestart(fn func() error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRestart = fn
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status := m.status
	if len(m.status.AvailableVersions) > 0 {
		status.AvailableVersions = append([]TagInfo(nil), m.status.AvailableVersions...)
	}
	return status
}

func (m *Manager) Check(ctx context.Context) (Status, error) {
	m.mu.RLock()
	cfg := Config{
		Enabled: m.status.Enabled,
		RepoURL: m.status.RepoURL,
		Channel: m.status.Channel,
	}
	prev := m.status
	m.mu.RUnlock()

	if !cfg.Enabled {
		status := m.setCheckResult(Status{
			Enabled:        false,
			RepoURL:        cfg.RepoURL,
			Channel:        normalizeChannel(cfg.Channel),
			CurrentVersion: m.currentVersion,
			LastCheckedAt:  time.Now(),
		}, "")
		return status, nil
	}
	if cfg.RepoURL == "" {
		err := fmt.Errorf("self update repo url is empty")
		return m.setCheckError(err), err
	}

	owner, repo, err := gh.ParseOwnerRepo(cfg.RepoURL)
	if err != nil {
		return m.setCheckError(err), err
	}

	tags, resp, err := m.client.ListTags(ctx, owner, repo, 30)
	if err != nil {
		gh.BackoffIfRateLimited(resp)
		return m.setCheckError(err), err
	}

	available := make([]TagInfo, 0, len(tags))
	for _, tag := range tags {
		name := normalizeVersion(tag.GetName())
		if name == "" {
			continue
		}
		available = append(available, TagInfo{
			Name:   name,
			Stable: version.IsStable(name),
		})
	}

	sort.SliceStable(available, func(i, j int) bool {
		return version.Compare(available[i].Name, available[j].Name) > 0
	})

	latest := pickLatest(available, normalizeChannel(cfg.Channel))
	status := Status{
		Enabled:           cfg.Enabled,
		RepoURL:           cfg.RepoURL,
		Channel:           normalizeChannel(cfg.Channel),
		CurrentVersion:    m.currentVersion,
		LatestVersion:     latest,
		HasUpdate:         latest != "" && version.Compare(latest, m.currentVersion) > 0,
		CanApply:          normalizeChannel(cfg.Channel) != string(ChannelNotify) && latest != "" && version.Compare(latest, m.currentVersion) > 0,
		PendingRestart:    prev.PendingRestart,
		LastCheckedAt:     time.Now(),
		LastAppliedAt:     prev.LastAppliedAt,
		AvailableVersions: available,
		LastApplyError:    prev.LastApplyError,
		LastApplyMessage:  prev.LastApplyMessage,
	}
	return m.setCheckResult(status, ""), nil
}

func (m *Manager) MarkApplied(version string, message string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.CurrentVersion = normalizeVersion(version)
	m.status.LatestVersion = normalizeVersion(version)
	m.status.HasUpdate = false
	m.status.CanApply = false
	m.status.PendingRestart = true
	m.status.LastAppliedAt = time.Now()
	m.status.LastApplyError = ""
	m.status.LastApplyMessage = message
	return m.status
}

func (m *Manager) SetApplyError(err error) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.LastApplyError = err.Error()
	m.status.LastApplyMessage = ""
	return m.status
}

func (m *Manager) ClearPendingRestart() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.PendingRestart = false
	m.status.LastApplyError = ""
	m.status.LastApplyMessage = ""
}

func (m *Manager) Apply(ctx context.Context) (Status, error) {
	if !m.applyMu.TryLock() {
		return m.Status(), fmt.Errorf("更新正在应用中，请勿重复操作")
	}
	defer m.applyMu.Unlock()

	m.mu.RLock()
	canApply := m.status.CanApply
	latestVersion := m.status.LatestVersion
	repoURL := m.status.RepoURL
	httpClient := m.httpClient
	m.mu.RUnlock()

	if !canApply {
		return m.Status(), fmt.Errorf("当前状态下不可应用更新")
	}

	owner, repo, err := gh.ParseOwnerRepo(repoURL)
	if err != nil {
		status := m.SetApplyError(err)
		return status, err
	}

	release, resp, err := m.client.GetReleaseByTag(ctx, owner, repo, latestVersion)
	if err != nil {
		gh.BackoffIfRateLimited(resp)
		status := m.SetApplyError(err)
		return status, err
	}

	asset, isArchive := findPlatformAsset(release)
	if asset == nil {
		err := fmt.Errorf("未找到匹配当前平台 (%s/%s) 的资产", runtime.GOOS, runtime.GOARCH)
		status := m.SetApplyError(err)
		return status, err
	}

	if err := downloadAndReplace(ctx, httpClient, asset, m.binaryPath, isArchive); err != nil {
		status := m.SetApplyError(err)
		return status, err
	}

	status := m.MarkApplied(latestVersion, fmt.Sprintf("已从 %s 下载并安装资产 %s", latestVersion, asset.GetName()))

	m.mu.RLock()
	autoRestart := m.autoRestart
	onRestart := m.onRestart
	m.mu.RUnlock()

	if autoRestart && onRestart != nil {
		m.ClearPendingRestart()
		go func() {
			log.Printf("自更新: 自动重启已启用，正在重启...")
			if err := onRestart(); err != nil {
				log.Printf("自更新: 自动重启失败: %v", err)
			}
		}()
	}

	return status, nil
}

func (m *Manager) BinaryPath() string {
	return m.binaryPath
}

func (m *Manager) setCheckError(err error) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.LastCheckedAt = time.Now()
	m.status.LastCheckError = err.Error()
	m.status.HasUpdate = false
	m.status.CanApply = false
	return m.status
}

func (m *Manager) setCheckResult(status Status, errMsg string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	status.PendingRestart = m.status.PendingRestart
	status.LastAppliedAt = m.status.LastAppliedAt
	status.LastApplyError = m.status.LastApplyError
	status.LastApplyMessage = m.status.LastApplyMessage
	status.LastCheckError = errMsg
	m.status = status
	return m.status
}

func pickLatest(tags []TagInfo, channel string) string {
	for _, tag := range tags {
		if channel == string(ChannelRelease) && !tag.Stable {
			continue
		}
		return tag.Name
	}
	return ""
}

func normalizeChannel(channel string) string {
	switch Channel(channel) {
	case ChannelRelease:
		return string(ChannelRelease)
	case ChannelPreview:
		return string(ChannelPreview)
	default:
		return string(ChannelNotify)
	}
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "dev"
	}
	return version
}

func ReplaceTargetPath(binaryPath string) string {
	return filepath.Clean(binaryPath)
}

func findPlatformAsset(release *gh.RepositoryRelease) (*gh.ReleaseAsset, bool) {
	patterns := platformPatterns()
	suffixes := []string{"", ".tar.gz", ".tgz", ".zip"}

	var archiveFallback *gh.ReleaseAsset
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.GetName())
		for _, pat := range patterns {
			matched := false
			for _, suf := range suffixes {
				if name == pat+suf || strings.Contains(name, "-"+pat+suf) || strings.Contains(name, "_"+pat+suf) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			if isArchiveAsset(name) {
				if archiveFallback == nil {
					archiveFallback = asset
				}
				continue
			}
			return asset, false
		}
	}

	if archiveFallback != nil {
		return archiveFallback, true
	}
	return nil, false
}

func isArchiveAsset(name string) bool {
	return strings.HasSuffix(name, ".tar.gz") ||
		strings.HasSuffix(name, ".tgz") ||
		strings.HasSuffix(name, ".zip")
}

func platformPatterns() []string {
	goos := strings.ToLower(runtime.GOOS)
	goarch := strings.ToLower(runtime.GOARCH)

	base := []string{
		goos + "-" + goarch,
		goos + "_" + goarch,
		goos + goarch,
	}

	if goos == "linux" && goarch == "arm64" {
		base = append(base, "aarch64")
	}
	if goos == "linux" && goarch == "amd64" {
		base = append(base, "x86_64")
	}

	return base
}

func downloadAndReplace(ctx context.Context, httpClient *http.Client, asset *gh.ReleaseAsset, targetPath string, isArchive bool) error {
	downloadURL := asset.GetBrowserDownloadURL()
	if downloadURL == "" {
		return fmt.Errorf("资产 %s 没有下载链接", asset.GetName())
	}

	log.Printf("自更新: 下载 %s (%d 字节)", asset.GetName(), asset.GetSize())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载返回状态码 %d", resp.StatusCode)
	}

	tmpFile := targetPath + ".download"
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpFile)

	bufWriter := bufio.NewWriterSize(f, 64*1024)
	written, err := io.Copy(bufWriter, io.TeeReader(resp.Body, &progressTracker{
		total:    resp.ContentLength,
		fileName: asset.GetName(),
	}))
	if err != nil {
		f.Close()
		return fmt.Errorf("下载写入失败: %w", err)
	}
	if err := bufWriter.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("刷新缓冲区失败: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}

	if resp.ContentLength > 0 && written != resp.ContentLength {
		return fmt.Errorf("下载字节数不匹配: 期望 %d, 实际 %d", resp.ContentLength, written)
	}

	binaryPath := tmpFile
	if isArchive {
		extracted, err := extractBinaryFromArchive(tmpFile)
		if err != nil {
			return fmt.Errorf("解压失败: %w", err)
		}
		binaryPath = extracted
		defer os.Remove(extracted)
	}

	tmpBin := targetPath + ".new"
	if err := copyFile(binaryPath, tmpBin); err != nil {
		return err
	}
	defer os.Remove(tmpBin)

	if err := os.Chmod(tmpBin, 0o755); err != nil {
		return fmt.Errorf("设置执行权限失败: %w", err)
	}

	if err := renameOrCopy(tmpBin, targetPath); err != nil {
		return fmt.Errorf("替换二进制失败: %w", err)
	}

	log.Printf("自更新: 已替换二进制 %s", targetPath)
	return nil
}

func extractBinaryFromArchive(archivePath string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractFromZip(archivePath)
	}
	return extractFromTarGz(archivePath)
}

func isExtractableBinary(name string) bool {
	base := strings.ToLower(filepath.Base(name))
	if base == "." || base == "" {
		return false
	}
	skipKeywords := []string{"config", "readme", "license", "copying", "changelog", "news", "notice", "authors", "contributors", "thanks", "todo", "install", "man"}
	for _, kw := range skipKeywords {
		if strings.Contains(base, kw) {
			return false
		}
	}
	skipExts := []string{".txt", ".md", ".rst", ".html", ".xml", ".json", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf", ".example", ".sample", ".pdf", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico"}
	for _, ext := range skipExts {
		if strings.HasSuffix(base, ext) {
			return false
		}
	}
	return true
}

func extractFromTarGz(tgzPath string) (string, error) {
	f, err := os.Open(tgzPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		// Zip Slip 防护：拒绝包含 ".." 的非规范化路径，拒绝非法路径
		if strings.Contains(header.Name, "..") || !filepath.IsLocal(header.Name) {
			continue
		}
		if header.Typeflag == tar.TypeReg {
			name := filepath.Base(header.Name)
			if !isExtractableBinary(name) {
				continue
			}
			outPath := tgzPath + ".extracted"
			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return "", err
			}
			defer out.Close()
			if _, err := io.Copy(out, tarReader); err != nil {
				out.Close()
				os.Remove(outPath)
				return "", err
			}
			out.Close()
			return outPath, nil
		}
	}
	return "", fmt.Errorf("在压缩包中未找到可执行文件")
}

func extractFromZip(zipPath string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// Zip Slip 防护：拒绝路径穿越条目
		if strings.Contains(f.Name, "..") || !filepath.IsLocal(f.Name) {
			continue
		}
		name := filepath.Base(f.Name)
		if !isExtractableBinary(name) {
			continue
		}
		outPath := zipPath + ".extracted"
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			rc.Close()
			return "", err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			os.Remove(outPath)
			return "", err
		}
		out.Close()
		rc.Close()
		return outPath, nil
	}
	return "", fmt.Errorf("在压缩包中未找到可执行文件")
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}

func renameOrCopy(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

type progressTracker struct {
	total      int64
	written    int64
	fileName   string
	lastUpdate time.Time
}

func (pt *progressTracker) Write(p []byte) (int, error) {
	n := len(p)
	pt.written += int64(n)
	if time.Since(pt.lastUpdate) > 2*time.Second {
		pt.lastUpdate = time.Now()
		percentage := float64(pt.written) / float64(pt.total) * 100
		log.Printf("自更新下载 %s: %d / %d (%.2f%%)", pt.fileName, pt.written, pt.total, percentage)
	}
	return n, nil
}
