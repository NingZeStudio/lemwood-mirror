package selfupdate

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	gh "lemwood_mirror/internal/github"
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
	Enabled     bool
	RepoURL     string
	Channel     string
	AutoRestart bool
}

type Manager struct {
	client         *gh.Client
	currentVersion string
	binaryPath     string
	mu             sync.RWMutex
	status         Status
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
	}
	return m
}

func (m *Manager) UpdateConfig(cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Enabled = cfg.Enabled
	m.status.RepoURL = cfg.RepoURL
	m.status.Channel = normalizeChannel(cfg.Channel)
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
			Stable: isStable(name),
		})
	}

	sort.SliceStable(available, func(i, j int) bool {
		return compareVersions(available[i].Name, available[j].Name) > 0
	})

	latest := pickLatest(available, normalizeChannel(cfg.Channel))
	status := Status{
		Enabled:           cfg.Enabled,
		RepoURL:           cfg.RepoURL,
		Channel:           normalizeChannel(cfg.Channel),
		CurrentVersion:    m.currentVersion,
		LatestVersion:     latest,
		HasUpdate:         latest != "" && compareVersions(latest, m.currentVersion) > 0,
		CanApply:          normalizeChannel(cfg.Channel) != string(ChannelNotify) && latest != "" && compareVersions(latest, m.currentVersion) > 0,
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
	m.mu.RLock()
	canApply := m.status.CanApply
	latestVersion := m.status.LatestVersion
	repoURL := m.status.RepoURL
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

	asset := findPlatformAsset(release)
	if asset == nil {
		err := fmt.Errorf("未找到匹配当前平台 (%s/%s) 的资产", runtime.GOOS, runtime.GOARCH)
		status := m.SetApplyError(err)
		return status, err
	}

	if err := downloadAndReplace(ctx, asset, m.binaryPath); err != nil {
		status := m.SetApplyError(err)
		return status, err
	}

	return m.MarkApplied(latestVersion, fmt.Sprintf("已从 %s 下载并安装资产 %s", latestVersion, asset.GetName())), nil
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

func isStable(v string) bool {
	vLower := strings.ToLower(v)
	keywords := []string{"alpha", "beta", "rc", "snapshot", "pre", "dev", "preview"}
	for _, k := range keywords {
		if strings.Contains(vLower, k) {
			return false
		}
	}
	return true
}

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
			if p1 > p2 {
				return 1
			}
			if p1 < p2 {
				return -1
			}
		} else {
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

func ReplaceTargetPath(binaryPath string) string {
	return filepath.Clean(binaryPath)
}

func findPlatformAsset(release *gh.RepositoryRelease) *gh.ReleaseAsset {
	patterns := platformPatterns()
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.GetName())
		for _, pat := range patterns {
			if strings.Contains(name, pat) {
				return asset
			}
		}
	}
	return nil
}

func platformPatterns() []string {
	goos := strings.ToLower(runtime.GOOS)
	goarch := strings.ToLower(runtime.GOARCH)

	switch goarch {
	case "amd64":
		goarch = "amd64"
	case "arm64":
		goarch = "arm64"
	case "arm":
		goarch = "arm"
	}

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

func downloadAndReplace(ctx context.Context, asset *gh.ReleaseAsset, targetPath string) error {
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载返回状态码 %d", resp.StatusCode)
	}

	tmpPath := targetPath + ".new"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpPath)

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

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("替换二进制失败: %w", err)
	}

	log.Printf("自更新: 已替换二进制 %s", targetPath)
	return nil
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
