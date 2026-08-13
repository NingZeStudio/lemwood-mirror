// Package pow 实现一个 ALTCHA 风格的 PBKDF2-SHA256 Proof-of-Work 挑战/校验，
// 用于替代第三方人机验证（极验）自动验证客户端正规性。
//
// 挑战态保存在主节点内存中（对齐 PoW实现.md §1.10 与 MapleMirror §5.1：挑战不落库）。
// 状态机：open -> issuing -> consumed；错误答案回退 open 允许重试；成功后 consumed 单次防重放。
//
// 协议（自实现，服务端 Go + 浏览器 Web Crypto 双端约定）：
//
//	challenge = { parameters:{algorithm, nonce, salt, cost, keyLength, difficulty, expiresAt, data}, signature }
//	solve     = 迭代 counter=0,1,2,... 计算 dk = PBKDF2-SHA256(password=str(counter), salt, cost, keyLength)
//	            直到 leadingZeroBits(dk) >= difficulty，提交 solution={counter, derivedKey}
//	verify    = 服务端用同一参数重算 PBKDF2，校验前导零位数，再校验签名/状态/过期/文件绑定并消费挑战
//
// 签名 = HMAC-SHA256(canonicalJSON(parameters), secret)，防止客户端篡改 difficulty/cost 等参数。
// 文件绑定：parameters.data.file_path 在签发与校验时都必须与请求文件一致，防止跨资产复用挑战。
package pow

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const (
	AlgorithmPBKDF2 = "PBKDF2-SHA256"
	defaultCost     = 5000
	defaultKeyLen   = 32
	defaultDifficulty = 14 // 前导零位数；~16K 次 PBKDF2 期望迭代
	defaultTTL      = 2 * time.Minute
	saltLen         = 16
	nonceLen        = 16
)

// 错误哨兵。
var (
	ErrChallengeNotFound   = errors.New("pow: challenge not found or expired")
	ErrChallengeConsumed   = errors.New("pow: challenge already consumed or in progress")
	ErrChallengeExpired    = errors.New("pow: challenge expired")
	ErrSolutionInvalid     = errors.New("pow: solution invalid")
	ErrSignatureInvalid    = errors.New("pow: challenge signature invalid")
	ErrFileBindingMismatch = errors.New("pow: challenge file binding mismatch")
)

// Manager 管理内存中的 PoW 挑战。线程安全。
type Manager struct {
	secret     []byte
	algorithm  string
	cost       int
	keyLength  int
	difficulty int
	ttl        time.Duration

	mu         sync.Mutex
	challenges map[string]*entry
	stop       chan struct{}
}

type entry struct {
	params     ChallengeParameters
	filePath   string
	clientIP   string
	sourceKind string // web | api
	state      string // open | issuing | consumed
	createdAt  time.Time
	expiresAt  time.Time
}

// ChallengeParameters 是挑战的可公开参数。客户端据此求解。
type ChallengeParameters struct {
	Algorithm  string                 `json:"algorithm"`
	Nonce      string                 `json:"nonce"`     // 挑战 ID（hex）
	Salt       string                 `json:"salt"`      // base64url
	Cost       int                    `json:"cost"`      // PBKDF2 迭代数
	KeyLength  int                    `json:"keyLength"` // 派生密钥字节数
	Difficulty int                    `json:"difficulty"`
	ExpiresAt  int64                  `json:"expiresAt"` // unix 秒
	Data       map[string]interface{} `json:"data,omitempty"`
}

// Challenge 是下发给客户端的挑战（参数 + 签名）。
type Challenge struct {
	Parameters ChallengeParameters `json:"parameters"`
	Signature  string              `json:"signature"` // hex HMAC-SHA256
}

// Solution 是客户端提交的解。
type Solution struct {
	Counter    int    `json:"counter"`
	DerivedKey string `json:"derivedKey"` // base64url，可选；服务端按 counter 重算
}

// Payload 是客户端提交的挑战+解。
type Payload struct {
	Challenge Challenge `json:"challenge"`
	Solution  Solution  `json:"solution"`
}

// Config 配置 PoW 管理器。
type Config struct {
	Secret     string // HMAC 签名密钥（hex 或任意字符串）
	Cost       int    // PBKDF2 迭代数；<=0 用默认
	KeyLength  int    // 派生密钥字节数；<=0 用默认
	Difficulty int    // 前导零位数；<=0 用默认
	TTL        time.Duration // 挑战有效期；<=0 用默认
}

