package stats

import (
	"database/sql"
	"encoding/json"
	"lemwood_mirror/internal/db"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// IPInfo 结构体
type IPInfo struct {
	Status  string `json:"status"`
	Country string `json:"country"`
	Region  string `json:"regionName"`
	City    string `json:"city"`
	Query   string `json:"query"`
}

// IP 缓存，避免重复请求
var (
	ipCache = make(map[string]*IPInfo)
	ipMutex sync.RWMutex
)

// RecordVisit 记录访问
func RecordVisit(r *http.Request) {
	ip := getClientIP(r)
	path := r.URL.Path
	ua := r.UserAgent()
	referer := r.Referer()

	// 忽略静态资源（可根据需要调整）
	if strings.HasPrefix(path, "/dist/") || path == "/favicon.svg" {
		return
	}

	// 异步处理
	go func() {
		// 获取 IP 信息
		info := getIPInfo(ip)
		country, region, city := "", "", ""
		if info != nil {
			country = info.Country
			region = info.Region
			city = info.City
		}

		_, err := db.DB.Exec(`INSERT INTO visits (ip, path, user_agent, referer, country, region, city) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			ip, path, ua, referer, country, region, city)
		if err != nil {
			log.Printf("Failed to record visit: %v", err)
		}
	}()
}

// RecordDownload 记录下载
func RecordDownload(r *http.Request, fileName, launcher, version string) {
	ip := getClientIP(r)

	go func() {
		info := getIPInfo(ip)
		country := ""
		if info != nil {
			country = info.Country
		}

		_, err := db.DB.Exec(`INSERT INTO downloads (file_name, launcher, version, ip, country) VALUES (?, ?, ?, ?, ?)`,
			fileName, launcher, version, ip, country)
		if err != nil {
			log.Printf("Failed to record download: %v", err)
		}
	}()
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	if strings.Contains(ip, ",") {
		ip = strings.Split(ip, ",")[0]
	}
	ip = strings.TrimSpace(ip)
	// 去掉端口号
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		if !strings.Contains(ip, "]") {
			ip = ip[:idx]
		} else if strings.HasSuffix(ip, "]") {
			// [::1]
		} else {
			// [::1]:8080 -> [::1]
			lastColon := strings.LastIndex(ip, ":")
			closingBracket := strings.LastIndex(ip, "]")
			if lastColon > closingBracket {
				ip = ip[:lastColon]
			}
		}
	}
	ip = strings.Trim(ip, "[]")
	return ip
}

func getIPInfo(ip string) *IPInfo {
	// 本地 IP
	if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || ip == "localhost" {
		return &IPInfo{Country: "Local", Region: "Local", City: "Local"}
	}

	ipMutex.RLock()
	if info, ok := ipCache[ip]; ok {
		ipMutex.RUnlock()
		return info
	}
	ipMutex.RUnlock()

	// 请求 ip-api.com
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://ip-api.com/json/" + ip + "?lang=zh-CN")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var info IPInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil
	}

	if info.Status == "success" {
		ipMutex.Lock()
		ipCache[ip] = &info
		ipMutex.Unlock()
		return &info
	}
	return nil
}

// 统计数据结构
type StatsData struct {
	TotalVisits     int64          `json:"total_visits"`
	TotalDownloads  int64          `json:"total_downloads"`
	TopDownloads    []DownloadRank `json:"top_downloads"`
	GeoDistribution []GeoStat      `json:"geo_distribution"`
	DailyStats      []DailyStat    `json:"daily_stats"`
}

type DownloadRank struct {
	Launcher string `json:"launcher"`
	Version  string `json:"version"`
	Count    int64  `json:"count"`
}

type GeoStat struct {
	Country string `json:"country"`
	Count   int64  `json:"count"`
}

type DailyStat struct {
	Date          string `json:"date"`
	VisitCount    int64  `json:"visit_count"`
	DownloadCount int64  `json:"download_count"`
}

func GetStats() (*StatsData, error) {
	data := &StatsData{
		TopDownloads:    []DownloadRank{},
		GeoDistribution: []GeoStat{},
		DailyStats:      []DailyStat{},
	}

	if db.DB == nil {
		return data, nil
	}

	// 总访问量
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM visits").Scan(&data.TotalVisits); err != nil && err != sql.ErrNoRows {
		log.Printf("Error counting visits: %v", err)
	}

	// 总下载量
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM downloads").Scan(&data.TotalDownloads); err != nil && err != sql.ErrNoRows {
		log.Printf("Error counting downloads: %v", err)
	}

	// 下载排行 (Top 10)
	rows, err := db.DB.Query(`
        SELECT launcher, version, COUNT(*) as c 
        FROM downloads 
        GROUP BY launcher, version 
        ORDER BY c DESC 
        LIMIT 10`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var r DownloadRank
			rows.Scan(&r.Launcher, &r.Version, &r.Count)
			data.TopDownloads = append(data.TopDownloads, r)
		}
	}

	// 地域分布
	rows2, err := db.DB.Query(`
        SELECT country, COUNT(*) as c 
        FROM visits 
        WHERE country != '' AND country != 'Local'
        GROUP BY country 
        ORDER BY c DESC`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var g GeoStat
			rows2.Scan(&g.Country, &g.Count)
			data.GeoDistribution = append(data.GeoDistribution, g)
		}
	}

	// 每日统计
	dailyMap := make(map[string]*DailyStat)

	// 查访问
	vRows, err := db.DB.Query(`SELECT date(created_at), COUNT(*) FROM visits GROUP BY date(created_at) ORDER BY date(created_at) DESC LIMIT 30`)
	if err == nil {
		defer vRows.Close()
		for vRows.Next() {
			var d string
			var c int64
			if err := vRows.Scan(&d, &c); err == nil {
				if dailyMap[d] == nil {
					dailyMap[d] = &DailyStat{Date: d}
				}
				dailyMap[d].VisitCount = c
			}
		}
	}

	// 查下载
	dRows, err := db.DB.Query(`SELECT date(created_at), COUNT(*) FROM downloads GROUP BY date(created_at) ORDER BY date(created_at) DESC LIMIT 30`)
	if err == nil {
		defer dRows.Close()
		for dRows.Next() {
			var d string
			var c int64
			if err := dRows.Scan(&d, &c); err == nil {
				if dailyMap[d] == nil {
					dailyMap[d] = &DailyStat{Date: d}
				}
				dailyMap[d].DownloadCount = c
			}
		}
	}

	for _, v := range dailyMap {
		data.DailyStats = append(data.DailyStats, *v)
	}

	// 排序
	sort.Slice(data.DailyStats, func(i, j int) bool {
		return data.DailyStats[i].Date > data.DailyStats[j].Date
	})

	return data, nil
}
