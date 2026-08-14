package stats

import (
	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"
	"testing"
	"time"
)

func setupStatsTestDB(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	cfg := &config.Config{}
	if db.DB != nil {
		_ = db.DB.Close()
		db.DB = nil
	}
	if err := db.InitDB(base, cfg); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() {
		if db.DB != nil {
			_ = db.DB.Close()
			db.DB = nil
		}
	})
}

func TestComputeStatsDataTraffic(t *testing.T) {
	setupStatsTestDB(t)

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// 写入 visits 记录，使 DailyStats 包含今日和昨日
	if _, err := db.DB.Exec("INSERT INTO visits (ip, path) VALUES (?, ?)", "1.1.1.1", "/"); err != nil {
		t.Fatalf("insert visit error = %v", err)
	}
	if _, err := db.DB.Exec("INSERT INTO visits (ip, path, created_at) VALUES (?, ?, ?)", "3.3.3.3", "/", yesterday+" 12:00:00"); err != nil {
		t.Fatalf("insert yesterday visit error = %v", err)
	}

	// 写入普通下载流量（served 口径）
	if err := db.RecordTraffic("1.1.1.1", 1024); err != nil {
		t.Fatalf("RecordTraffic() error = %v", err)
	}
	if err := db.RecordTraffic("2.2.2.2", 2048); err != nil {
		t.Fatalf("RecordTraffic() error = %v", err)
	}
	// 写入昨日流量（应在 last_30 范围内）——同时写入 IP 级表和聚合表
	if _, err := db.DB.Exec("INSERT INTO ip_daily_traffic (ip, date, bytes_downloaded) VALUES (?, ?, ?)", "3.3.3.3", yesterday, 4096); err != nil {
		t.Fatalf("insert yesterday traffic error = %v", err)
	}
	if _, err := db.DB.Exec("INSERT INTO daily_traffic (date, bytes_downloaded) VALUES (?, ?)", yesterday, 4096); err != nil {
		t.Fatalf("insert yesterday daily_traffic error = %v", err)
	}

	// 写入完整传输流量（completed 展示口径）：今日 512 + 1024 = 1536，昨日 2048
	if err := db.RecordCompletedTraffic(512); err != nil {
		t.Fatalf("RecordCompletedTraffic() error = %v", err)
	}
	if err := db.RecordCompletedTraffic(1024); err != nil {
		t.Fatalf("RecordCompletedTraffic() error = %v", err)
	}
	if _, err := db.DB.Exec("INSERT INTO daily_completed_traffic (date, bytes_downloaded) VALUES (?, ?)", yesterday, 2048); err != nil {
		t.Fatalf("insert yesterday daily_completed_traffic error = %v", err)
	}

	// 清除可能存在的快照缓存
	snapshotMu.Lock()
	lastSnapshot = nil
	lastSnapshotTime = time.Time{}
	snapshotMu.Unlock()
	if _, err := db.DB.Exec("DELETE FROM stats_snapshot"); err != nil {
		t.Fatalf("clear snapshot error = %v", err)
	}

	data := computeStatsData()

	// 展示口径（completed）总流量 = 512 + 1024 + 2048 = 3584
	if data.TotalTrafficBytes != 3584 {
		t.Fatalf("TotalTrafficBytes = %d, want 3584", data.TotalTrafficBytes)
	}

	// 展示口径最近30天流量 = 3584（全部在30天内）
	if data.Last30TrafficBytes != 3584 {
		t.Fatalf("Last30TrafficBytes = %d, want 3584", data.Last30TrafficBytes)
	}

	// served 口径总流量 = 1024 + 2048 + 4096 = 7168
	if data.TotalServedBytes != 7168 {
		t.Fatalf("TotalServedBytes = %d, want 7168", data.TotalServedBytes)
	}
	if data.Last30ServedBytes != 7168 {
		t.Fatalf("Last30ServedBytes = %d, want 7168", data.Last30ServedBytes)
	}

	// 验证 DailyStats 中的流量（展示口径）
	todayTraffic := int64(0)
	yesterdayTraffic := int64(0)
	for _, ds := range data.DailyStats {
		if ds.Date == today {
			todayTraffic = ds.TrafficBytes
		}
		if ds.Date == yesterday {
			yesterdayTraffic = ds.TrafficBytes
		}
	}

	// 今日完整传输流量 = 512 + 1024 = 1536
	if todayTraffic != 1536 {
		t.Fatalf("today traffic = %d, want 1536", todayTraffic)
	}
	// 昨日完整传输流量 = 2048
	if yesterdayTraffic != 2048 {
		t.Fatalf("yesterday traffic = %d, want 2048", yesterdayTraffic)
	}
}

