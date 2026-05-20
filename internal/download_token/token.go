package download_token

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const (
	defaultTTL = 5 * time.Minute
)

type TokenEntry struct {
	FilePath  string
	ReturnURL string
	Source    string
	Flow      string
	ExpiresAt time.Time
}

type Manager struct {
	tokens sync.Map
	ttl    time.Duration
}

func NewManager() *Manager {
	m := &Manager{
		ttl: defaultTTL,
	}
	go m.cleanupExpired()
	return m
}

func (m *Manager) Generate(entry TokenEntry) string {
	token := generateToken()
	entry.ExpiresAt = time.Now().Add(m.ttl)
	m.tokens.Store(token, entry)
	return token
}

func (m *Manager) Validate(token string) (TokenEntry, bool) {
	value, ok := m.tokens.Load(token)
	if !ok {
		return TokenEntry{}, false
	}

	entry := value.(TokenEntry)
	if time.Now().After(entry.ExpiresAt) {
		m.tokens.Delete(token)
		return TokenEntry{}, false
	}

	m.tokens.Delete(token)
	return entry, true
}

func (m *Manager) Peek(token string) (TokenEntry, bool) {
	value, ok := m.tokens.Load(token)
	if !ok {
		return TokenEntry{}, false
	}

	entry := value.(TokenEntry)
	if time.Now().After(entry.ExpiresAt) {
		m.tokens.Delete(token)
		return TokenEntry{}, false
	}

	return entry, true
}

func (m *Manager) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		m.tokens.Range(func(key, value interface{}) bool {
			entry := value.(TokenEntry)
			if time.Now().After(entry.ExpiresAt) {
				m.tokens.Delete(key)
			}
			return true
		})
	}
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
