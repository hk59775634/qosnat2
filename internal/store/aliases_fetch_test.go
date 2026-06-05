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
