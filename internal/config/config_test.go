package config

import "testing"

func TestNormalizeMaxVersions(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "negative uses default", in: -1, want: 3},
		{name: "zero uses default", in: 0, want: 3},
		{name: "positive keeps value", in: 5, want: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeMaxVersions(tt.in); got != tt.want {
				t.Fatalf("NormalizeMaxVersions(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeLauncherMode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    LauncherMode
		wantErr bool
	}{
		{name: "empty defaults to release", in: "", want: LauncherModeRelease},
		{name: "release", in: "release", want: LauncherModeRelease},
		{name: "clone", in: "clone", want: LauncherModeClone},
		{name: "all", in: "all", want: LauncherModeAll},
		{name: "invalid", in: "weird", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeLauncherMode(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeLauncherMode(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeLauncherMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeSelfUpdateChannel(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    SelfUpdateChannel
		wantErr bool
	}{
		{name: "empty defaults to notify", in: "", want: SelfUpdateChannelNotify},
		{name: "notify", in: "notify", want: SelfUpdateChannelNotify},
		{name: "release", in: "release", want: SelfUpdateChannelRelease},
		{name: "preview", in: "preview", want: SelfUpdateChannelPreview},
		{name: "invalid", in: "beta", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeSelfUpdateChannel(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeSelfUpdateChannel(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeSelfUpdateChannel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestShouldSyncByMode(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		wantRelease bool
		wantClone   bool
	}{
		{name: "default", mode: "", wantRelease: true, wantClone: false},
		{name: "release", mode: "release", wantRelease: true, wantClone: false},
		{name: "clone", mode: "clone", wantRelease: false, wantClone: true},
		{name: "all", mode: "all", wantRelease: true, wantClone: true},
		{name: "invalid", mode: "invalid", wantRelease: false, wantClone: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldSyncRelease(tt.mode); got != tt.wantRelease {
				t.Fatalf("ShouldSyncRelease(%q) = %v, want %v", tt.mode, got, tt.wantRelease)
			}
			if got := ShouldSyncClone(tt.mode); got != tt.wantClone {
				t.Fatalf("ShouldSyncClone(%q) = %v, want %v", tt.mode, got, tt.wantClone)
			}
		})
	}
}

func TestDefaultConfigSelfUpdateChannel(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.SelfUpdateChannel != string(SelfUpdateChannelNotify) {
		t.Fatalf("DefaultConfig().SelfUpdateChannel = %q, want %q", cfg.SelfUpdateChannel, SelfUpdateChannelNotify)
	}
}
