package certs

import (
	"errors"
	"strings"
	"testing"
)

func TestClassifyACMEErrorDNS(t *testing.T) {
	err := errors.New(`acme: error: 400 :: urn:ietf:params:acme:error:dns :: no valid A records found for vpn.example.com`)
	info := ClassifyACMEError(err)
	if !info.IsDNS || !info.PauseAutoRenew {
		t.Fatalf("expected dns pause, got %+v", info)
	}
}

func TestClassifyACMEErrorOther(t *testing.T) {
	err := errors.New("connection refused")
	info := ClassifyACMEError(err)
	if info.PauseAutoRenew {
		t.Fatalf("expected no pause for generic error")
	}
}

func TestClassifyACMEErrorRateLimited(t *testing.T) {
	err := errors.New(`acme: error: 429 :: POST :: https://acme-v02.api.letsencrypt.org/acme/new-order :: urn:ietf:params:acme:error:rateLimited :: too many certificates (5) already issued for this exact set of identifiers in the last 168h0m0s, retry after 2026-05-28 16:37:17 UTC: see https://letsencrypt.org/docs/rate-limits/`)
	info := ClassifyACMEError(err)
	if info.PauseAutoRenew || info.IsDNS {
		t.Fatalf("rate limit should not pause auto-renew, got %+v", info)
	}
	if !strings.Contains(info.Summary, "限速") || !strings.Contains(info.Summary, "168") {
		t.Fatalf("expected Chinese rate-limit summary, got %q", info.Summary)
	}
	if !strings.Contains(info.Summary, "2026-05-28") {
		t.Fatalf("expected retry time in summary, got %q", info.Summary)
	}
}

func TestClassifyACMEErrorLongUnauthorized(t *testing.T) {
	err := errors.New(`acme: error: 403 :: POST :: https://acme-v02.api.letsencrypt.org/acme/chall/challenge-id :: urn:ietf:params:acme:error:unauthorized :: 1.2.3.4: Invalid response from http://1.2.3.4/.well-known/acme-challenge/TOKEN: 404` + strings.Repeat("x", 300))
	info := ClassifyACMEError(err)
	if len(info.Summary) > 400 {
		t.Fatalf("summary still too long: %d", len(info.Summary))
	}
	if !strings.Contains(info.Summary, "授权") && !strings.Contains(info.Summary, "unauthorized") {
		t.Fatalf("expected abbreviated unauthorized hint, got %q", info.Summary)
	}
}
