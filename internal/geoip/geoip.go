package geoip

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

// ip2region 官方仓库提供的离线数据库下载地址（v3 格式，IPv4/IPv6 分别独立 xdb 文件）。
const (
	DefaultV4XdbURL = "https://raw.githubusercontent.com/lionsoul2014/ip2region/master/data/ip2region_v4.xdb"
	DefaultV6XdbURL = "https://raw.githubusercontent.com/lionsoul2014/ip2region/master/data/ip2region_v6.xdb"
)

// Info 表示一次 IP 归属地查询的结构化结果。
// ip2region 的 region 字符串格式为 "国家|区域|省份|城市|ISP"。
type Info struct {
	Country  string
	Region   string
	Province string
	City     string
	ISP      string
}

var (
	mu      sync.Mutex
	v4Buf   []byte
	v6Buf   []byte
	v4Ready bool
	v6Ready bool
)

// Init 加载 ip2region xdb 数据库到内存。
// 缺失时分别用所给的 downloadURL（为空则用默认 URL）自动下载。
// 任一份加载失败仅记录日志，对应版本查询会返回 nil，调用方降级处理。
func Init(v4File, v4URL, v6File, v6URL string) {
	mu.Lock()
	defer mu.Unlock()

	if !v4Ready {
		buf, err := loadXdb(v4File, v4URL, DefaultV4XdbURL, xdb.IPv4)
		if err != nil {
			log.Printf("[GeoIP] 加载 IPv4 xdb 失败: %v", err)
		} else {
			v4Buf = buf
			v4Ready = true
			log.Printf("[GeoIP] IPv4 xdb 已加载: %s", v4File)
		}
	}
	if !v6Ready {
		buf, err := loadXdb(v6File, v6URL, DefaultV6XdbURL, xdb.IPv6)
		if err != nil {
			log.Printf("[GeoIP] 加载 IPv6 xdb 失败: %v", err)
		} else {
			v6Buf = buf
			v6Ready = true
			log.Printf("[GeoIP] IPv6 xdb 已加载: %s", v6File)
		}
	}
}

// Close 释放已加载的 xdb 缓冲。
func Close() {
	mu.Lock()
	defer mu.Unlock()
	v4Buf = nil
	v6Buf = nil
	v4Ready = false
	v6Ready = false
}

func loadXdb(dbFile, downloadURL, defaultURL string, version *xdb.Version) ([]byte, error) {
	if _, err := os.Stat(dbFile); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("访问 xdb 文件失败: %w", err)
		}
		url := downloadURL
		if url == "" {
			url = defaultURL
		}
		if err := os.MkdirAll(filepath.Dir(dbFile), 0o755); err != nil {
			return nil, fmt.Errorf("创建 xdb 目录失败: %w", err)
		}
		if err := downloadXdb(dbFile, url); err != nil {
			return nil, fmt.Errorf("下载 xdb 失败: %w", err)
		}
	}

	content, err := xdb.LoadContentFromFile(dbFile)
	if err != nil {
		return nil, fmt.Errorf("加载 xdb 内容失败: %w", err)
	}
	// 校验文件与当前 searcher 实现的兼容性（仅启动时一次，无需计入热路径）。
	if vErr := xdb.VerifyFromFile(dbFile); vErr != nil {
		return nil, fmt.Errorf("校验 xdb 失败: %w", vErr)
	}
	if _, err := xdb.NewWithBuffer(version, content); err != nil {
		return nil, fmt.Errorf("初始化 %s searcher 失败: %w", version.Name, err)
	}
	return content, nil
}

func downloadXdb(dst, url string) error {
	log.Printf("[GeoIP] 正在下载 ip2region.xdb: %s", url)
	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载返回状态码 %d", resp.StatusCode)
	}

	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	log.Printf("[GeoIP] xdb 下载完成: %s", filepath.Base(dst))
	return nil
}

// Lookup 查询指定 IP 的归属地信息。
// 自动区分 IPv4/IPv6 并选用对应的 xdb searcher。
// 未初始化对应版本、IP 解析失败或查询无结果时返回 nil，调用方应自行降级。
func Lookup(ip string) *Info {
	mu.Lock()
	ready := false
	var buf []byte
	parsed := net.ParseIP(ip)
	if parsed == nil {
		mu.Unlock()
		return nil
	}
	// To4() 对 IPv4-mapped IPv6 (::ffff:a.b.c.d) 也返回非 nil，走 IPv4 库更精确。
	if v4 := parsed.To4(); v4 != nil {
		ready = v4Ready
		buf = v4Buf
	} else {
		ready = v6Ready
		buf = v6Buf
	}
	mu.Unlock()
	if !ready || len(buf) == 0 {
		return nil
	}

	searcher, err := xdb.NewWithBuffer(versionFor(parsed), buf)
	if err != nil {
		return nil
	}
	region, err := searcher.Search(ip)
	if err != nil {
		return nil
	}
	return parseRegion(region)
}

func versionFor(ip net.IP) *xdb.Version {
	if ip.To4() != nil {
		return xdb.IPv4
	}
	return xdb.IPv6
}

// parseRegion 将 ip2region 的 "国家|区域|省份|城市|ISP" 字符串解析为 Info。
// "0" 占位表示无数据，统一转换为空串。
func parseRegion(region string) *Info {
	if region == "" {
		return nil
	}
	parts := strings.Split(region, "|")
	for i := range parts {
		if parts[i] == "0" {
			parts[i] = ""
		}
	}
	get := func(i int) string {
		if i < len(parts) {
			return strings.TrimSpace(parts[i])
		}
		return ""
	}
	return &Info{
		Country:  get(0),
		Region:   get(1),
		Province: get(2),
		City:     get(3),
		ISP:      get(4),
	}
}