package api

import (
	"encoding/json"
	"testing"
)

func TestJSONStringUnmarshal(t *testing.T) {
	var s JSONString
	if err := json.Unmarshal([]byte(`"9443"`), &s); err != nil || s.String() != "9443" {
		t.Fatalf("string: err=%v s=%q", err, s)
	}
	if err := json.Unmarshal([]byte(`9443`), &s); err != nil || s.String() != "9443" {
		t.Fatalf("number: err=%v s=%q", err, s)
	}
}
