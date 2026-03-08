package blacklist

import (
	"bufio"
	"io"
	"lemwood_mirror/internal/db"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	externalIPs     map[string]bool
	externalIPsMu   sync.RWMutex
	lastSyncTime    time.Time
)

func init() {
	externalIPs = make(map[string]bool)
}

func SyncExternalBlacklist(url string) error {
	if url == "" {
		return nil
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[黑名单同步] 获取外部黑名单失败，状态码: %d", resp.StatusCode)
		return nil
	}

	var ips []string
	var newExternalIPs = make(map[string]bool)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, ":") {
			line = strings.Split(line, ":")[0]
		}
		if line != "" {
			ips = append(ips, line)
			newExternalIPs[line] = true
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return err
	}

	externalIPsMu.Lock()
	externalIPs = newExternalIPs
	lastSyncTime = time.Now()
	externalIPsMu.Unlock()

	if err := db.AddExternalBlacklist(ips); err != nil {
		return err
	}

	log.Printf("[黑名单同步] 成功同步 %d 个外部黑名单IP", len(ips))
	return nil
}

func IsExternalBlacklisted(ip string) bool {
	externalIPsMu.RLock()
	defer externalIPsMu.RUnlock()
	return externalIPs[ip]
}

func GetExternalBlacklistCount() int {
	externalIPsMu.RLock()
	defer externalIPsMu.RUnlock()
	return len(externalIPs)
}

func GetLastSyncTime() time.Time {
	externalIPsMu.RLock()
	defer externalIPsMu.RUnlock()
	return lastSyncTime
}
