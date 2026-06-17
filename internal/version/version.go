// Package version provides unified SemVer-like version comparison
// used by both the server (launcher index) and selfupdate subsystems.
package version

import (
	"fmt"
	"strings"
)

// IsStable reports whether a version string represents a stable release
// (i.e. not alpha, beta, rc, snapshot, pre, preview, or dev).
func IsStable(v string) bool {
	vLower := strings.ToLower(v)
	keywords := []string{"alpha", "beta", "rc", "snapshot", "pre", "dev", "preview"}
	for _, k := range keywords {
		if strings.Contains(vLower, k) {
			return false
		}
	}
	// 额外检查：如果包含横杠，通常也是非稳定版（如 1.2.3-v1）
	// 但有些启动器可能使用横杠作为正常版本号的一部分，所以以关键词优先
	return true
}

// SplitPreRelease splits "1.2.3-beta.1" into core "1.2.3" and suffix "beta.1".
func SplitPreRelease(v string) (string, string) {
	if idx := strings.Index(v, "-"); idx >= 0 {
		return v[:idx], v[idx+1:]
	}
	return v, ""
}

func parseFirstInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// Compare returns 1 if v1 > v2, -1 if v1 < v2, 0 if equal.
// Versions are compared segment by segment with numeric priority;
// a SemVer-style pre-release suffix (after "-") is ranked lower than the
// same version without a suffix.
func Compare(v1, v2 string) int {
	if v1 == v2 {
		return 0
	}

	v1Core, v1Pre := SplitPreRelease(strings.TrimPrefix(v1, "v"))
	v2Core, v2Pre := SplitPreRelease(strings.TrimPrefix(v2, "v"))

	parts1 := strings.Split(v1Core, ".")
	parts2 := strings.Split(v2Core, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 string
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		if p1 == p2 {
			continue
		}

		n1, err1 := parseFirstInt(p1)
		n2, err2 := parseFirstInt(p2)

		if err1 == nil && err2 == nil {
			if n1 > n2 {
				return 1
			}
			if n1 < n2 {
				return -1
			}
			// 数字部分相同，按字符串字典序（如 2.0.0_beta-1 vs 2.0.0_beta-2）
			if p1 > p2 {
				return 1
			}
			if p1 < p2 {
				return -1
			}
		} else {
			if p1 > p2 {
				return 1
			}
			if p1 < p2 {
				return -1
			}
		}
	}

	// SemVer: 无 pre-release 的版本高于带 pre-release 的相同核心版本
	if v1Pre == "" && v2Pre != "" {
		return 1
	}
	if v1Pre != "" && v2Pre == "" {
		return -1
	}
	if v1Pre != v2Pre {
		if v1Pre > v2Pre {
			return 1
		}
		return -1
	}
	return 0
}
