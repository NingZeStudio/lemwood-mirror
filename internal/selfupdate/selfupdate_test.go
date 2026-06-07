package selfupdate

import "testing"

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
		if got := isStable(tt.version); got != tt.stable {
			t.Fatalf("isStable(%q) = %v, want %v", tt.version, got, tt.stable)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	if got := compareVersions("v1.2.4", "v1.2.3"); got <= 0 {
		t.Fatalf("expected newer version comparison > 0, got %d", got)
	}
	if got := compareVersions("v1.2.3", "v1.2.3"); got != 0 {
		t.Fatalf("expected equal version comparison = 0, got %d", got)
	}
	if got := compareVersions("v1.2.3-beta.1", "v1.2.3-beta.2"); got >= 0 {
		t.Fatalf("expected beta.1 < beta.2, got %d", got)
	}
	// SemVer: pre-release is lower than the corresponding release
	if got := compareVersions("v1.2.3", "v1.2.3-beta.1"); got <= 0 {
		t.Fatalf("expected v1.2.3 > v1.2.3-beta.1 (pre-release is lower), got %d", got)
	}
	if got := compareVersions("v1.2.3-beta.1", "v1.2.3"); got >= 0 {
		t.Fatalf("expected v1.2.3-beta.1 < v1.2.3 (pre-release is lower), got %d", got)
	}
	if got := compareVersions("v1.2.3-alpha.1", "v1.2.3-beta.1"); got >= 0 {
		t.Fatalf("expected alpha < beta (lexicographic), got %d", got)
	}
	if got := compareVersions("v1.2.4", "v1.2.3-rc1"); got <= 0 {
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

func TestPlatformPatterns(t *testing.T) {
	patterns := platformPatterns()
	if len(patterns) < 2 {
		t.Fatal("platformPatterns should return at least 2 patterns")
	}
	for _, pat := range patterns {
		if pat == "" {
			t.Fatal("platformPatterns should not contain empty pattern")
		}
	}
}

func TestSplitPreRelease(t *testing.T) {
	core, pre := splitPreRelease("1.2.3-beta.1")
	if core != "1.2.3" || pre != "beta.1" {
		t.Fatalf("splitPreRelease(1.2.3-beta.1) = %q, %q; want %q, %q", core, pre, "1.2.3", "beta.1")
	}
	core, pre = splitPreRelease("1.2.3")
	if core != "1.2.3" || pre != "" {
		t.Fatalf("splitPreRelease(1.2.3) = %q, %q; want %q, %q", core, pre, "1.2.3", "")
	}
}

func TestIsExtractableBinary(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"lemwood-mirror", true},
		{"README.md", false},
		{"LICENSE", false},
		{"config.yaml", false},
		{"CHANGELOG.txt", false},
		{"contributors.json", false},
		{"image.png", false},
		{"install.sh", false},
		{"man.1", false},
		{"binary", true},
	}
	for _, tt := range cases {
		if got := isExtractableBinary(tt.name); got != tt.want {
			t.Fatalf("isExtractableBinary(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
