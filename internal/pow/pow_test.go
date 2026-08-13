package pow

import (
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

func testManager(t *testing.T, ttl time.Duration) *Manager {
	t.Helper()
	return NewManager(Config{
		Secret:     "test-secret",
		Cost:       200, // 测试用低 cost 加速求解
		KeyLength:  32,
		Difficulty: 10, // ~1K 次迭代
		TTL:        ttl,
	})
}

// solve 暴力搜索一个满足前导零位数的 counter。
// DerivedKey 用无填充 base64url 编码（对齐浏览器/CLI 客户端实际行为）。
func solve(p ChallengeParameters) (Solution, bool) {
	salt, _ := base64.URLEncoding.DecodeString(p.Salt)
	for counter := 0; counter < 10_000_000; counter++ {
		dk := pbkdf2.Key([]byte(strconv.Itoa(counter)), salt, p.Cost, p.KeyLength, sha256.New)
		if leadingZeroBits(dk) >= p.Difficulty {
			return Solution{Counter: counter, DerivedKey: base64.RawURLEncoding.EncodeToString(dk)}, true
		}
	}
	return Solution{}, false
}

func TestCreateAndVerifyRoundTrip(t *testing.T) {
	m := testManager(t, time.Minute)
	defer m.Close()

	ch, err := m.CreateChallenge("fcl/1.0.0/a.apk", "1.2.3.4", "web")
	if err != nil {
		t.Fatalf("CreateChallenge error = %v", err)
	}

	sol, ok := solve(ch.Parameters)
	if !ok {
		t.Fatal("solve failed to find a counter")
	}

	if err := m.VerifyAndConsume(Payload{Challenge: ch, Solution: sol}, "fcl/1.0.0/a.apk"); err != nil {
		t.Fatalf("VerifyAndConsume error = %v", err)
	}
}

func TestVerifyRejectsWrongSolution(t *testing.T) {
	m := testManager(t, time.Minute)
	defer m.Close()

	ch, _ := m.CreateChallenge("fcl/1.0.0/a.apk", "1.2.3.4", "web")

	// 故意提交一个错误 counter（几乎不可能满足）
	err := m.VerifyAndConsume(Payload{Challenge: ch, Solution: Solution{Counter: 0, DerivedKey: ""}}, "fcl/1.0.0/a.apk")
	if err != ErrSolutionInvalid {
		t.Fatalf("expected ErrSolutionInvalid, got %v", err)
	}

	// 错误答案后挑战仍可重试
	sol, ok := solve(ch.Parameters)
	if !ok {
		t.Fatal("solve failed")
	}
	if err := m.VerifyAndConsume(Payload{Challenge: ch, Solution: sol}, "fcl/1.0.0/a.apk"); err != nil {
		t.Fatalf("retry after wrong answer error = %v", err)
	}
}

func TestVerifyRejectsReplay(t *testing.T) {
	m := testManager(t, time.Minute)
	defer m.Close()

	ch, _ := m.CreateChallenge("fcl/1.0.0/a.apk", "1.2.3.4", "web")
	sol, ok := solve(ch.Parameters)
	if !ok {
		t.Fatal("solve failed")
	}
	if err := m.VerifyAndConsume(Payload{Challenge: ch, Solution: sol}, "fcl/1.0.0/a.apk"); err != nil {
		t.Fatalf("first verify error = %v", err)
	}
	// 同一挑战重放应被拒
	if err := m.VerifyAndConsume(Payload{Challenge: ch, Solution: sol}, "fcl/1.0.0/a.apk"); err != ErrChallengeConsumed {
		t.Fatalf("expected ErrChallengeConsumed on replay, got %v", err)
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	m := testManager(t, time.Minute)
	defer m.Close()

	ch, _ := m.CreateChallenge("fcl/1.0.0/a.apk", "1.2.3.4", "web")
	// 篡改 difficulty 但不改签名
	tampered := ch
	tampered.Parameters.Difficulty = 1
	err := m.VerifyAndConsume(Payload{Challenge: tampered, Solution: Solution{Counter: 0}}, "fcl/1.0.0/a.apk")
	if err != ErrSignatureInvalid {
		t.Fatalf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestVerifyRejectsFileBindingMismatch(t *testing.T) {
	m := testManager(t, time.Minute)
	defer m.Close()

	ch, _ := m.CreateChallenge("fcl/1.0.0/a.apk", "1.2.3.4", "web")
	sol, _ := solve(ch.Parameters)
	err := m.VerifyAndConsume(Payload{Challenge: ch, Solution: sol}, "hmcl/3.0/b.jar")
	if err != ErrFileBindingMismatch {
		t.Fatalf("expected ErrFileBindingMismatch, got %v", err)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	m := testManager(t, 100*time.Millisecond)
	defer m.Close()

	ch, _ := m.CreateChallenge("fcl/1.0.0/a.apk", "1.2.3.4", "web")
	time.Sleep(200 * time.Millisecond)
	sol, _ := solve(ch.Parameters)
	err := m.VerifyAndConsume(Payload{Challenge: ch, Solution: sol}, "fcl/1.0.0/a.apk")
	if err != ErrChallengeExpired {
		t.Fatalf("expected ErrChallengeExpired, got %v", err)
	}
}

func TestLeadingZeroBits(t *testing.T) {
	cases := []struct {
		b    []byte
		want int
	}{
		{[]byte{0x00}, 8},
		{[]byte{0x00, 0x00}, 16},
		{[]byte{0x00, 0x01}, 15},
		{[]byte{0x80}, 0},
		{[]byte{0x40}, 1},
		{[]byte{0x20}, 2},
		{[]byte{0x10}, 3},
		{[]byte{0x01}, 7},
		{[]byte{0xFF}, 0},
	}
	for _, c := range cases {
		if got := leadingZeroBits(c.b); got != c.want {
			t.Errorf("leadingZeroBits(% x) = %d, want %d", c.b, got, c.want)
		}
	}
}