// TestComputeStatsDataEventsMerge 验证流量字节 = 冻结基线 + 事件表的合并口径。
// 基线表（daily_traffic/daily_completed_traffic）承载历史字节；download_events 承载新字节。
func TestComputeStatsDataEventsMerge(t *testing.T) {
	setupStatsTestDB(t)

	today := time.Now().UTC().Format("2006-01-02")

	// 今日访问，使 DailyStats 包含今日行
	if _, err := db.DB.Exec("INSERT INTO visits (ip, path, created_at) VALUES (?, ?, ?)", "1.1.1.1", "/", today+" 12:00:00"); err != nil {
		t.Fatalf("insert visit error = %v", err)
	}

	// 冻结基线：completed today 1000, served today 2000
	if _, err := db.DB.Exec("INSERT INTO daily_completed_traffic (date, bytes_downloaded) VALUES (?, ?)", today, 1000); err != nil {
		t.Fatalf("insert baseline completed error = %v", err)
	}
	if _, err := db.DB.Exec("INSERT INTO daily_traffic (date, bytes_downloaded) VALUES (?, ?)", today, 2000); err != nil {
		t.Fatalf("insert baseline served error = %v", err)
	}

	// 事件表新数据：一次下载 bytes_served=500 completed=1
	if err := db.RecordDownloadEvent(db.DownloadEvent{
		FilePath: "fcl/1.0/a.apk", Launcher: "fcl", Version: "1.0",
		ClientIP: "1.1.1.1", BytesServed: 500, Completed: true, StatusCode: 200,
	}); err != nil {
		t.Fatalf("RecordDownloadEvent error = %v", err)
	}

	snapshotMu.Lock()
	lastSnapshot = nil
	lastSnapshotTime = time.Time{}
	snapshotMu.Unlock()
	if _, err := db.DB.Exec("DELETE FROM stats_snapshot"); err != nil {
		t.Fatalf("clear snapshot error = %v", err)
	}

	data := computeStatsData()

	// completed 总 = 基线 1000 + 事件 500 = 1500
	if data.TotalTrafficBytes != 1500 {
		t.Fatalf("TotalTrafficBytes = %d, want 1500", data.TotalTrafficBytes)
	}
	if data.Last30TrafficBytes != 1500 {
		t.Fatalf("Last30TrafficBytes = %d, want 1500", data.Last30TrafficBytes)
	}
	// served 总 = 基线 2000 + 事件 500 = 2500
	if data.TotalServedBytes != 2500 {
		t.Fatalf("TotalServedBytes = %d, want 2500", data.TotalServedBytes)
	}
	if data.Last30ServedBytes != 2500 {
		t.Fatalf("Last30ServedBytes = %d, want 2500", data.Last30ServedBytes)
	}
	// 下载次数来自事件表（=1），downloads 表冻结为 0 行
	if data.TotalDownloads != 1 {
		t.Fatalf("TotalDownloads = %d, want 1 (from events)", data.TotalDownloads)
	}
	// top downloads 来自事件：fcl=1
	if len(data.TopDownloads) != 1 || data.TopDownloads[0].Launcher != "fcl" || data.TopDownloads[0].Count != 1 {
		t.Fatalf("TopDownloads = %+v, want fcl=1", data.TopDownloads)
	}
	// DailyStats 今日 completed = 1000 + 500 = 1500
	for _, ds := range data.DailyStats {
		if ds.Date == today && ds.TrafficBytes != 1500 {
			t.Fatalf("today TrafficBytes = %d, want 1500", ds.TrafficBytes)
		}
	}
	// DailyStats 今日下载次数 = 事件表 1（downloads 表冻结为 0 行，不得回退为 0）
	for _, ds := range data.DailyStats {
		if ds.Date == today && ds.DownloadCount != 1 {
			t.Fatalf("today DownloadCount = %d, want 1 (from events)", ds.DownloadCount)
		}
	}
}