// NewManager 创建管理器并启动过期清理协程。
func NewManager(cfg Config) *Manager {
	m := &Manager{
		secret:     []byte(cfg.Secret),
		algorithm:  AlgorithmPBKDF2,
		cost:       defaultCost,
		keyLength:  defaultKeyLen,
		difficulty: defaultDifficulty,
		ttl:        defaultTTL,
		challenges: make(map[string]*entry),
		stop:       make(chan struct{}),
	}
	if cfg.Cost > 0 {
		m.cost = cfg.Cost
	}
	if cfg.KeyLength > 0 {
		m.keyLength = cfg.KeyLength
	}
	if cfg.Difficulty > 0 {
		m.difficulty = cfg.Difficulty
	}
	if cfg.TTL > 0 {
		m.ttl = cfg.TTL
	}
	go m.cleanupLoop()
	return m
}

// Close 停止清理协程。可重复调用。
func (m *Manager) Close() {
	select {
	case <-m.stop:
	default:
		close(m.stop)
	}
}

// CreateChallenge 为指定文件签发一条新挑战。返回可公开的 Challenge。
// file_path 绑定进 data，授权时必须一致。
func (m *Manager) CreateChallenge(filePath, clientIP, sourceKind string) (Challenge, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return Challenge{}, fmt.Errorf("pow: read salt: %w", err)
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return Challenge{}, fmt.Errorf("pow: read nonce: %w", err)
	}
	now := time.Now()
	expires := now.Add(m.ttl)
	nonceHex := hexEncode(nonce)
	params := ChallengeParameters{
		Algorithm:  m.algorithm,
		Nonce:      nonceHex,
		Salt:       base64.URLEncoding.EncodeToString(salt),
		Cost:       m.cost,
		KeyLength:  m.keyLength,
		Difficulty: m.difficulty,
		ExpiresAt:  expires.Unix(),
		Data: map[string]interface{}{
			"file_path":   filePath,
			"client_ip":   clientIP,
			"source_kind": sourceKind,
		},
	}
	sig, err := m.sign(params)
	if err != nil {
		return Challenge{}, err
	}
	ch := Challenge{Parameters: params, Signature: sig}

	m.mu.Lock()
	m.challenges[nonceHex] = &entry{
		params:     params,
		filePath:   filePath,
		clientIP:   clientIP,
		sourceKind: sourceKind,
		state:      "open",
		createdAt:  now,
		expiresAt:  expires,
	}
	m.mu.Unlock()

	return ch, nil
}

// PublicConfig 返回客户端可见的 PoW 参数（用于 /pow/config 端点）。
type PublicConfig struct {
	Algorithm  string `json:"algorithm"`
	Cost       int    `json:"cost"`
	KeyLength  int    `json:"keyLength"`
	Difficulty int    `json:"difficulty"`
}

func (m *Manager) PublicConfig() PublicConfig {
	return PublicConfig{
		Algorithm:  m.algorithm,
		Cost:       m.cost,
		KeyLength:  m.keyLength,
		Difficulty: m.difficulty,
	}
}

// ChallengeCount 返回当前内存中的挑战数（诊断用）。
func (m *Manager) ChallengeCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.challenges)
}

