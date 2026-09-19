package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// A login is one password, and nothing else stands between the network and
// the notes, so guessing must be slow. An address that sent loginTries wrong
// passwords within loginWindow is refused for loginBlock, right password or
// not: refusing only the wrong ones would tell the two apart.
const (
	loginTries  = 10
	loginWindow = time.Minute
	loginBlock  = 5 * time.Minute
	// Addresses remembered at most. Beyond it the oldest records go, so that
	// a flood of addresses cannot grow the map without end.
	loginClients = 10000
)

type attempts struct {
	failed  int
	since   time.Time // start of the window the failures are counted in
	blocked time.Time // refused until then
}

type throttle struct {
	mu      sync.Mutex
	clients map[string]*attempts
	now     func() time.Time
}

func newThrottle() *throttle {
	return &throttle{clients: map[string]*attempts{}, now: time.Now}
}

// blockedFor reports how long client is still refused, 0 if it is not.
func (t *throttle) blockedFor(client string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if a := t.clients[client]; a != nil {
		return max(a.blocked.Sub(t.now()), 0)
	}
	return 0
}

func (t *throttle) failed(client string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	a := t.clients[client]
	if a == nil || now.Sub(a.since) > loginWindow {
		if len(t.clients) >= loginClients {
			t.forget(now)
		}
		a = &attempts{since: now}
		t.clients[client] = a
	}
	if a.failed++; a.failed >= loginTries {
		a.blocked = now.Add(loginBlock)
	}
}

func (t *throttle) succeeded(client string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.clients, client)
}

// forget drops what no longer matters, and everything if that is not enough.
func (t *throttle) forget(now time.Time) {
	for client, a := range t.clients {
		if now.After(a.blocked) && now.Sub(a.since) > loginWindow {
			delete(t.clients, client)
		}
	}
	if len(t.clients) >= loginClients {
		clear(t.clients)
	}
}

// clientAddr is the address a login attempt is counted against. Behind a
// reverse proxy every request comes from the proxy, and one guesser would lock
// out the owner; so when the peer is on this machine or a private network, the
// last entry of X-Forwarded-For counts. That one the proxy wrote itself - the
// entries before it are whatever the client sent, and are not believed.
func clientAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate()) {
		forwarded := r.Header.Values("X-Forwarded-For")
		if len(forwarded) > 0 {
			parts := strings.Split(forwarded[len(forwarded)-1], ",")
			if last := net.ParseIP(strings.TrimSpace(parts[len(parts)-1])); last != nil {
				return last.String()
			}
		}
	}
	return host
}

func basicAuth(user, password string, next http.Handler) http.Handler {
	// Hashing first makes the comparison constant-time regardless of length.
	wantUser, wantPass := sha256.Sum256([]byte(user)), sha256.Sum256([]byte(password))
	limit := newThrottle()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := clientAddr(r)
		if wait := limit.blockedFor(client); wait > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
			http.Error(w, "too many failed logins, try again later", http.StatusTooManyRequests)
			return
		}
		u, p, ok := r.BasicAuth()
		gotUser, gotPass := sha256.Sum256([]byte(u)), sha256.Sum256([]byte(p))
		userOK := subtle.ConstantTimeCompare(gotUser[:], wantUser[:]) == 1
		passOK := subtle.ConstantTimeCompare(gotPass[:], wantPass[:]) == 1
		if !ok || !userOK || !passOK {
			// A request without a login is how every browser begins; only a
			// wrong login is a guess.
			if ok {
				limit.failed(client)
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="vault", charset="UTF-8"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		limit.succeeded(client)
		next.ServeHTTP(w, r)
	})
}

// secureHeaders sets what every response carries. The pages add their own
// Content-Security-Policy (web.writePage), attachments theirs.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff") // the type is the one we say, never guessed from the content
		h.Set("X-Frame-Options", "DENY")           // no other site shows these pages in a frame, to have them clicked
		h.Set("Referrer-Policy", "same-origin")    // a link out of a note does not tell the site where it came from
		next.ServeHTTP(w, r)
	})
}

// sandboxed is for WebDAV, which also answers a browser's GET with the file as
// it is. An HTML or SVG file synced into the vault would run as a page of this
// origin, with the owner's login, and could rewrite the vault. The web view
// sandboxes such attachments; here everything is, since no sync client cares.
func sandboxed(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "sandbox")
		next.ServeHTTP(w, r)
	})
}
