package geoip

import (
	"testing"
)

func TestLookup(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	tests := []struct {
		ip      string
		wantOK  bool
		country string
	}{
		{"8.8.8.8", true, "United States"},
		{"223.5.5.5", true, "中国"},
		{"2408:4000::1", true, "中国"},
		{"127.0.0.1", false, ""},
	}

	for _, tc := range tests {
		country, region, city, ok := Lookup(tc.ip)
		t.Logf("%s => ok=%v country=%s region=%s city=%s", tc.ip, ok, country, region, city)
		if ok != tc.wantOK {
			t.Errorf("Lookup(%s) ok = %v, want %v", tc.ip, ok, tc.wantOK)
		}
		if ok && tc.country != "" && country != tc.country {
			t.Errorf("Lookup(%s) country = %s, want %s", tc.ip, country, tc.country)
		}
	}
}
