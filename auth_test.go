package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginThrottle(t *testing.T) {
	h := basicAuth("vault", "right", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	try := func(addr, password string, header ...string) int {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = addr
		if password != "" {
			r.SetBasicAuth("vault", password)
		}
		for i := 0; i+1 < len(header); i += 2 {
			r.Header.Add(header[i], header[i+1])
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	// A request without a login is how a browser starts, not a guess.
	for i := 0; i < 3*loginTries; i++ {
		if code := try("203.0.113.1:1000", ""); code != 401 {
			t.Fatalf("no login: %d", code)
		}
	}
	if code := try("203.0.113.1:1000", "right"); code != 200 {
		t.Fatalf("right password after requests without login: %d", code)
	}
	for i := 0; i < loginTries; i++ {
		if code := try("203.0.113.1:1000", "wrong"); code != 401 {
			t.Fatalf("wrong password, try %d: %d", i, code)
		}
	}
	// Now even the right one is refused: otherwise guessing could go on, and
	// a 200 among the 429 would be the answer.
	if code := try("203.0.113.1:2000", "right"); code != 429 {
		t.Errorf("after %d wrong passwords: %d, want 429", loginTries, code)
	}
	if code := try("203.0.113.2:1000", "right"); code != 200 {
		t.Errorf("another address is locked out too: %d", code)
	}

	// Behind a proxy the address is the one the proxy saw, and only that one.
	for i := 0; i < loginTries; i++ {
		try("127.0.0.1:1000", "wrong", "X-Forwarded-For", "198.51.100.7")
	}
	if code := try("127.0.0.1:1000", "right", "X-Forwarded-For", "198.51.100.7"); code != 429 {
		t.Errorf("the guesser behind the proxy: %d, want 429", code)
	}
	if code := try("127.0.0.1:1000", "right", "X-Forwarded-For", "198.51.100.8"); code != 200 {
		t.Errorf("the owner behind the same proxy: %d, want 200", code)
	}
	// What a client claims is not believed: the proxy appends the real address.
	if code := try("127.0.0.1:1000", "right", "X-Forwarded-For", "10.9.9.9, 198.51.100.7"); code != 429 {
		t.Errorf("a made-up first entry gets around the block: %d", code)
	}
	if code := try("203.0.113.1:1000", "right", "X-Forwarded-For", "10.9.9.9"); code != 429 {
		t.Errorf("a header from a public address is believed: %d", code)
	}
}

func TestThrottleForgets(t *testing.T) {
	now := time.Now()
	limit := newThrottle()
	limit.now = func() time.Time { return now }
	for i := 0; i < loginTries; i++ {
		limit.failed("a")
	}
	if limit.blockedFor("a") == 0 {
		t.Fatal("not blocked")
	}
	now = now.Add(loginBlock + time.Second)
	if wait := limit.blockedFor("a"); wait != 0 {
		t.Errorf("still blocked for %v after the block", wait)
	}
	// failures spread thinly never add up
	for i := 0; i < 3*loginTries; i++ {
		limit.failed("slow")
		now = now.Add(loginWindow / 2)
	}
	if limit.blockedFor("slow") != 0 {
		t.Error("blocked, though never many failures within one window")
	}
	for i := 0; i < loginClients+5; i++ {
		limit.failed(time.Duration(i).String())
	}
	if len(limit.clients) > loginClients {
		t.Errorf("remembers %d addresses", len(limit.clients))
	}
}

func TestHeaders(t *testing.T) {
	page := secureHeaders(sandboxed(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	rec := httptest.NewRecorder()
	page.ServeHTTP(rec, httptest.NewRequest("GET", "/dav/notes/evil.html", nil))
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY",
		"Referrer-Policy": "same-origin", "Content-Security-Policy": "sandbox",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}
