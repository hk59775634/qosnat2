package releasecatalog

import "testing"

func TestMirrorURLs(t *testing.T) {
	direct := "https://raw.githubusercontent.com/hk59775634/qosnat2/main/releases/qosnat2-versions.json"
	urls := MirrorURLs(direct)
	if len(urls) != 3 {
		t.Fatalf("want 3 urls, got %d: %v", len(urls), urls)
	}
	if urls[0] != direct {
		t.Fatalf("first url: %q", urls[0])
	}
	want1 := "https://v4.gh-proxy.org/" + direct
	want2 := "https://cdn.gh-proxy.org/" + direct
	if urls[1] != want1 || urls[2] != want2 {
		t.Fatalf("proxy urls: %v", urls[1:])
	}
}

func TestQosnatDownloadURLs(t *testing.T) {
	urls := QosnatDownloadURLs("2026052801")
	if len(urls) != 3 {
		t.Fatalf("want 3, got %d", len(urls))
	}
	if urls[0] != "https://github.com/hk59775634/qosnat2/releases/download/v2026052801/qosnat2-linux-amd64.tar.gz" {
		t.Fatalf("direct: %q", urls[0])
	}
}

func TestMirrorURLsEmpty(t *testing.T) {
	if got := MirrorURLs(""); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
