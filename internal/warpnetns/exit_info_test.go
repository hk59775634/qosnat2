package warpnetns

import "testing"

func TestParseCloudflareTrace(t *testing.T) {
	raw := `fl=1176f60
h=1.1.1.1
ip=1.2.3.4
colo=LAX
loc=US
warp=on
gateway=off
`
	got := parseCloudflareTrace([]byte(raw))
	if got.IP != "1.2.3.4" || got.Country != "US" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.Region != "LAX" {
		t.Fatalf("region: %q", got.Region)
	}
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
}

func TestParseCloudflareTrace_missingIP(t *testing.T) {
	got := parseCloudflareTrace([]byte("loc=US\ncolo=HKG\n"))
	if got.Error == "" {
		t.Fatalf("expected error, got %+v", got)
	}
}
