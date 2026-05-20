package netutil

import (
	"net/http/httptest"
	"testing"
)

func TestExtractClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		expected   string
	}{
		{
			name:       "xff takes precedence",
			remoteAddr: "127.0.0.1:1234",
			xff:        " 203.0.113.10 , 198.51.100.2 ",
			xri:        "198.51.100.1",
			expected:   "203.0.113.10",
		},
		{
			name:       "xri used when xff empty",
			remoteAddr: "127.0.0.1:1234",
			xri:        "198.51.100.5",
			expected:   "198.51.100.5",
		},
		{
			name:       "remote addr trims port",
			remoteAddr: "198.51.100.20:4567",
			expected:   "198.51.100.20",
		},
		{
			name:       "ipv6 remote addr trims brackets and port",
			remoteAddr: "[2001:db8::1]:8080",
			expected:   "2001:db8::1",
		},
		{
			name:       "plain remote addr is returned as is",
			remoteAddr: "203.0.113.77",
			expected:   "203.0.113.77",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.xri != "" {
				req.Header.Set("X-Real-IP", tc.xri)
			}

			if got := ExtractClientIP(req); got != tc.expected {
				t.Fatalf("ExtractClientIP() = %q, want %q", got, tc.expected)
			}
		})
	}
}
