package traffic

import (
	"context"
	"fmt"
	"io"
	"lemwood_mirror/internal/db"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Tracker struct {
	limitGB       int64
	banRecordFile string
	appealContact string
	fileMutex     sync.Mutex
	banMutex      sync.Mutex
	pendingMutex  sync.Mutex
	pendingBytes  map[string]int64
	storagePath   string
	syncChan      chan struct{} // 用于异步触发文件同步
	ctx           context.Context
	cancel        context.CancelFunc
}

var defaultTracker *Tracker

func InitTracker(limitGB int, banRecordFile, appealContact, storagePath string) {
	ctx, cancel := context.WithCancel(context.Background())
	defaultTracker = &Tracker{
		limitGB:       int64(limitGB) * 1024 * 1024 * 1024,
		banRecordFile: banRecordFile,
		appealContact: appealContact,
		pendingBytes:  make(map[string]int64),
		storagePath:   storagePath,
		syncChan:      make(chan struct{}, 1),
		ctx:           ctx,
		cancel:        cancel,
	}
	// limitGB 为 0 时禁用防刷墙
	if limitGB > 0 && defaultTracker.banRecordFile != "" {
		defaultTracker.initBanRecordFile()
		go defaultTracker.syncWorker()
	}
}

func (t *Tracker) syncWorker() {
	const debounceDuration = 2 * time.Second
	var timer *time.Timer

	for {
		select {
		case <-t.syncChan:
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounceDuration, func() {
				if err := t.SyncBanRecordFile(); err != nil {
					log.Printf("[防刷墙] 异步同步封禁记录文件失败: %v", err)
				}
			})
		case <-t.ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}

func CloseTracker() {
	if defaultTracker != nil && defaultTracker.cancel != nil {
		defaultTracker.cancel()
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

// EstimateTransferBytes returns the conservative byte estimate for a request.
func EstimateTransferBytes(fileSize int64, rangeHeader string) int64 {
	if fileSize <= 0 {
		return 0
	}

	rangeHeader = strings.TrimSpace(rangeHeader)
	if rangeHeader == "" || !strings.HasPrefix(rangeHeader, "bytes=") {
		return fileSize
	}

	spec := strings.TrimSpace(strings.TrimPrefix(rangeHeader, "bytes="))
	if spec == "" || strings.Contains(spec, ",") {
		return fileSize
	}

	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return fileSize
	}

	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])

	switch {
	case startStr == "" && endStr == "":
		return fileSize
	case startStr == "":
		suffixLen, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || suffixLen <= 0 {
			return fileSize
		}
		if suffixLen > fileSize {
			return fileSize
		}
		return suffixLen
	default:
		start, err := strconv.ParseInt(startStr, 10, 64)
		if err != nil || start < 0 || start >= fileSize {
			return fileSize
		}
		if endStr == "" {
			return fileSize - start
		}
		end, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || end < start {
			return fileSize
		}
		if end >= fileSize {
			end = fileSize - 1
		}
		return end - start + 1
	}
}

func (t *Tracker) ReserveTraffic(ip string, estimatedBytes int64) (bool, int64, int64, string) {
	if t == nil {
		return true, 0, estimatedBytes, ""
	}
	if estimatedBytes < 0 {
		estimatedBytes = 0
	}
	if t.limitGB == 0 || estimatedBytes == 0 {
		currentBytes, err := t.GetDailyTraffic(ip)
		if err != nil {
			log.Printf("[防刷墙] 获取IP %s 流量失败: %v", ip, err)
			return false, 0, 0, "获取当日流量失败"
		}
		return true, currentBytes, currentBytes + estimatedBytes, ""
	}

	t.pendingMutex.Lock()
	defer t.pendingMutex.Unlock()

	currentBytes, err := t.GetDailyTraffic(ip)
	if err != nil {
		log.Printf("[防刷墙] 获取IP %s 流量失败: %v", ip, err)
		return false, 0, 0, "获取当日流量失败"
	}

	pendingBytes := t.pendingBytes[ip]
	projectedBytes := currentBytes + pendingBytes + estimatedBytes
	if projectedBytes > t.limitGB {
		reason := fmt.Sprintf(
			"单日下载流量超过%dGB限制（当前 %.2fGB，预计 %.2fGB）",
			t.limitGB/(1024*1024*1024),
			ToGB(currentBytes+pendingBytes),
			ToGB(projectedBytes),
		)
		return false, currentBytes, projectedBytes, reason
	}

	t.pendingBytes[ip] = pendingBytes + estimatedBytes
	return true, currentBytes, projectedBytes, ""
}

func (t *Tracker) releasePending(ip string, estimatedBytes int64) {
	if t == nil || estimatedBytes <= 0 {
		return
	}

	t.pendingMutex.Lock()
	defer t.pendingMutex.Unlock()

	remaining := t.pendingBytes[ip] - estimatedBytes
	if remaining <= 0 {
		delete(t.pendingBytes, ip)
		return
	}
	t.pendingBytes[ip] = remaining
}

func (t *Tracker) FinalizeTraffic(ip string, estimatedBytes int64, actualBytes int64) (bool, string, float64, error) {
	if t == nil {
		return false, "", 0, nil
	}
	defer t.releasePending(ip, estimatedBytes)

	if actualBytes > 0 {
		if err := t.RecordTraffic(ip, actualBytes); err != nil {
			return false, "", 0, err
		}
	}

	if t.limitGB == 0 {
		return false, "", 0, nil
	}

	banned, reason, trafficGB := t.CheckAndBan(ip)
	return banned, reason, trafficGB, nil
}

// ToGB 将字节转换为 GB
func ToGB(bytes int64) float64 {
	return float64(bytes) / (1024 * 1024 * 1024)
}

func (t *Tracker) CheckAndBan(ip string) (bool, string, float64) {
	if t == nil || t.limitGB == 0 {
		return false, "", 0
	}

	t.banMutex.Lock()
	defer t.banMutex.Unlock()

	if db.IsIPBlacklisted(ip) {
		return false, "", 0
	}

	traffic, err := t.GetDailyTraffic(ip)
	if err != nil {
		log.Printf("[防刷墙] 获取IP %s 流量失败: %v", ip, err)
		return false, "", 0
	}

	if traffic > t.limitGB {
		trafficGB := ToGB(traffic)
		reason := fmt.Sprintf("单日下载流量超过%dGB限制", t.limitGB/(1024*1024*1024))

		if err := db.AddIPToBlacklistWithSource(ip, reason, "local", "traffic"); err != nil {
			log.Printf("[防刷墙] 封禁IP %s 失败: %v", ip, err)
			return false, "", trafficGB
		}

		t.TriggerSync()

		log.Printf("[防刷墙] IP %s 已被封禁，原因: %s，当日流量: %.2fGB，如有误封请联系 %s",
			ip, reason, trafficGB, t.appealContact)

		return true, reason, trafficGB
	}

	return false, "", 0
}

// TriggerSync 异步触发文件同步
func (t *Tracker) TriggerSync() {
	if t == nil || t.syncChan == nil {
		return
	}
	select {
	case t.syncChan <- struct{}{}:
	default:
		// 如果 channel 已满（即已有同步请求正在排队或 debounce 中），则跳过
	}
}

// SyncBanRecordFile 从数据库重新生成封禁记录文件，确保与数据库同步并去重
func (t *Tracker) SyncBanRecordFile() error {
	if t == nil || t.banRecordFile == "" {
		return nil
	}

	blacklist, err := db.GetLocalIPBlacklist()
	if err != nil {
		return fmt.Errorf("获取本地黑名单失败: %w", err)
	}

	header := fmt.Sprintf(`# IP封禁记录 - 公开数据
# 格式: IP | 封禁时间 | 封禁理由 | 当日流量(GB)
# 如有误封，请加入 %s 进行申诉

`, t.appealContact)

	var content strings.Builder
	content.WriteString(header)

	for _, item := range blacklist {
		ip := item["ip"]
		reason := item["reason"]
		createdAtStr := item["created_at"]

		createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
		if err != nil {
			createdAt, err = time.Parse(time.RFC3339, createdAtStr)
		}

		timestamp := createdAtStr
		date := time.Now().Format("2006-01-02")
		if err == nil {
			timestamp = createdAt.Format("2006-01-02 15:04:05")
			date = createdAt.Format("2006-01-02")
		}

		traffic, _ := db.GetTrafficOnDate(ip, date)
		trafficGB := ToGB(traffic)

		line := fmt.Sprintf("%s | %s | %s | %.2f\n", ip, timestamp, reason, trafficGB)
		content.WriteString(line)
	}

	contentBytes := []byte(content.String())
	fullPath := filepath.Join(t.storagePath, t.banRecordFile)

	t.fileMutex.Lock()
	err = os.WriteFile(fullPath, contentBytes, 0644)
	t.fileMutex.Unlock()

	if err != nil {
		return fmt.Errorf("更新封禁记录文件失败: %w", err)
	}

	return nil
}

// SyncBanRecord 暴露全局异步同步函数
func SyncBanRecord() error {
	if defaultTracker == nil {
		return nil
	}
	defaultTracker.TriggerSync()
	return nil
}

// SyncBanRecordNow 暴露全局立即同步函数
func SyncBanRecordNow() error {
	if defaultTracker == nil {
		return nil
	}
	return defaultTracker.SyncBanRecordFile()
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

func ReserveTraffic(ip string, estimatedBytes int64) (bool, int64, int64, string) {
	if defaultTracker == nil {
		return true, 0, estimatedBytes, ""
	}
	return defaultTracker.ReserveTraffic(ip, estimatedBytes)
}

func FinalizeTraffic(ip string, estimatedBytes int64, actualBytes int64) (bool, string, float64, error) {
	if defaultTracker == nil {
		return false, "", 0, nil
	}
	return defaultTracker.FinalizeTraffic(ip, estimatedBytes, actualBytes)
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
