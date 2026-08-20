package traffic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lemwood_mirror/internal/db"
	"log"
	"math"
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
	recordTrafficFunc   func(string, int64) error
	getDailyTrafficFunc func(string) (int64, error)
	getTrafficOnDateFunc func(string, string) (int64, error)
}

var defaultTracker *Tracker

func InitTracker(limitGB int, banRecordFile, appealContact, storagePath string) {
	defaultTracker = newTracker(limitGB, banRecordFile, appealContact, storagePath,
		func(string, int64) error { return nil }, // served 字节现由 download_events 承载，不再写聚合表
		db.GetDailyServedByIPFromEventsToday, // 防刷墙按 IP 当日 served 读事件表
		db.GetDailyServedByIPFromEvents) // 封禁记录文件的按日流量也取自事件表
}

func newTracker(limitGB int, banRecordFile, appealContact, storagePath string, recordTrafficFunc func(string, int64) error, getDailyTrafficFunc func(string) (int64, error), getTrafficOnDateFunc func(string, string) (int64, error)) *Tracker {
	ctx, cancel := context.WithCancel(context.Background())
	tracker := &Tracker{
		limitGB:       int64(limitGB) * 1024 * 1024 * 1024,
		banRecordFile: banRecordFile,
		appealContact: appealContact,
		pendingBytes:  make(map[string]int64),
		storagePath:   storagePath,
		syncChan:      make(chan struct{}, 1),
		ctx:           ctx,
		cancel:        cancel,
		recordTrafficFunc: recordTrafficFunc,
		getDailyTrafficFunc: getDailyTrafficFunc,
		getTrafficOnDateFunc: getTrafficOnDateFunc,
	}
	if limitGB > 0 && tracker.banRecordFile != "" {
		tracker.initBanRecordFile()
		go tracker.syncWorker()
	}
	return tracker
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
		content, err := t.buildBanRecordJSON(nil)
		if err != nil {
			log.Printf("[防刷墙] 生成初始封禁记录失败: %v", err)
			return
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			log.Printf("[防刷墙] 初始化封禁记录文件失败: %v", err)
		}
	}
}

func (t *Tracker) RecordTraffic(ip string, bytes int64) error {
	if t == nil || t.recordTrafficFunc == nil {
		return nil
	}
	return t.recordTrafficFunc(ip, bytes)
}

func (t *Tracker) GetDailyTraffic(ip string) (int64, error) {
	if t == nil || t.getDailyTrafficFunc == nil {
		return 0, nil
	}
	return t.getDailyTrafficFunc(ip)
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

	// served 字节现由调用方（下载处理器）写入 download_events 状态表（全切口径）；
	// 此处不再写 ip_daily_traffic/daily_traffic，两张聚合表冻结为历史基线。
	// 为保证防刷墙 CheckAndBan 能读到本次下载字节，调用方须先 RecordDownloadEvent 再 FinalizeTraffic。
	_ = actualBytes

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

// banRecordEntry 是封禁记录文件中的单条记录。
type banRecordEntry struct {
	IP        string  `json:"ip"`
	Reason    string  `json:"reason"`
	Source    string  `json:"source"`
	BanType   string  `json:"ban_type"`
	CreatedAt string  `json:"created_at"`
	TrafficGB float64 `json:"traffic_gb"`
}

// banRecordFile 是公开封禁记录文件的 JSON 结构。
type banRecordFile struct {
	UpdatedAt      string           `json:"updated_at"`
	TrafficLimitGB int              `json:"traffic_limit_gb"`
	AppealContact  string           `json:"appeal_contact"`
	Count          int              `json:"count"`
	Records        []banRecordEntry `json:"records"`
}

// buildBanRecordJSON 将黑名单列表序列化为公开 JSON 内容。
// blacklist 为 nil 时生成空记录（用于初始化文件）。
func (t *Tracker) buildBanRecordJSON(blacklist []map[string]string) ([]byte, error) {
	records := make([]banRecordEntry, 0, len(blacklist))
	for _, item := range blacklist {
		ip := item["ip"]
		createdAtStr := item["created_at"]

		date := time.Now().Format("2006-01-02")
		if createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
			date = createdAt.Format("2006-01-02")
		} else if createdAt, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			date = createdAt.Format("2006-01-02")
		}

		traffic := int64(0)
		if t.getTrafficOnDateFunc != nil {
			traffic, _ = t.getTrafficOnDateFunc(ip, date)
		}

		records = append(records, banRecordEntry{
			IP:        ip,
			Reason:    item["reason"],
			Source:    item["source"],
			BanType:   item["ban_type"],
			CreatedAt: createdAtStr,
			TrafficGB: round2(ToGB(traffic)),
		})
	}

	doc := banRecordFile{
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
		TrafficLimitGB: t.GetTrafficLimitGB(),
		AppealContact:  t.appealContact,
		Count:          len(records),
		Records:        records,
	}

	content, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// SyncBanRecordFile 从数据库重新生成封禁记录文件（JSON），确保与数据库同步并去重
func (t *Tracker) SyncBanRecordFile() error {
	if t == nil || t.banRecordFile == "" {
		return nil
	}

	blacklist, err := db.GetLocalIPBlacklist()
	if err != nil {
		return fmt.Errorf("获取本地黑名单失败: %w", err)
	}

	content, err := t.buildBanRecordJSON(blacklist)
	if err != nil {
		return fmt.Errorf("序列化封禁记录失败: %w", err)
	}

	fullPath := filepath.Join(t.storagePath, t.banRecordFile)

	t.fileMutex.Lock()
	err = os.WriteFile(fullPath, content, 0644)
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
