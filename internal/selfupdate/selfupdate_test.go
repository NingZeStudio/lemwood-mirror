package selfupdate

import (
	"runtime"
	"strings"
	"lemwood_mirror/internal/version"
	"testing"
)

func TestLooksLikeProxyURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
		desc string
	}{
		{"http://127.0.0.1:7890", true, "本地 HTTP 代理"},
		{"http://127.0.0.1:7890/", true, "带尾斜杠的本地代理"},
		{"http://proxy.example.com:8080", true, "远程 HTTP 代理"},
		{"socks5://127.0.0.1:1080", true, "SOCKS5 代理"},
		{"https://ghproxy.com/", false, "镜像前缀（https 无端口）"},
		{"https://ghproxy.com", false, "无尾斜杠镜像前缀"},
		{"https://mirror.example.com/gh/", false, "带路径的镜像前缀"},
		{"https://proxy.example.com:8443", true, "https 显式端口的代理"},
		{"", false, "空字符串"},
		{"not a url", false, "非法字符串"},
		{"://broken", false, "残缺 URL"},
	}
	for _, tt := range cases {
		if got := looksLikeProxyURL(tt.in); got != tt.want {
			t.Fatalf("looksLikeProxyURL(%q) = %v, want %v (%s)", tt.in, got, tt.want, tt.desc)
		}
	}
}

