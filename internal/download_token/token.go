package download_token

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
	stop   chan struct{}
}

func NewManager() *Manager {
	m := &Manager{
		ttl:  defaultTTL,
		stop: make(chan struct{}),
	}
	go m.cleanupExpired()
	return m
}

func (m *Manager) Close() {
	close(m.stop)
}

func (m *Manager) Generate(entry TokenEntry) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	entry.ExpiresAt = time.Now().Add(m.ttl)
	m.tokens.Store(token, entry)
	return token, nil
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

func (m *Manager) Consume(token string) bool {
	_, loaded := m.tokens.LoadAndDelete(token)
	return loaded
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
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.tokens.Range(func(key, value interface{}) bool {
				entry := value.(TokenEntry)
				if time.Now().After(entry.ExpiresAt) {
					m.tokens.Delete(key)
				}
				return true
			})
		}
	}
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
