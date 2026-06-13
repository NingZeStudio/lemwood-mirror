package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"lemwood_mirror/internal/db"
	"lemwood_mirror/internal/netutil"
	"log"
	"net"
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

	lastSnapshot     *StatsData
	lastSnapshotTime time.Time
	snapshotMu       sync.RWMutex
	refreshInFlight  sync.Mutex

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
	snapshotTTL      = 15 * time.Minute
)

func scanCount(label, query string) int64 {
	var n int64
	if err := db.DB.QueryRow(query).Scan(&n); err != nil {
		log.Printf("[Stats] %s 查询失败: %v", label, err)
		return 0
	}
	return n
}

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
	if writeQueue == nil && ipInfoQueue == nil {
		return
	}
	if writeQueue != nil {
		close(writeQueue)
	}
	if ipInfoQueue != nil {
		close(ipInfoQueue)
	}
	if workerCancel != nil {
		workerCancel()
	}

	done := make(chan struct{})
	go func() {
		workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		log.Printf("[Stats] 关闭写入池超时，可能仍有未落盘记录")
	}
}

func DroppedCount() int64 {
	return atomic.LoadInt64(&droppedCount)
}

func writeWorker() {
	defer workerWg.Done()
	for task := range writeQueue {
		if _, err := db.DB.Exec(task.query, task.args...); err != nil {
			log.Printf("数据库写入失败: %v", err)
		}
	}
}

