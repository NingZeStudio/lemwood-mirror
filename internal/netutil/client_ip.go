package netutil

import (
	"net"
	"net/http"
	"strings"
)

// ExtractClientIP returns the canonical client IP from common proxy headers.
func ExtractClientIP(r *http.Request) string {
	candidates := []string{
		firstNonEmpty(strings.Split(r.Header.Get("X-Forwarded-For"), ",")),
		r.Header.Get("X-Real-IP"),
		r.RemoteAddr,
	}

	for _, candidate := range candidates {
		if ip := normalizeIP(candidate); ip != "" {
			return ip
		}
	}

	return ""
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}

	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")

	return value
}
