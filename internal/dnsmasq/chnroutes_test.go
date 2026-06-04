package dnsmasq

import (
	"strings"
	"testing"
)

func TestFilterChnroutesBody(t *testing.T) {
	body, err := filterChnroutesBody(strings.NewReader("# comment\n1.1.1.0/24\n\n# tail\n2.2.2.0/24\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "1.1.1.0/24\n") {
		t.Fatalf("missing cidr: %q", body)
	}
	if strings.Contains(string(body), "#") {
		t.Fatal("comments should be stripped")
	}
	if countChnrouteLines(string(body)) != 2 {
		t.Fatalf("expected 2 entries, got %d", countChnrouteLines(string(body)))
	}
}

func TestChnroutesFileInfoMissing(t *testing.T) {
	info := ChnroutesFileInfo("/nonexistent/qosnat2-chnroutes-test.txt")
	if info.Exists {
		t.Fatal("expected missing file")
	}
}

func TestValidateChnroutesPath(t *testing.T) {
	if _, err := ValidateChnroutesPath("/etc/qosnat2/chnroutes.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateChnroutesPath("/etc/passwd"); err == nil {
		t.Fatal("expected reject outside /etc/qosnat2")
	}
}