// TestComputeTotalDaysFromVisits 有访问记录时，运行天数取 visits 最早记录至今，至少为 1。
func TestComputeTotalDaysFromVisits(t *testing.T) {
	setupStatsTestDB(t)

	if _, err := db.DB.Exec("INSERT INTO visits (ip, path) VALUES (?, ?)", "1.1.1.1", "/"); err != nil {
		t.Fatalf("insert visit error = %v", err)
	}

	data := &StatsData{}
	computeTotalDays(data)
	if data.TotalDays < 1 {
		t.Fatalf("TotalDays = %d, want >= 1", data.TotalDays)
	}
}

// TestComputeTotalDaysFallbackStartTime 无访问记录时回退到 system_info.start_time，至少为 1。
func TestComputeTotalDaysFallbackStartTime(t *testing.T) {
	setupStatsTestDB(t)

	data := &StatsData{}
	computeTotalDays(data)
	if data.TotalDays < 1 {
		t.Fatalf("TotalDays = %d, want >= 1 (start_time fallback)", data.TotalDays)
	}
}

// TestComputeTotalDaysOldVisit 最早的访问记录在 N 天前时，TotalDays = N+1。
func TestComputeTotalDaysOldVisit(t *testing.T) {
	setupStatsTestDB(t)

	tenDaysAgo := time.Now().AddDate(0, 0, -10).Format("2006-01-02 15:04:05")
	if _, err := db.DB.Exec("INSERT INTO visits (ip, path, created_at) VALUES (?, ?, ?)", "1.1.1.1", "/", tenDaysAgo); err != nil {
		t.Fatalf("insert old visit error = %v", err)
	}

	data := &StatsData{}
	computeTotalDays(data)
	if data.TotalDays != 11 {
		t.Fatalf("TotalDays = %d, want 11", data.TotalDays)
	}
}

// TestParseStatsTime 兼容多种历史时间格式。
func TestParseStatsTime(t *testing.T) {
	cases := []string{
		"2006-01-02 15:04:05",
		"2026-07-19 06:50:43",
		"2026-07-19T06:50:43Z",
		"2026-07-19T06:50:43",
		"2026-07-19",
		"2026-07-19 06:50:43.123456",
	}
	for _, s := range cases {
		if _, ok := parseStatsTime(s); !ok {
			t.Errorf("parseStatsTime(%q) failed, want success", s)
		}
	}
	for _, s := range []string{"", "not-a-time", "2026/07/19"} {
		if _, ok := parseStatsTime(s); ok {
			t.Errorf("parseStatsTime(%q) succeeded, want failure", s)
		}
	}
}

// TestDaysSinceClamp 起始时间略在未来（时区偏差）时，天数至少为 1。
func TestDaysSinceClamp(t *testing.T) {
	if d := daysSince(time.Now().Add(8 * time.Hour)); d != 1 {
		t.Fatalf("daysSince(future) = %d, want 1", d)
	}
	if d := daysSince(time.Now().AddDate(0, 0, -2)); d != 3 {
		t.Fatalf("daysSince(-2d) = %d, want 3", d)
	}
}
