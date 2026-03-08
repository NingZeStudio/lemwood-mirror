package traffic

import (
	"fmt"
	"io"
	"lemwood_mirror/internal/db"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Tracker struct {
	limitGB          int64
	banRecordFile    string
	appealContact    string
	fileMutex        sync.Mutex
	storagePath      string
}

var defaultTracker *Tracker

func InitTracker(limitGB int, banRecordFile, appealContact, storagePath string) {
	defaultTracker = &Tracker{
		limitGB:       int64(limitGB) * 1024 * 1024 * 1024,
		banRecordFile: banRecordFile,
		appealContact: appealContact,
		storagePath:   storagePath,
	}
	if defaultTracker.banRecordFile != "" {
		defaultTracker.initBanRecordFile()
	}
}

func GetTracker() *Tracker {
	return defaultTracker
}

func (t *Tracker) initBanRecordFile() {
	fullPath := filepath.Join(t.storagePath, t.banRecordFile)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[防刷墙] 创建封禁记录目录失败: %v", err)
		return
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		header := fmt.Sprintf(`# IP封禁记录 - 公开数据
# 格式: IP | 封禁时间 | 封禁理由 | 当日流量(GB)
# 如有误封，请加入 %s 进行申诉

`, t.appealContact)
		if err := os.WriteFile(fullPath, []byte(header), 0644); err != nil {
			log.Printf("[防刷墙] 初始化封禁记录文件失败: %v", err)
		}
	}
}

func (t *Tracker) RecordTraffic(ip string, bytes int64) error {
	return db.RecordTraffic(ip, bytes)
}

func (t *Tracker) GetDailyTraffic(ip string) (int64, error) {
	return db.GetDailyTraffic(ip)
}

func (t *Tracker) CheckAndBan(ip string) (bool, string, float64) {
	if t == nil {
		return false, "", 0
	}

	traffic, err := t.GetDailyTraffic(ip)
	if err != nil {
		log.Printf("[防刷墙] 获取IP %s 流量失败: %v", ip, err)
		return false, "", 0
	}

	if traffic > t.limitGB {
		trafficGB := float64(traffic) / float64(1024*1024*1024)
		reason := fmt.Sprintf("单日下载流量超过%dGB限制", t.limitGB/(1024*1024*1024))

		if err := db.AddIPToBlacklistWithSource(ip, reason, "local", "traffic"); err != nil {
			log.Printf("[防刷墙] 封禁IP %s 失败: %v", ip, err)
			return false, "", trafficGB
		}

		t.appendBanRecord(ip, reason, trafficGB)

		log.Printf("[防刷墙] IP %s 已被封禁，原因: %s，当日流量: %.2fGB，如有误封请联系 %s",
			ip, reason, trafficGB, t.appealContact)

		return true, reason, trafficGB
	}

	return false, "", 0
}

func (t *Tracker) appendBanRecord(ip, reason string, trafficGB float64) {
	if t.banRecordFile == "" {
		return
	}

	t.fileMutex.Lock()
	defer t.fileMutex.Unlock()

	fullPath := filepath.Join(t.storagePath, t.banRecordFile)

	f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[防刷墙] 打开封禁记录文件失败: %v", err)
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("%s | %s | %s | %.2f\n", ip, timestamp, reason, trafficGB)
	if _, err := f.WriteString(line); err != nil {
		log.Printf("[防刷墙] 写入封禁记录失败: %v", err)
	}
}

func (t *Tracker) GetTrafficLimitGB() int {
	if t == nil {
		return 5
	}
	return int(t.limitGB / (1024 * 1024 * 1024))
}

func (t *Tracker) GetAppealContact() string {
	if t == nil {
		return ""
	}
	return t.appealContact
}

func RecordTraffic(ip string, bytes int64) error {
	if defaultTracker == nil {
		return nil
	}
	return defaultTracker.RecordTraffic(ip, bytes)
}

func CheckAndBan(ip string) (bool, string, float64) {
	if defaultTracker == nil {
		return false, "", 0
	}
	return defaultTracker.CheckAndBan(ip)
}

type CountingWriter struct {
	Total int64
}

func (w *CountingWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.Total += int64(n)
	return n, nil
}

type CountingReader struct {
	reader io.Reader
	Total  int64
}

func NewCountingReader(r io.Reader) *CountingReader {
	return &CountingReader{reader: r}
}

func (r *CountingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.Total += int64(n)
	return n, err
}
