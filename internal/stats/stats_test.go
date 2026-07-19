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

	// 写入普通下载流量
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

	// 清除可能存在的快照缓存
	snapshotMu.Lock()
	lastSnapshot = nil
	lastSnapshotTime = time.Time{}
	snapshotMu.Unlock()
	if _, err := db.DB.Exec("DELETE FROM stats_snapshot"); err != nil {
		t.Fatalf("clear snapshot error = %v", err)
	}

	data := computeStatsData()

	// 总流量 = 1024 + 2048 + 4096 = 7168
	if data.TotalTrafficBytes != 7168 {
		t.Fatalf("TotalTrafficBytes = %d, want 7168", data.TotalTrafficBytes)
	}

	// 最近30天流量 = 7168（全部在30天内）
	if data.Last30TrafficBytes != 7168 {
		t.Fatalf("Last30TrafficBytes = %d, want 7168", data.Last30TrafficBytes)
	}

	// 验证 DailyStats 中今日流量
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

	// 今日普通流量 = 1024 + 2048 = 3072
	if todayTraffic != 3072 {
		t.Fatalf("today traffic = %d, want 3072", todayTraffic)
	}
	// 昨日普通流量 = 4096
	if yesterdayTraffic != 4096 {
		t.Fatalf("yesterday traffic = %d, want 4096", yesterdayTraffic)
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
