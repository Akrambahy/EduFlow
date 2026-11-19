package attendance

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const window = "2b2fa9fb-15b8-4d4e-9765-3049fbb8a3cf"

func TestTokenRoundTrip(t *testing.T) {
	secret, _ := NewSecret()
	ttl := 30 * time.Second
	now := time.Date(2026, 11, 20, 10, 0, 5, 0, time.UTC)
	tok, err := IssueToken(window, secret, ttl, now)
	if err != nil {
		t.Fatal(err)
	}
	if !tok.ExpiresAt.After(now) || tok.ExpiresAt.Sub(now) > ttl {
		t.Fatalf("bad expiry %v", tok.ExpiresAt)
	}
	id, err := ParseWindowID(tok.Value)
	if err != nil || id != window {
		t.Fatalf("window id %q %v", id, err)
	}
	if err := VerifyToken(tok.Value, secret, ttl, now); err != nil {
		t.Fatal(err)
	}
	if err := VerifyToken(tok.Value, secret, ttl, now.Add(ttl)); err != nil {
		t.Fatalf("previous step should be accepted for scan latency: %v", err)
	}
}

func TestTokenExpires(t *testing.T) {
	secret, _ := NewSecret()
	ttl := 30 * time.Second
	now := time.Date(2026, 11, 20, 10, 0, 5, 0, time.UTC)
	tok, _ := IssueToken(window, secret, ttl, now)
	if err := VerifyToken(tok.Value, secret, ttl, now.Add(2*ttl)); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("want expired, got %v", err)
	}
}

func TestTokenRejectsTamperingAndFuture(t *testing.T) {
	secret, _ := NewSecret()
	other, _ := NewSecret()
	ttl := 30 * time.Second
	now := time.Date(2026, 11, 20, 10, 0, 5, 0, time.UTC)
	tok, _ := IssueToken(window, secret, ttl, now)

	if err := VerifyToken(tok.Value, other, ttl, now); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("wrong secret: %v", err)
	}
	future, _ := IssueToken(window, secret, ttl, now.Add(5*ttl))
	if err := VerifyToken(future.Value, secret, ttl, now); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("future token: %v", err)
	}
	flipped := []byte(tok.Value)
	last := 30
	if flipped[last] == 'A' {
		flipped[last] = 'B'
	} else {
		flipped[last] = 'A'
	}
	if err := VerifyToken(string(flipped), secret, ttl, now); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("tampered: %v", err)
	}
	for _, bad := range []string{"", "abc", strings.Repeat("A", 54), "!!!"} {
		if err := VerifyToken(bad, secret, ttl, now); !errors.Is(err, ErrTokenInvalid) {
			t.Fatalf("garbage %q: %v", bad, err)
		}
	}
}

func TestLateDecision(t *testing.T) {
	start := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	grace := 10 * time.Minute
	cases := map[time.Time]string{
		start.Add(-5 * time.Minute):             StatusPresent,
		start.Add(10 * time.Minute):             StatusPresent,
		start.Add(10*time.Minute + time.Second): StatusLate,
	}
	for at, want := range cases {
		if got := checkInStatus(start, grace, at); got != want {
			t.Errorf("at %v got %s want %s", at, got, want)
		}
	}
}