func TestNormalizeChannel(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: string(ChannelNotify)},
		{in: "notify", want: string(ChannelNotify)},
		{in: "release", want: string(ChannelRelease)},
		{in: "preview", want: string(ChannelPreview)},
		{in: "weird", want: string(ChannelNotify)},
	}

	for _, tt := range tests {
		if got := normalizeChannel(tt.in); got != tt.want {
			t.Fatalf("normalizeChannel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeVersion(t *testing.T) {
	if got := normalizeVersion(""); got != "dev" {
		t.Fatalf("normalizeVersion(empty) = %q, want %q", got, "dev")
	}
	if got := normalizeVersion(" v1.2.3 "); got != "v1.2.3" {
		t.Fatalf("normalizeVersion(trimmed) = %q, want %q", got, "v1.2.3")
	}
}

func TestIsStable(t *testing.T) {
	cases := []struct {
		version string
		stable  bool
	}{
		{version: "v1.2.3", stable: true},
		{version: "1.2.3-beta.1", stable: false},
		{version: "1.2.3-preview", stable: false},
		{version: "1.2.3-rc1", stable: false},
	}

	for _, tt := range cases {
		if got := version.IsStable(tt.version); got != tt.stable {
			t.Fatalf("version.IsStable(%q) = %v, want %v", tt.version, got, tt.stable)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	if got := version.Compare("v1.2.4", "v1.2.3"); got <= 0 {
		t.Fatalf("expected newer version comparison > 0, got %d", got)
	}
	if got := version.Compare("v1.2.3", "v1.2.3"); got != 0 {
		t.Fatalf("expected equal version comparison = 0, got %d", got)
	}
	if got := version.Compare("v1.2.3-beta.1", "v1.2.3-beta.2"); got >= 0 {
		t.Fatalf("expected beta.1 < beta.2, got %d", got)
	}
	// SemVer: pre-release is lower than the corresponding release
	if got := version.Compare("v1.2.3", "v1.2.3-beta.1"); got <= 0 {
		t.Fatalf("expected v1.2.3 > v1.2.3-beta.1 (pre-release is lower), got %d", got)
	}
	if got := version.Compare("v1.2.3-beta.1", "v1.2.3"); got >= 0 {
		t.Fatalf("expected v1.2.3-beta.1 < v1.2.3 (pre-release is lower), got %d", got)
	}
	if got := version.Compare("v1.2.3-alpha.1", "v1.2.3-beta.1"); got >= 0 {
		t.Fatalf("expected alpha < beta (lexicographic), got %d", got)
	}
	if got := version.Compare("v1.2.4", "v1.2.3-rc1"); got <= 0 {
		t.Fatalf("expected v1.2.4 > v1.2.3-rc1, got %d", got)
	}
}

func TestPickLatest(t *testing.T) {
	tags := []TagInfo{
		{Name: "v1.3.0-preview", Stable: false},
		{Name: "v1.2.0", Stable: true},
		{Name: "v1.1.0", Stable: true},
	}

	if got := pickLatest(tags, string(ChannelNotify)); got != "v1.3.0-preview" {
		t.Fatalf("notify latest = %q, want %q", got, "v1.3.0-preview")
	}
	if got := pickLatest(tags, string(ChannelPreview)); got != "v1.3.0-preview" {
		t.Fatalf("preview latest = %q, want %q", got, "v1.3.0-preview")
	}
	if got := pickLatest(tags, string(ChannelRelease)); got != "v1.2.0" {
		t.Fatalf("release latest = %q, want %q", got, "v1.2.0")
	}
}

func TestPlatformAssetName(t *testing.T) {
	name := platformAssetName()
	if name == "" {
		t.Fatal("platformAssetName should not be empty")
	}
	if !strings.HasPrefix(name, "mirror-") {
		t.Fatalf("platformAssetName = %q, want mirror- prefix", name)
	}
	// 运行时平台必须出现在名字里
	goos := strings.ToLower(runtime.GOOS)
	if !strings.Contains(name, goos) {
		t.Fatalf("platformAssetName = %q, should contain goos %q", name, goos)
	}
}

func TestBuildUpdateCandidates(t *testing.T) {
	candidates, err := buildUpdateCandidates("https://github.com/foo/bar", "v1.2.3", "")
	if err != nil {
		t.Fatalf("buildUpdateCandidates error = %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("want 2 candidates (raw + archive), got %d", len(candidates))
	}
	// 第一个候选 = 裸二进制
	if candidates[0].isArchive {
		t.Fatal("first candidate should be raw binary, not archive")
	}
	if !strings.Contains(candidates[0].url, "github.com/foo/bar/releases/download/v1.2.3/mirror-") {
		t.Fatalf("raw binary URL = %q", candidates[0].url)
	}
	// 第二个候选 = 压缩包
	if !candidates[1].isArchive {
		t.Fatal("second candidate should be archive")
	}

	// asset_proxy_url 镜像前缀
	candidates, _ = buildUpdateCandidates("https://github.com/foo/bar", "v1.2.3", "https://mirror.example.com")
	if !strings.HasPrefix(candidates[0].url, "https://mirror.example.com") {
		t.Fatalf("mirror prefix not applied: %q", candidates[0].url)
	}

	// 无效仓库地址应报错
	if _, err := buildUpdateCandidates("not-a-url", "v1.0", ""); err == nil {
		t.Fatal("buildUpdateCandidates with invalid repo should return error")
	}
}

func TestSplitPreRelease(t *testing.T) {
	core, pre := version.SplitPreRelease("1.2.3-beta.1")
	if core != "1.2.3" || pre != "beta.1" {
		t.Fatalf("version.SplitPreRelease(1.2.3-beta.1) = %q, %q; want %q, %q", core, pre, "1.2.3", "beta.1")
	}
	core, pre = version.SplitPreRelease("1.2.3")
	if core != "1.2.3" || pre != "" {
		t.Fatalf("version.SplitPreRelease(1.2.3) = %q, %q; want %q, %q", core, pre, "1.2.3", "")
	}
}

func TestEffectiveRepoURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", DefaultRepoURL},
		{"   ", DefaultRepoURL},
		{"https://github.com/foo/bar", "https://github.com/foo/bar"},
		{"  https://github.com/foo/bar  ", "https://github.com/foo/bar"},
	}
	for _, tt := range cases {
		if got := effectiveRepoURL(tt.in); got != tt.want {
			t.Fatalf("effectiveRepoURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNewManagerAppliesDefaultRepoURL(t *testing.T) {
	m := NewManager(nil, "v1.0.0", "/tmp/bin", Config{Enabled: true, RepoURL: ""})
	if m.Status().RepoURL != DefaultRepoURL {
		t.Fatalf("empty RepoURL should fall back to DefaultRepoURL, got %q", m.Status().RepoURL)
	}

	m2 := NewManager(nil, "v1.0.0", "/tmp/bin", Config{Enabled: true, RepoURL: "https://github.com/foo/bar"})
	if got := m2.Status().RepoURL; got != "https://github.com/foo/bar" {
		t.Fatalf("explicit RepoURL should be preserved, got %q", got)
	}
}

func TestUpdateConfigAppliesDefaultRepoURL(t *testing.T) {
	m := NewManager(nil, "v1.0.0", "/tmp/bin", Config{Enabled: true, RepoURL: "https://github.com/foo/bar"})
	m.UpdateConfig(Config{Enabled: true, RepoURL: ""})
	if got := m.Status().RepoURL; got != DefaultRepoURL {
		t.Fatalf("UpdateConfig with empty RepoURL should fall back to DefaultRepoURL, got %q", got)
	}
}
