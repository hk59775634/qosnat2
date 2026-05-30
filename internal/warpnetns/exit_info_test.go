package warpnetns

import "testing"

func TestParseIPInfoJSON(t *testing.T) {
	raw := `{"ip":"1.2.3.4","city":"Los Angeles","region":"California","country":"US","org":"AS13335 Cloudflare, Inc."}`
	got := parseIPInfoJSON([]byte(raw))
	if got.IP != "1.2.3.4" || got.Country != "US" || got.City != "Los Angeles" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
}

func TestParseIPInfoJSON_error(t *testing.T) {
	got := parseIPInfoJSON([]byte(`{"error":"rate limited"}`))
	if got.Error != "rate limited" {
		t.Fatalf("got %+v", got)
	}
}
