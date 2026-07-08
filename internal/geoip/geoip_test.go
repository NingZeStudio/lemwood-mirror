package geoip

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRequiresInitBeforeLookup 验证未初始化时 Lookup 返回 nil 而不是 panic。
func TestRequiresInitBeforeLookup(t *testing.T) {
	Close()
	if got := Lookup("1.2.3.4"); got != nil {
		t.Fatalf("Lookup before Init = %v, want nil", got)
	}
	if got := Lookup("240e:3b7:3272:d8d0:db09:c067:8d59:539e"); got != nil {
		t.Fatalf("Lookup v6 before Init = %v, want nil", got)
	}
}

// TestParseRegionFormat 验证 region 字符串解析与 0 占位转换。
func TestParseRegionFormat(t *testing.T) {
	info := parseRegion("中国|0|上海|上海|电信")
	if info == nil {
		t.Fatal("parseRegion returned nil")
	}
	if info.Country != "中国" || info.Region != "" || info.Province != "上海" ||
		info.City != "上海" || info.ISP != "电信" {
		t.Fatalf("parseRegion unexpected: %+v", info)
	}
	if parseRegion("") != nil {
		t.Fatal("empty region should return nil")
	}
}

// TestInitIntegration 真实下载并加载 v4/v6 xdb 后查询一个 IPv4 与一个 IPv6 地址。
// 需联网下载约 46MB，默认跳过；设置 GEOIP_RUN_INTEGRATION=1 时启用。
func TestInitIntegration(t *testing.T) {
	if os.Getenv("GEOIP_RUN_INTEGRATION") != "1" {
		t.Skip("GEOIP_RUN_INTEGRATION!=1, skipping integration test")
	}
	dir := t.TempDir()
	Init(
		filepath.Join(dir, "ip2region_v4.xdb"), "",
		filepath.Join(dir, "ip2region_v6.xdb"), "",
	)
	t.Cleanup(Close)

	if v4 := Lookup("8.8.8.8"); v4 == nil || v4.Country == "" {
		t.Fatalf("Lookup IPv4 8.8.8.8 = %+v, want non-empty country", v4)
	}

	if v6 := Lookup("240e:3b7:3272:d8d0:db09:c067:8d59:539e"); v6 == nil || v6.Country == "" {
		t.Fatalf("Lookup IPv6 = %+v, want non-empty country", v6)
	}
}