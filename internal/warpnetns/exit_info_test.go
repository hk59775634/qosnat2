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
	if got.Warp != "on" || got.Gateway != "off" {
		t.Fatalf("warp/gateway: %+v", got)
	}
	if got.WarpTier != "standard" {
		t.Fatalf("warp_tier: %q", got.WarpTier)
	}
	if got.Org != "warp=on, gateway=off" {
		t.Fatalf("org: %q", got.Org)
	}
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
}

func TestParseCloudflareTrace_plus(t *testing.T) {
	raw := `ip=5.6.7.8
loc=US
colo=SJC
warp=plus
gateway=on
`
	got := parseCloudflareTrace([]byte(raw))
	if got.WarpTier != "plus" {
		t.Fatalf("warp_tier: %q", got.WarpTier)
	}
	if got.Warp != "plus" || got.Gateway != "on" {
		t.Fatalf("warp/gateway: %+v", got)
	}
}

func TestParseCloudflareTrace_missingIP(t *testing.T) {
	got := parseCloudflareTrace([]byte("loc=US\ncolo=HKG\n"))
	if got.Error == "" {
		t.Fatalf("expected error, got %+v", got)
	}
}

func TestNormalizeWarpTier(t *testing.T) {
	cases := map[string]string{
		"":      "off",
		"off":   "off",
		"on":    "standard",
		"plus":  "plus",
		"2xc":   "2xc",
		"PLUS":  "plus",
		"weird": "weird",
	}
	for in, want := range cases {
		if got := NormalizeWarpTier(in); got != want {
			t.Fatalf("NormalizeWarpTier(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseRegistrationAccountType(t *testing.T) {
	raw := `Account type: Unlimited
Device ID: abc
`
	if got := parseRegistrationAccountType([]byte(raw)); got != "Unlimited" {
		t.Fatalf("got %q", got)
	}
}
