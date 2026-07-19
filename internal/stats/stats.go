package stats

import (
	"context"
	"database/sql"
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

	// 启动 IP 流量记录定期清理 goroutine（每小时清理 24 小时前的 IP 级流量数据）
	workerWg.Add(1)
	go trafficCleanupWorker()

	log.Printf("[Stats] 写入工作池已初始化: %d workers, queue size: %d", workers, queueSize)
}

// trafficCleanupWorker 每小时清理一次 ip_daily_traffic 中
// 超过 24 小时的记录。IP 级数据仅保留当日用于防刷墙，历史流量已聚合到无 IP 的 daily_traffic 表。
func trafficCleanupWorker() {
	defer workerWg.Done()
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// 启动时先清理一次
	if deleted, err := db.CleanupOldTrafficRecords(); err != nil {
		log.Printf("[Stats] 清理过期 IP 流量记录失败: %v", err)
	} else if deleted > 0 {
		log.Printf("[Stats] 清理了 %d 条过期 IP 流量记录", deleted)
	}

	for {
		select {
		case <-ticker.C:
			if deleted, err := db.CleanupOldTrafficRecords(); err != nil {
				log.Printf("[Stats] 清理过期 IP 流量记录失败: %v", err)
			} else if deleted > 0 {
				log.Printf("[Stats] 清理了 %d 条过期 IP 流量记录", deleted)
			}
		case <-workerCtx.Done():
			return
		}
	}
}

