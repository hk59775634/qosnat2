package store

import "testing"

func TestParseCIDRListText_googleSample(t *testing.T) {
	text := `8.8.4.0/24
8.8.8.0/24
# comment
34.0.0.0/15
`
	got, err := ParseCIDRListText(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %v", got)
	}
}

func TestParseCIDRListText_empty(t *testing.T) {
	if _, err := ParseCIDRListText("\n# only comments\n"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseFQDNListText(t *testing.T) {
	text := `api.openai.com
# comment
claude.ai
API.OpenAI.COM
*.bad.com
`
	got, err := ParseFQDNListText(text)
	if err == nil {
		t.Fatal("expected wildcard line to fail")
	}
	text = `api.openai.com
# comment
claude.ai
API.OpenAI.COM
`
	got, err = ParseFQDNListText(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("dedupe: %v", got)
	}
	if got[0] != "api.openai.com" || got[1] != "claude.ai" {
		t.Fatalf("got %v", got)
	}
}

func TestParseFQDNListText_empty(t *testing.T) {
	if _, err := ParseFQDNListText("\n# only\n"); err == nil {
		t.Fatal("expected error")
	}
}
