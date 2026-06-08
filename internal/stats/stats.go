package stats

import (
	"context"
	"encoding/json"
	"lemwood_mirror/internal/db"
	"lemwood_mirror/internal/netutil"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type IPInfo struct {
	Status  string `json:"status"`
	Country string `json:"country"`
	Region  string `json:"regionName"`
	City    string `json:"city"`
	Query   string `json:"query"`
	Expires time.Time
}

type writeTask struct {
	query string
	args  []interface{}
}

type ipInfoTask struct {
	ip       string
	callback func(info *IPInfo)
}

var (
	ipCache = make(map[string]*IPInfo)
	ipMutex sync.RWMutex

	statsCache     *StatsData
	statsCacheTime time.Time
	statsMutex     sync.RWMutex

	writeQueue   chan *writeTask
	ipInfoQueue  chan *ipInfoTask
	workerWg     sync.WaitGroup
	workerCtx    context.Context
	workerCancel context.CancelFunc

	droppedCount int64
)

const (
	defaultWorkers   = 4
	defaultQueueSize = 1000
	maxIPCacheSize   = 50000
	cacheTTL         = 5 * time.Minute
)

func InitWritePool(workers int, queueSize int) {
	if workers <= 0 {
		workers = defaultWorkers
	}
	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}

	workerCtx, workerCancel = context.WithCancel(context.Background())
	writeQueue = make(chan *writeTask, queueSize)
	ipInfoQueue = make(chan *ipInfoTask, queueSize)

	for i := 0; i < workers; i++ {
		workerWg.Add(1)
		go writeWorker()
	}

	workerWg.Add(1)
	go ipInfoWorker()

	log.Printf("[Stats] 写入工作池已初始化: %d workers, queue size: %d", workers, queueSize)
}

func CloseWritePool() {
	if workerCancel != nil {
		workerCancel()
		workerWg.Wait()
	}
}

func DroppedCount() int64 {
	return atomic.LoadInt64(&droppedCount)
}

func writeWorker() {
	defer workerWg.Done()
	for {
		select {
		case <-workerCtx.Done():
			return
		case task, ok := <-writeQueue:
			if !ok {
				return
			}
			if _, err := db.DB.Exec(task.query, task.args...); err != nil {
				log.Printf("数据库写入失败: %v", err)
			}
		}
	}
}

func ipInfoWorker() {
	defer workerWg.Done()
	client := &http.Client{Timeout: 5 * time.Second}
	var lastReq time.Time
	minInterval := 2 * time.Second

	for {
		select {
		case <-workerCtx.Done():
			return
		case task, ok := <-ipInfoQueue:
			if !ok {
				return
			}
			if elapsed := time.Since(lastReq); elapsed < minInterval {
				time.Sleep(minInterval - elapsed)
			}
			info := fetchIPInfo(client, task.ip)
			lastReq = time.Now()
			if info != nil {
				ipMutex.Lock()
				if len(ipCache) >= maxIPCacheSize {
					evictIPCache()
				}
				ipCache[task.ip] = info
				ipMutex.Unlock()
			}
			if task.callback != nil {
				task.callback(info)
			}
		}
	}
}

func evictIPCache() {
	now := time.Now()
	for k, v := range ipCache {
		if now.After(v.Expires) {
			delete(ipCache, k)
		}
	}
	if len(ipCache) >= maxIPCacheSize {
		oldest := ""
		oldestTime := time.Now()
		for k, v := range ipCache {
			if v.Expires.Before(oldestTime) {
				oldestTime = v.Expires
				oldest = k
			}
		}
		if oldest != "" {
			delete(ipCache, oldest)
		}
	}
}

func isPrivateIP(ip string) bool {
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" ||
		strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") {
		return true
	}
	if strings.HasPrefix(ip, "172.") {
		parts := strings.SplitN(ip, ".", 3)
		if len(parts) >= 2 {
			var second int
			for _, c := range parts[1] {
				if c >= '0' && c <= '9' {
					second = second*10 + int(c-'0')
				} else {
					break
				}
			}
			if second >= 16 && second <= 31 {
				return true
			}
		}
	}
	return false
}

func fetchIPInfo(client *http.Client, ip string) *IPInfo {
	if isPrivateIP(ip) {
		return &IPInfo{
			Country: "Local",
			Region:  "Local",
			City:    "Local",
		}
	}

	resp, err := client.Get("https://ip-api.com/json/" + ip + "?lang=zh-CN")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var info IPInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil
	}

	if info.Status == "success" {
		info.Expires = time.Now().Add(24 * time.Hour)
		return &info
	}
	return nil
}