// VerifyAndConsume 校验提交的解并消费挑战。成功返回 nil；失败返回具体错误哨兵。
// filePath 必须与签发时一致（文件绑定）。
//
// 错误答案不消费挑战，客户端可在过期前用同一挑战重试；并发提交同一挑战时，
// 后到的请求在锁内看到 issuing/consumed，返回 ErrChallengeConsumed。
func (m *Manager) VerifyAndConsume(payload Payload, filePath string) error {
	// 1. 校验签名（防篡改 difficulty/cost 等）
	sig, err := m.sign(payload.Challenge.Parameters)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(sig), []byte(payload.Challenge.Signature)) != 1 {
		return ErrSignatureInvalid
	}

	nonce := payload.Challenge.Parameters.Nonce

	m.mu.Lock()
	e, ok := m.challenges[nonce]
	if !ok {
		m.mu.Unlock()
		return ErrChallengeNotFound
	}
	if e.state != "open" {
		m.mu.Unlock()
		return ErrChallengeConsumed
	}
	if time.Now().After(e.expiresAt) {
		e.state = "consumed" // 过期即清理，不再可用
		m.mu.Unlock()
		return ErrChallengeExpired
	}
	// 文件绑定
	if e.filePath != filePath {
		m.mu.Unlock()
		return ErrFileBindingMismatch
	}
	// 进入 issuing，串行化同一挑战的并发提交
	e.state = "issuing"
	m.mu.Unlock()

	// 2. 重算 PBKDF2 并校验前导零位数（大整数计算不在锁内）
	valid := m.verifySolution(payload.Challenge.Parameters, payload.Solution)

	m.mu.Lock()
	defer m.mu.Unlock()
	if valid {
		e.state = "consumed"
		return nil
	}
	// 校验失败：回退 open 允许重试
	e.state = "open"
	return ErrSolutionInvalid
}

// Solve 暴力搜索一个满足前导零位数的 counter，返回解。供测试与 CLI 求解器复用。
// maxIterations<=0 时使用内部上限。未找到返回 false。
func Solve(p ChallengeParameters, maxIterations int) (Solution, bool) {
	salt, err := base64.URLEncoding.DecodeString(p.Salt)
	if err != nil || p.Cost <= 0 || p.KeyLength <= 0 || p.Difficulty < 0 {
		return Solution{}, false
	}
	if maxIterations <= 0 {
		maxIterations = 50_000_000
	}
	for counter := 0; counter < maxIterations; counter++ {
		dk := pbkdf2.Key([]byte(strconv.Itoa(counter)), salt, p.Cost, p.KeyLength, sha256.New)
		if leadingZeroBits(dk) >= p.Difficulty {
			return Solution{Counter: counter, DerivedKey: base64.URLEncoding.EncodeToString(dk)}, true
		}
	}
	return Solution{}, false
}

// verifySolution 重算派生密钥并检查前导零位数。同时校验 DerivedKey（若提供）一致。
func (m *Manager) verifySolution(p ChallengeParameters, s Solution) bool {
	salt, err := base64.URLEncoding.DecodeString(p.Salt)
	if err != nil {
		return false
	}
	if p.Cost <= 0 || p.KeyLength <= 0 || p.Difficulty < 0 {
		return false
	}
	dk := pbkdf2.Key([]byte(strconv.Itoa(s.Counter)), salt, p.Cost, p.KeyLength, sha256.New)
	if leadingZeroBits(dk) < p.Difficulty {
		return false
	}
	// 客户端 DerivedKey 为无填充 base64url；解码后与重算字节比较（容错带填充输入）。
	if s.DerivedKey != "" {
		clientKey, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s.DerivedKey, "="))
		if err != nil || subtle.ConstantTimeCompare(clientKey, dk) != 1 {
			return false
		}
	}
	return true
}

// sign 对参数计算 HMAC-SHA256 签名。基于 json.Marshal 的规范化编码
// （Go 结构体字段顺序固定、map 键字典序，输出确定性）。
func (m *Manager) sign(p ChallengeParameters) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("pow: marshal params: %w", err)
	}
	h := hmac.New(sha256.New, m.secret)
	h.Write(b)
	return hexEncode(h.Sum(nil)), nil
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.mu.Lock()
			now := time.Now()
			for id, e := range m.challenges {
				if now.After(e.expiresAt) || e.state == "consumed" {
					delete(m.challenges, id)
				}
			}
			m.mu.Unlock()
		}
	}
}

// leadingZeroBits 返回字节切片从 MSB 起的连续零位数。
func leadingZeroBits(b []byte) int {
	bits := 0
	for _, byt := range b {
		if byt == 0 {
			bits += 8
			continue
		}
		// 计算该字节前导零
		for i := 7; i >= 0; i-- {
			if byt&(1<<uint(i)) != 0 {
				return bits
			}
			bits++
		}
		return bits
	}
	return bits
}

// hexEncode 等价于 hex.EncodeToString，避免额外 import 冲突命名。
func hexEncode(b []byte) string {
	const hexChars = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexChars[v>>4]
		out[i*2+1] = hexChars[v&0x0f]
	}
	return string(out)
}
