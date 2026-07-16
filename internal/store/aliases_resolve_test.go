package store

import "testing"

func TestNormalizeFQDN(t *testing.T) {
	got, err := NormalizeFQDN("API.OpenAI.COM.")
	if err != nil {
		t.Fatal(err)
	}
	if got != "api.openai.com" {
		t.Fatalf("got %q", got)
	}
	if _, err := NormalizeFQDN("*.openai.com"); err == nil {
		t.Fatal("expected wildcard rejection")
	}
	if _, err := NormalizeFQDN("localhost"); err == nil {
		t.Fatal("expected single-label rejection")
	}
}

func TestNormalizeAliasFQDN(t *testing.T) {
	a := &AliasSet{
		Name:    "ai_dst",
		Type:    "fqdn",
		Domains: []string{"api.openai.com", "API.OpenAI.COM", "claude.ai"},
	}
	if err := NormalizeAlias(a); err != nil {
		t.Fatal(err)
	}
	if len(a.Domains) != 2 {
		t.Fatalf("domains dedupe: %v", a.Domains)
	}
	if len(a.Members) != 0 {
		t.Fatalf("members should be empty before resolve: %v", a.Members)
	}
}

func TestNormalizeAliasFQDNWithURL(t *testing.T) {
	a := &AliasSet{Name: "x", Type: "fqdn", URL: "https://example.com/domains.txt"}
	if err := NormalizeAlias(a); err != nil {
		t.Fatal(err)
	}
	if a.URL != "https://example.com/domains.txt" {
		t.Fatalf("url: %q", a.URL)
	}
	if len(a.Domains) != 0 {
		t.Fatalf("domains should be empty until fetch: %v", a.Domains)
	}
}

func TestNormalizeAliasFQDNRequiresDomainsOrURL(t *testing.T) {
	a := &AliasSet{Name: "x", Type: "fqdn"}
	if err := NormalizeAlias(a); err == nil {
		t.Fatal("expected error")
	}
}

func TestRefreshAliasFromFQDN_localhost(t *testing.T) {
	// 127.0.0.1 is not a multi-label FQDN in Normalize; use a name that resolves in most environments.
	a := &AliasSet{Name: "dns", Type: "fqdn", Domains: []string{"one.one.one.one"}}
	if err := NormalizeAlias(a); err != nil {
		t.Fatal(err)
	}
	warn, err := RefreshAliasDynamic(a)
	if err != nil {
		t.Skipf("DNS resolve unavailable: %v", err)
	}
	_ = warn
	if len(a.Members) == 0 {
		t.Fatal("expected resolved members")
	}
	if a.ResolvedAt == "" {
		t.Fatal("expected resolved_at")
	}
}

func TestAliasNeedsDynamicRefresh(t *testing.T) {
	if !AliasNeedsDynamicRefresh(AliasSet{Type: "fqdn", Domains: []string{"a.example.com"}}) {
		t.Fatal("fqdn")
	}
	if !AliasNeedsDynamicRefresh(AliasSet{Type: "fqdn", URL: "https://x"}) {
		t.Fatal("fqdn url")
	}
	if !AliasNeedsDynamicRefresh(AliasSet{Type: "ipv4_addr", URL: "https://x"}) {
		t.Fatal("url")
	}
	if AliasNeedsDynamicRefresh(AliasSet{Type: "ipv4_addr", Members: []string{"1.1.1.1/32"}}) {
		t.Fatal("static")
	}
}
