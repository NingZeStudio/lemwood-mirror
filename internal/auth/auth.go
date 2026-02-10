package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

var (
	tokens = make(map[string]time.Time)
	mu     sync.RWMutex
)

const tokenDuration = 24 * time.Hour

func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)

	mu.Lock()
	tokens[token] = time.Now().Add(tokenDuration)
	mu.Unlock()

	return token, nil
}

func ValidateToken(token string) bool {
	mu.RLock()
	expiry, ok := tokens[token]
	mu.RUnlock()

	if !ok {
		return false
	}

	if time.Now().After(expiry) {
		mu.Lock()
		delete(tokens, token)
		mu.Unlock()
		return false
	}

	return true
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func ValidateTOTP(code, secret string) bool {
	return totp.Validate(code, secret)
}

func CleanupTokens() {
	for {
		time.Sleep(1 * time.Hour)
		mu.Lock()
		for token, expiry := range tokens {
			if time.Now().After(expiry) {
				delete(tokens, token)
			}
		}
		mu.Unlock()
	}
}