func CloseWritePool() {
	if writeQueue == nil && ipInfoQueue == nil {
		return
	}
	// 先取消 ctx：让 ipInfoWorker / trafficCleanupWorker 能立即停止 HTTP 限流与远程请求，
	// 进入"仅排空队列"模式，从而在 close 之前快速 drain ipInfoQueue 剩余任务。
	if workerCancel != nil {
		workerCancel()
	}
	// 再关闭队列：writeWorker 不会感知 ctx，会继续把 writeQueue 剩余任务写入库后退出。
	if writeQueue != nil {
		close(writeQueue)
	}
	if ipInfoQueue != nil {
		close(ipInfoQueue)
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
		// 关闭中：跳过限流 sleep 和网络请求，仅快速排空队列。
		// 此时 callback（RecordVisit/RecordDownload 等）写入 writeQueue 的写入路径会
		// 被 enqueueWrite 的 panic/recover 丢弃，无需在此调用以免无谓的资源消耗。
		if workerCtx.Err() != nil {
			continue
		}

		if elapsed := time.Since(lastReq); elapsed < minInterval {
			select {
			case <-workerCtx.Done():
				continue
			case <-time.After(minInterval - elapsed):
			}
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

	// 解析 IP 属地后只存储地域信息，不存储 IP 本身
	getIPInfoAsync(ip, func(info *IPInfo) {
		country, region, city := "", "", ""
		if info != nil {
			country = info.Country
			region = info.Region
			city = info.City
		}

		enqueueWrite(&writeTask{
			query: `INSERT INTO visits (ip, path, user_agent, referer, country, region, city) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			args:  []interface{}{"", path, ua, referer, country, region, city},
		})
	})
}

func RecordDownload(r *http.Request, fileName, launcher, version string) {
	ip := netutil.ExtractClientIP(r)

	if writeQueue == nil {
		return
	}

	// 解析 IP 属地后只存储国家，不存储 IP 本身
	getIPInfoAsync(ip, func(info *IPInfo) {
		country := ""
		if info != nil {
			country = info.Country
		}

		enqueueWrite(&writeTask{
			query: `INSERT INTO downloads (file_name, launcher, version, ip, country) VALUES (?, ?, ?, ?, ?)`,
			args:  []interface{}{fileName, launcher, version, "", country},
		})
	})
}

type StatsData struct {
	TotalVisits        int64          `json:"total_visits"`
	TotalDownloads     int64          `json:"total_downloads"`
	TotalDays          int64          `json:"total_days"`
	Last30Visits       int64          `json:"last_30_visits"`
	Last30Downloads    int64          `json:"last_30_downloads"`
	TotalTrafficBytes  int64          `json:"total_traffic_bytes"`
	Last30TrafficBytes int64          `json:"last_30_traffic_bytes"`
	Disk               *DiskInfo      `json:"disk"`
	TopDownloads       []DownloadRank `json:"top_downloads"`
	GeoDistribution    []GeoStat      `json:"geo_distribution"`
	DailyStats         []DailyStat    `json:"daily_stats"`
	DroppedRecords     int64          `json:"dropped_records"`
}

type DownloadRank struct {
	Launcher string `json:"launcher"`
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
	TrafficBytes  int64  `json:"traffic_bytes"`
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
			TopDownloads:    []DownloadRank{},
			GeoDistribution: []GeoStat{},
			DailyStats:      []DailyStat{},
			DroppedRecords:  DroppedCount(),
		}, err
	}

	snapshot, _ = loadSnapshot()
	if snapshot == nil {
		snapshot = &StatsData{
			TopDownloads:    []DownloadRank{},
			GeoDistribution: []GeoStat{},
			DailyStats:      []DailyStat{},
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
		TopDownloads:    []DownloadRank{},
		GeoDistribution: []GeoStat{},
		DailyStats:      []DailyStat{},
		DroppedRecords:  DroppedCount(),
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
			computeTotalDays(data)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			queryTopDownloads(data)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			queryGeoDistribution(data)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			queryDailyStats(data)
		}()

		// 流量统计：goroutine 中只计算总量和构建 date→bytes map，
		// DailyStats 合并放到 wg.Wait() 后顺序执行，避免与 queryDailyStats 竞争。
		var dailyTrafficMap map[string]int64
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v, err := db.GetTotalTraffic(); err == nil {
				data.TotalTrafficBytes = v
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			stats, err := db.GetDailyTrafficStats(30)
			if err != nil {
				return
			}
			dailyTrafficMap = make(map[string]int64, len(stats))
			for _, s := range stats {
				dailyTrafficMap[s.Date] = s.Bytes
				data.Last30TrafficBytes += s.Bytes
			}
		}()

		wg.Wait()

		// 顺序合并每日流量到 DailyStats
		mergeDailyTraffic(data, dailyTrafficMap)

		return data
	}

	data.TotalVisits = scanCount("total_visits", "SELECT COUNT(*) FROM visits")
	data.TotalDownloads = scanCount("total_downloads", "SELECT COUNT(*) FROM downloads")

	data.Last30Visits = scanCount("last_30_visits", "SELECT COUNT(*) FROM visits WHERE created_at > datetime('now', '-30 days')")
	data.Last30Downloads = scanCount("last_30_downloads", "SELECT COUNT(*) FROM downloads WHERE created_at > datetime('now', '-30 days')")

	computeTotalDays(data)
	queryTopDownloads(data)
	queryGeoDistribution(data)
	queryDailyStats(data)

	// 流量统计（SQLite 顺序执行）
	if v, err := db.GetTotalTraffic(); err == nil {
		data.TotalTrafficBytes = v
	}
	dailyTrafficMap := make(map[string]int64)
	if stats, err := db.GetDailyTrafficStats(30); err == nil {
		for _, s := range stats {
			dailyTrafficMap[s.Date] = s.Bytes
			data.Last30TrafficBytes += s.Bytes
		}
	}
	mergeDailyTraffic(data, dailyTrafficMap)

	return data
}

// mergeDailyTraffic 将每日流量 map 合并到 DailyStats 中对应的日期。
func mergeDailyTraffic(data *StatsData, trafficMap map[string]int64) {
	for i := range data.DailyStats {
		date := data.DailyStats[i].Date
		if trafficMap != nil {
			data.DailyStats[i].TrafficBytes = trafficMap[date]
		}
	}
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

// minVisitDateQuery 返回按数据库类型定制的"最早访问日期"查询，
// 结果为 'YYYY-MM-DD' 字符串，无记录时为 NULL。
// MySQL 必须用 DATE_FORMAT 强制返回字符串：DSN 开启 parseTime=True 后，
// DATE 类型结果会被驱动转成 time.Time，无法 Scan 进 string（旧实现的隐患）。
func minVisitDateQuery() string {
	if db.IsMySQL() {
		return "SELECT DATE_FORMAT(MIN(created_at), '%Y-%m-%d') FROM visits"
	}
	return "SELECT date(MIN(created_at)) FROM visits"
}

// parseStatsTime 兼容解析历史数据里可能出现的多种时间格式。
// 无时区的格式按 UTC 解析（写入侧统一为 UTC，见 db.createTables）。
func parseStatsTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999",
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// daysSince 计算自 t 至今的天数，至少为 1（运行当天即算第 1 天），
// 容忍写入/读取时区差异导致的轻微"未来时间"。
func daysSince(t time.Time) int64 {
	days := int64(time.Since(t).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	return days
}

// computeTotalDays 计算站点运行天数：优先取 visits 表最早记录的日期，
// 无访问记录（或查询失败）时回退到 system_info.start_time（服务首次启动时间）。
// 只要能确定任一有效起始时间，TotalDays 至少为 1；两者都不可用时保持 0。
func computeTotalDays(data *StatsData) {
	var minDate sql.NullString
	if err := db.DB.QueryRow(minVisitDateQuery()).Scan(&minDate); err != nil {
		log.Printf("[Stats] min_visit_date 查询失败: %v", err)
	} else if minDate.Valid {
		if t, ok := parseStatsTime(minDate.String); ok {
			data.TotalDays = daysSince(t)
			return
		}
		log.Printf("[Stats] min_visit_date 解析失败: %q", minDate.String)
	}

	// 回退：系统首次启动时间
	keyRef := "key"
	if db.IsMySQL() {
		keyRef = "`key`"
	}
	var startTimeStr string
	if err := db.DB.QueryRow("SELECT value FROM system_info WHERE " + keyRef + " = 'start_time'").Scan(&startTimeStr); err != nil {
		log.Printf("[Stats] system_info.start_time 查询失败: %v", err)
		return
	}
	if t, ok := parseStatsTime(startTimeStr); ok {
		data.TotalDays = daysSince(t)
	} else {
		log.Printf("[Stats] system_info.start_time 解析失败: %q", startTimeStr)
	}
}

func queryTopDownloads(data *StatsData) {
	rows, err := db.DB.Query(`
		SELECT launcher, COUNT(*) as c
		FROM downloads
		GROUP BY launcher
		ORDER BY c DESC
		LIMIT 10`)
	if err != nil {
		return
	}
	defer rows.Close()

	var ranks []DownloadRank
	for rows.Next() {
		var r DownloadRank
		if err := rows.Scan(&r.Launcher, &r.Count); err != nil {
			continue
		}
		ranks = append(ranks, r)
	}
	data.TopDownloads = ranks
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

func dailyQueryFlavor() (visitQ, downloadQ string) {
	if db.IsMySQL() {
		visitQ = "SELECT DATE_FORMAT(created_at, '%Y-%m-%d') as d, COUNT(*) FROM visits GROUP BY d LIMIT 30"
		downloadQ = "SELECT DATE_FORMAT(created_at, '%Y-%m-%d') as d, COUNT(*) FROM downloads GROUP BY d LIMIT 30"
	} else {
		visitQ = "SELECT date(created_at) as d, COUNT(*) FROM visits GROUP BY d LIMIT 30"
		downloadQ = "SELECT date(created_at) as d, COUNT(*) FROM downloads GROUP BY d LIMIT 30"
	}
	return
}

func queryDailyStats(data *StatsData) {
	visitQ, downloadQ := dailyQueryFlavor()
	fillDailyStats(data, visitQ, downloadQ)
}

func fillDailyStats(data *StatsData, visitQ, downloadQ string) {
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

	var daily []DailyStat
	for _, v := range dailyMap {
		daily = append(daily, *v)
	}
	sort.Slice(daily, func(i, j int) bool {
		return daily[i].Date > daily[j].Date
	})

	data.DailyStats = daily
}