func getIPInfoAsync(ip string, callback func(info *IPInfo)) {
	ipMutex.RLock()
	if info, ok := ipCache[ip]; ok {
		if time.Now().Before(info.Expires) {
			ipMutex.RUnlock()
			if callback != nil {
				callback(info)
			}
			return
		}
	}
	ipMutex.RUnlock()

	if ipInfoQueue == nil {
		if callback != nil {
			callback(nil)
		}
		return
	}

	select {
	case ipInfoQueue <- &ipInfoTask{ip: ip, callback: callback}:
	default:
		if callback != nil {
			callback(nil)
		}
	}
}

func enqueueWrite(task *writeTask) {
	if writeQueue == nil {
		return
	}
	select {
	case writeQueue <- task:
	default:
		atomic.AddInt64(&droppedCount, 1)
		log.Printf("写入队列已满，丢弃记录 (总丢弃: %d)", atomic.LoadInt64(&droppedCount))
	}
}

func RecordVisit(r *http.Request) {
	ip := netutil.ExtractClientIP(r)
	path := r.URL.Path
	ua := r.UserAgent()
	referer := r.Referer()

	if strings.HasPrefix(path, "/dist/") ||
		strings.HasPrefix(path, "/assets/") ||
		strings.HasPrefix(path, "/api/") ||
		path == "/favicon.svg" {
		return
	}

	if writeQueue == nil {
		return
	}

	getIPInfoAsync(ip, func(info *IPInfo) {
		country, region, city := "", "", ""
		if info != nil {
			country = info.Country
			region = info.Region
			city = info.City
		}

		enqueueWrite(&writeTask{
			query: `INSERT INTO visits (ip, path, user_agent, referer, country, region, city) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			args:  []interface{}{ip, path, ua, referer, country, region, city},
		})
	})
}

func RecordDownload(r *http.Request, fileName, launcher, version string) {
	ip := netutil.ExtractClientIP(r)

	if writeQueue == nil {
		return
	}

	getIPInfoAsync(ip, func(info *IPInfo) {
		country := ""
		if info != nil {
			country = info.Country
		}

		enqueueWrite(&writeTask{
			query: `INSERT INTO downloads (file_name, launcher, version, ip, country) VALUES (?, ?, ?, ?, ?)`,
			args:  []interface{}{fileName, launcher, version, ip, country},
		})
	})
}

func RecordRepoDownload(r *http.Request, repoName, repoPath string) {
	ip := netutil.ExtractClientIP(r)

	if writeQueue == nil {
		return
	}

	getIPInfoAsync(ip, func(info *IPInfo) {
		country := ""
		if info != nil {
			country = info.Country
		}

		enqueueWrite(&writeTask{
			query: `INSERT INTO repo_downloads (repo_name, repo_path, ip, country) VALUES (?, ?, ?, ?)`,
			args:  []interface{}{repoName, repoPath, ip, country},
		})
	})
}

type StatsData struct {
	TotalVisits         int64              `json:"total_visits"`
	TotalDownloads      int64              `json:"total_downloads"`
	TotalRepoDownloads  int64              `json:"total_repo_downloads"`
	TotalDays           int64              `json:"total_days"`
	Last30Visits        int64              `json:"last_30_visits"`
	Last30Downloads     int64              `json:"last_30_downloads"`
	Last30RepoDownloads int64              `json:"last_30_repo_downloads"`
	Disk                *DiskInfo          `json:"disk"`
	TopDownloads        []DownloadRank     `json:"top_downloads"`
	TopRepoDownloads    []RepoDownloadRank `json:"top_repo_downloads"`
	GeoDistribution     []GeoStat          `json:"geo_distribution"`
	DailyStats          []DailyStat        `json:"daily_stats"`
	DroppedRecords      int64              `json:"dropped_records"`
}

type DownloadRank struct {
	Launcher string `json:"launcher"`
	Version  string `json:"version"`
	Count    int64  `json:"count"`
}

type RepoDownloadRank struct {
	RepoName string `json:"repo_name"`
	Count    int64  `json:"count"`
}

type GeoStat struct {
	Country string `json:"country"`
	Count   int64  `json:"count"`
}

type DailyStat struct {
	Date              string `json:"date"`
	VisitCount        int64  `json:"visit_count"`
	DownloadCount     int64  `json:"download_count"`
	RepoDownloadCount int64  `json:"repo_download_count"`
}

func GetStats(storagePath string) (*StatsData, error) {
	statsMutex.RLock()
	if statsCache != nil && time.Since(statsCacheTime) < cacheTTL {
		cached := statsCache
		statsMutex.RUnlock()
		return cached, nil
	}
	statsMutex.RUnlock()

	data := &StatsData{
		TopDownloads:     []DownloadRank{},
		TopRepoDownloads: []RepoDownloadRank{},
		GeoDistribution:  []GeoStat{},
		DailyStats:       []DailyStat{},
		DroppedRecords:   DroppedCount(),
	}

	if db.DB == nil {
		return data, nil
	}

	if db.IsMySQL() {
		return getStatsMySQL(data, storagePath)
	}
	return getStatsSQLite(data, storagePath)
}

func getStatsMySQL(data *StatsData, storagePath string) (*StatsData, error) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if storagePath != "" {
			if diskInfo, err := GetDiskUsage(storagePath); err == nil {
				data.Disk = diskInfo
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		db.DB.QueryRow("SELECT COUNT(*) FROM visits").Scan(&data.TotalVisits)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		db.DB.QueryRow("SELECT COUNT(*) FROM downloads").Scan(&data.TotalDownloads)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		db.DB.QueryRow("SELECT COUNT(*) FROM repo_downloads").Scan(&data.TotalRepoDownloads)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		db.DB.QueryRow("SELECT COUNT(*) FROM visits WHERE created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)").Scan(&data.Last30Visits)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		db.DB.QueryRow("SELECT COUNT(*) FROM downloads WHERE created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)").Scan(&data.Last30Downloads)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		db.DB.QueryRow("SELECT COUNT(*) FROM repo_downloads WHERE created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)").Scan(&data.Last30RepoDownloads)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		computeTotalDaysMySQL(data)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		queryTopDownloads(data)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		queryTopRepoDownloads(data)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		queryGeoDistribution(data)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		queryDailyStatsMySQL(data)
	}()

	wg.Wait()

	statsMutex.Lock()
	statsCache = data
	statsCacheTime = time.Now()
	statsMutex.Unlock()

	return data, nil
}

func getStatsSQLite(data *StatsData, storagePath string) (*StatsData, error) {
	if storagePath != "" {
		if diskInfo, err := GetDiskUsage(storagePath); err == nil {
			data.Disk = diskInfo
		}
	}

	db.DB.QueryRow("SELECT COUNT(*) FROM visits").Scan(&data.TotalVisits)
	db.DB.QueryRow("SELECT COUNT(*) FROM downloads").Scan(&data.TotalDownloads)
	db.DB.QueryRow("SELECT COUNT(*) FROM repo_downloads").Scan(&data.TotalRepoDownloads)

	db.DB.QueryRow("SELECT COUNT(*) FROM visits WHERE created_at > datetime('now', '-30 days')").Scan(&data.Last30Visits)
	db.DB.QueryRow("SELECT COUNT(*) FROM downloads WHERE created_at > datetime('now', '-30 days')").Scan(&data.Last30Downloads)
	db.DB.QueryRow("SELECT COUNT(*) FROM repo_downloads WHERE created_at > datetime('now', '-30 days')").Scan(&data.Last30RepoDownloads)

	computeTotalDaysSQLite(data)
	queryTopDownloads(data)
	queryTopRepoDownloads(data)
	queryGeoDistribution(data)
	queryDailyStatsSQLite(data)

	statsMutex.Lock()
	statsCache = data
	statsCacheTime = time.Now()
	statsMutex.Unlock()

	return data, nil
}

func computeTotalDaysMySQL(data *StatsData) {
	var minDate string
	if err := db.DB.QueryRow("SELECT MIN(DATE(created_at)) FROM visits").Scan(&minDate); err != nil || minDate == "" {
		var startTimeStr string
		if err := db.DB.QueryRow("SELECT value FROM system_info WHERE `key` = 'start_time'").Scan(&startTimeStr); err == nil {
			if t, err := time.Parse("2006-01-02 15:04:05", startTimeStr); err == nil {
				data.TotalDays = int64(time.Since(t).Hours()/24) + 1
			}
		}
		return
	}
	if t, err := time.Parse("2006-01-02", minDate); err == nil {
		data.TotalDays = int64(time.Since(t).Hours()/24) + 1
	}
}

func computeTotalDaysSQLite(data *StatsData) {
	var minDate string
	if err := db.DB.QueryRow("SELECT date(MIN(created_at)) FROM visits").Scan(&minDate); err != nil || minDate == "" {
		var startTimeStr string
		if err := db.DB.QueryRow("SELECT value FROM system_info WHERE key = 'start_time'").Scan(&startTimeStr); err == nil {
			if t, err := time.Parse("2006-01-02 15:04:05", startTimeStr); err == nil {
				data.TotalDays = int64(time.Since(t).Hours()/24) + 1
			}
		}
		return
	}
	if t, err := time.Parse("2006-01-02", minDate); err == nil {
		data.TotalDays = int64(time.Since(t).Hours()/24) + 1
	}
}

func queryTopDownloads(data *StatsData) {
	rows, err := db.DB.Query(`
		SELECT launcher, version, COUNT(*) as c
		FROM downloads
		GROUP BY launcher, version
		ORDER BY c DESC
		LIMIT 10`)
	if err != nil {
		return
	}
	defer rows.Close()

	var ranks []DownloadRank
	for rows.Next() {
		var r DownloadRank
		if err := rows.Scan(&r.Launcher, &r.Version, &r.Count); err != nil {
			continue
		}
		ranks = append(ranks, r)
	}
	data.TopDownloads = ranks
}

func queryTopRepoDownloads(data *StatsData) {
	rows, err := db.DB.Query(`
		SELECT repo_name, COUNT(*) as c
		FROM repo_downloads
		GROUP BY repo_name
		ORDER BY c DESC
		LIMIT 10`)
	if err != nil {
		return
	}
	defer rows.Close()

	var ranks []RepoDownloadRank
	for rows.Next() {
		var r RepoDownloadRank
		if err := rows.Scan(&r.RepoName, &r.Count); err != nil {
			continue
		}
		ranks = append(ranks, r)
	}
	data.TopRepoDownloads = ranks
}

func queryGeoDistribution(data *StatsData) {
	rows, err := db.DB.Query(`
		SELECT country, COUNT(*) as c
		FROM visits
		WHERE country != '' AND country != 'Local'
		GROUP BY country
		ORDER BY c DESC
		LIMIT 50`)
	if err != nil {
		return
	}
	defer rows.Close()

	var geos []GeoStat
	for rows.Next() {
		var g GeoStat
		if err := rows.Scan(&g.Country, &g.Count); err != nil {
			continue
		}
		geos = append(geos, g)
	}
	data.GeoDistribution = geos
}

func dailyQueryFlavor() (visitQ, downloadQ, repoQ string) {
	if db.IsMySQL() {
		visitQ = "SELECT DATE_FORMAT(created_at, '%Y-%m-%d') as d, COUNT(*) FROM visits GROUP BY d LIMIT 30"
		downloadQ = "SELECT DATE_FORMAT(created_at, '%Y-%m-%d') as d, COUNT(*) FROM downloads GROUP BY d LIMIT 30"
		repoQ = "SELECT DATE_FORMAT(created_at, '%Y-%m-%d') as d, COUNT(*) FROM repo_downloads GROUP BY d LIMIT 30"
	} else {
		visitQ = "SELECT date(created_at) as d, COUNT(*) FROM visits GROUP BY d LIMIT 30"
		downloadQ = "SELECT date(created_at) as d, COUNT(*) FROM downloads GROUP BY d LIMIT 30"
		repoQ = "SELECT date(created_at) as d, COUNT(*) FROM repo_downloads GROUP BY d LIMIT 30"
	}
	return
}

func queryDailyStatsMySQL(data *StatsData) {
	visitQ, downloadQ, repoQ := dailyQueryFlavor()
	fillDailyStats(data, visitQ, downloadQ, repoQ)
}

func queryDailyStatsSQLite(data *StatsData) {
	visitQ, downloadQ, repoQ := dailyQueryFlavor()
	fillDailyStats(data, visitQ, downloadQ, repoQ)
}

func fillDailyStats(data *StatsData, visitQ, downloadQ, repoQ string) {
	dailyMap := make(map[string]*DailyStat)

	vRows, err := db.DB.Query(visitQ)
	if err == nil {
		defer vRows.Close()
		for vRows.Next() {
			var d string
			var c int64
			if err := vRows.Scan(&d, &c); err != nil {
				continue
			}
			if dailyMap[d] == nil {
				dailyMap[d] = &DailyStat{Date: d}
			}
			dailyMap[d].VisitCount = c
		}
	}

	dRows, err := db.DB.Query(downloadQ)
	if err == nil {
		defer dRows.Close()
		for dRows.Next() {
			var d string
			var c int64
			if err := dRows.Scan(&d, &c); err != nil {
				continue
			}
			if dailyMap[d] == nil {
				dailyMap[d] = &DailyStat{Date: d}
			}
			dailyMap[d].DownloadCount = c
		}
	}

	rRows, err := db.DB.Query(repoQ)
	if err == nil {
		defer rRows.Close()
		for rRows.Next() {
			var d string
			var c int64
			if err := rRows.Scan(&d, &c); err != nil {
				continue
			}
			if dailyMap[d] == nil {
				dailyMap[d] = &DailyStat{Date: d}
			}
			dailyMap[d].RepoDownloadCount = c
		}
	}

	var daily []DailyStat
	for _, v := range dailyMap {
		daily = append(daily, *v)
	}
	sort.Slice(daily, func(i, j int) bool {
		return daily[i].Date > daily[j].Date
	})

	data.DailyStats = daily
}
