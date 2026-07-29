package geoip

import (
	"embed"
	"errors"
	"net"
	"strings"
	"sync"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

// 显式引用 embed 包，避免 //go:embed 在部分工具链下触发未使用导入警告。
var _ embed.FS

//go:embed ip2region_v4.xdb
var v4Data []byte

//go:embed ip2region_v6.xdb
var v6Data []byte

var (
	v4Searcher *xdb.Searcher
	v6Searcher *xdb.Searcher
	once       sync.Once
	initErr    error
)

// Init 初始化 IPv4/IPv6 本地查询器（仅执行一次）。
// 数据文件通过 go:embed 内嵌，无需外部文件。
func Init() error {
	once.Do(func() {
		if len(v4Data) == 0 || len(v6Data) == 0 {
			initErr = errors.New("ip2region 数据文件未内嵌")
			return
		}
		v4Searcher, initErr = xdb.NewWithBuffer(xdb.IPv4, v4Data)
		if initErr != nil {
			return
		}
		v6Searcher, initErr = xdb.NewWithBuffer(xdb.IPv6, v6Data)
	})
	return initErr
}

// Lookup 查询 IP 属地，返回国家/省/市。
// 支持 IPv4 和 IPv6；本地解析，无需联网。
func Lookup(ipStr string) (country, region, city string, ok bool) {
	if Init() != nil || v4Searcher == nil || v6Searcher == nil {
		return "", "", "", false
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", "", "", false
	}

	var searcher *xdb.Searcher
	if ip.To4() != nil {
		searcher = v4Searcher
	} else {
		searcher = v6Searcher
	}

	raw, err := searcher.Search(ipStr)
	if err != nil {
		return "", "", "", false
	}

	// ip2region 返回格式：国家|区域|省份|城市|ISP，"0" 表示无数据
	parts := strings.Split(raw, "|")
	if len(parts) < 5 {
		return "", "", "", false
	}

	country = normalize(parts[0])
	region = normalize(parts[2])
	city = normalize(parts[3])

	if country == "" && region == "" && city == "" {
		return "", "", "", false
	}
	return country, region, city, true
}

func normalize(s string) string {
	s = strings.TrimSpace(s)
	if s == "0" || s == "null" || s == "Reserved" || s == "" {
		return ""
	}
	return s
}