func ipInfoWorker() {
	defer workerWg.Done()
	client := &http.Client{Timeout: 5 * time.Second}
	var lastReq time.Time
	minInterval := 2 * time.Second

	for task := range ipInfoQueue {
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

func isPrivateIP(ipStr string) bool {
	if ipStr == "localhost" {
		return true
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
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

	if resp.StatusCode == http.StatusTooManyRequests {
		log.Printf("[Stats] ip-api.com 429 限流，暂停 IP 查询: ip=%s", ip)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("[Stats] ip-api.com 返回异常状态: %d, ip=%s", resp.StatusCode, ip)
		return nil
	}

	var info IPInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		log.Printf("[Stats] ip-api.com 响应解析失败: %v, ip=%s", err, ip)
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

	defer func() {
		if r := recover(); r != nil {
			if callback != nil {
				callback(nil)
			}
		}
	}()
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
	defer func() {
		if r := recover(); r != nil {
			atomic.AddInt64(&droppedCount, 1)
		}
	}()
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
	snapshot, updatedAt := loadSnapshot()

	if snapshot != nil {
		age := time.Since(updatedAt)
		if age < snapshotTTL {
			if storagePath != "" {
				if diskInfo, err := GetDiskUsage(storagePath); err == nil {
					snapshot.Disk = diskInfo
				}
			}
			snapshot.DroppedRecords = DroppedCount()
			return snapshot, nil
		}
		// Stale: serve old data, refresh in background
		go RefreshSnapshot()
		if storagePath != "" {
			if diskInfo, err := GetDiskUsage(storagePath); err == nil {
				snapshot.Disk = diskInfo
			}
		}
		snapshot.DroppedRecords = DroppedCount()
		return snapshot, nil
	}

	// Cold start: no snapshot yet, compute synchronously
	if err := RefreshSnapshot(); err != nil {
		return &StatsData{
			TopDownloads:     []DownloadRank{},
			TopRepoDownloads: []RepoDownloadRank{},
			GeoDistribution:  []GeoStat{},
			DailyStats:       []DailyStat{},
			DroppedRecords:   DroppedCount(),
		}, err
	}

	snapshot, _ = loadSnapshot()
	if snapshot == nil {
		snapshot = &StatsData{
			TopDownloads:     []DownloadRank{},
			TopRepoDownloads: []RepoDownloadRank{},
			GeoDistribution:  []GeoStat{},
			DailyStats:       []DailyStat{},
		}
	}
	if storagePath != "" {
		if diskInfo, err := GetDiskUsage(storagePath); err == nil {
			snapshot.Disk = diskInfo
		}
	}
	snapshot.DroppedRecords = DroppedCount()
	return snapshot, nil
}

func loadSnapshot() (*StatsData, time.Time) {
	snapshotMu.RLock()
	if lastSnapshot != nil {
		cached := lastSnapshot
		updated := lastSnapshotTime
		snapshotMu.RUnlock()
		return cached, updated
	}
	snapshotMu.RUnlock()

	if db.DB == nil {
		return nil, time.Time{}
	}

	var dataJSON string
	var updatedAt time.Time
	var err error
	if db.IsMySQL() {
		err = db.DB.QueryRow("SELECT data, updated_at FROM stats_snapshot WHERE id = 1").Scan(&dataJSON, &updatedAt)
	} else {
		var raw interface{}
		err = db.DB.QueryRow("SELECT data, updated_at FROM stats_snapshot WHERE id = 1").Scan(&dataJSON, &raw)
		if err == nil {
			switch v := raw.(type) {
			case time.Time:
				updatedAt = v
			case string:
				updatedAt, _ = time.Parse("2006-01-02 15:04:05", v)
			case []byte:
				updatedAt, _ = time.Parse("2006-01-02 15:04:05", string(v))
			}
		}
	}
	if err != nil {
		return nil, time.Time{}
	}

	var data StatsData
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		log.Printf("[Stats] 快照 JSON 解析失败: %v", err)
		return nil, time.Time{}
	}

	snapshotMu.Lock()
	lastSnapshot = &data
	lastSnapshotTime = updatedAt
	snapshotMu.Unlock()

	return &data, updatedAt
}

func computeStatsData() *StatsData {
	data := &StatsData{
		TopDownloads:     []DownloadRank{},
		TopRepoDownloads: []RepoDownloadRank{},
		GeoDistribution:  []GeoStat{},
		DailyStats:       []DailyStat{},
		DroppedRecords:   DroppedCount(),
	}

	if db.DB == nil {
		return data
	}

	if db.IsMySQL() {
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			data.TotalVisits = scanCount("total_visits", "SELECT COUNT(*) FROM visits")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			data.TotalDownloads = scanCount("total_downloads", "SELECT COUNT(*) FROM downloads")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			data.TotalRepoDownloads = scanCount("total_repo_downloads", "SELECT COUNT(*) FROM repo_downloads")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			data.Last30Visits = scanCount("last_30_visits", "SELECT COUNT(*) FROM visits WHERE created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			data.Last30Downloads = scanCount("last_30_downloads", "SELECT COUNT(*) FROM downloads WHERE created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			data.Last30RepoDownloads = scanCount("last_30_repo_downloads", "SELECT COUNT(*) FROM repo_downloads WHERE created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)")
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
		return data
	}

	data.TotalVisits = scanCount("total_visits", "SELECT COUNT(*) FROM visits")
	data.TotalDownloads = scanCount("total_downloads", "SELECT COUNT(*) FROM downloads")
	data.TotalRepoDownloads = scanCount("total_repo_downloads", "SELECT COUNT(*) FROM repo_downloads")

	data.Last30Visits = scanCount("last_30_visits", "SELECT COUNT(*) FROM visits WHERE created_at > datetime('now', '-30 days')")
	data.Last30Downloads = scanCount("last_30_downloads", "SELECT COUNT(*) FROM downloads WHERE created_at > datetime('now', '-30 days')")
	data.Last30RepoDownloads = scanCount("last_30_repo_downloads", "SELECT COUNT(*) FROM repo_downloads WHERE created_at > datetime('now', '-30 days')")

	computeTotalDaysSQLite(data)
	queryTopDownloads(data)
	queryTopRepoDownloads(data)
	queryGeoDistribution(data)
	queryDailyStatsSQLite(data)

	return data
}

func saveSnapshot(data *StatsData) error {
	if db.DB == nil {
		return nil
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化快照失败: %w", err)
	}
	var query string
	if db.IsMySQL() {
		query = "INSERT INTO stats_snapshot (id, data, updated_at) VALUES (1, ?, NOW()) ON DUPLICATE KEY UPDATE data = VALUES(data), updated_at = NOW()"
	} else {
		query = "INSERT INTO stats_snapshot (id, data, updated_at) VALUES (1, ?, datetime('now')) ON CONFLICT(id) DO UPDATE SET data = excluded.data, updated_at = datetime('now')"
	}
	if _, err := db.DB.Exec(query, string(b)); err != nil {
		return fmt.Errorf("保存快照失败: %w", err)
	}
	return nil
}

func RefreshSnapshot() error {
	refreshInFlight.Lock()
	defer refreshInFlight.Unlock()

	data := computeStatsData()
	if err := saveSnapshot(data); err != nil {
		return err
	}

	snapshotMu.Lock()
	lastSnapshot = data
	lastSnapshotTime = time.Now()
	snapshotMu.Unlock()

	return nil
}

func computeTotalDaysMySQL(data *StatsData) {
	var minDate string
	if err := db.DB.QueryRow("SELECT MIN(DATE(created_at)) FROM visits").Scan(&minDate); err != nil || minDate == "" {
		if err != nil {
			log.Printf("[Stats] min_visit_date 查询失败: %v", err)
		}
		var startTimeStr string
		if err := db.DB.QueryRow("SELECT value FROM system_info WHERE `key` = 'start_time'").Scan(&startTimeStr); err == nil {
			if t, err := time.Parse("2006-01-02 15:04:05", startTimeStr); err == nil {
				data.TotalDays = int64(time.Since(t).Hours()/24) + 1
			}
		} else {
			log.Printf("[Stats] system_info.start_time 查询失败: %v", err)
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
		if err != nil {
			log.Printf("[Stats] min_visit_date 查询失败: %v", err)
		}
		var startTimeStr string
		if err := db.DB.QueryRow("SELECT value FROM system_info WHERE key = 'start_time'").Scan(&startTimeStr); err == nil {
			if t, err := time.Parse("2006-01-02 15:04:05", startTimeStr); err == nil {
				data.TotalDays = int64(time.Since(t).Hours()/24) + 1
			}
		} else {
			log.Printf("[Stats] system_info.start_time 查询失败: %v", err)
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
