package releasecatalog

import "testing"

func TestMirrorURLs(t *testing.T) {
	direct := "https://raw.githubusercontent.com/hk59775634/qosnat2/main/releases/qosnat2-versions.json"
	urls := MirrorURLs(direct)
	if len(urls) != 3 {
		t.Fatalf("want 3 urls, got %d: %v", len(urls), urls)
	}
	want1 := "https://v4.gh-proxy.org/" + direct
	want2 := "https://cdn.gh-proxy.org/" + direct
	if urls[0] != want1 || urls[1] != want2 {
		t.Fatalf("proxy urls: %v", urls[:2])
	}
	if urls[2] != direct {
		t.Fatalf("direct fallback last: %q", urls[2])
	}
}

func TestQosnatDownloadURLs(t *testing.T) {
	urls := QosnatDownloadURLs("2026052801")
	if len(urls) != 3 {
		t.Fatalf("want 3, got %d", len(urls))
	}
	if urls[0] != "https://v4.gh-proxy.org/https://github.com/hk59775634/qosnat2/releases/download/v2026052801/qosnat2-linux-amd64.tar.gz" {
		t.Fatalf("v4 first: %q", urls[0])
	}
}

func TestNotesURL(t *testing.T) {
	got := NotesURL("2026053111")
	want := "https://raw.githubusercontent.com/hk59775634/qosnat2/main/releases/notes/2026053111.md"
	if got != want {
		t.Fatalf("NotesURL: got %q want %q", got, want)
	}
}

func TestToReleaseMapsSummary(t *testing.T) {
	maps := ToReleaseMaps([]VersionEntry{{
		ID: "2026053111", Tag: "v2026053111", Summary: "test summary",
	}})
	if len(maps) != 1 {
		t.Fatalf("want 1 map")
	}
	if maps[0]["summary"] != "test summary" {
		t.Fatalf("summary: %v", maps[0]["summary"])
	}
	if maps[0]["notes_url"] == "" {
		t.Fatalf("notes_url empty")
	}
}
