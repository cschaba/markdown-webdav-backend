package web

import (
	"net/http"
	"net/url"
	"strings"
)

// The search history lives in a cookie: the server stays without state, and
// without a database there is nowhere else to put it. It is therefore per
// browser — the phone does not know what the laptop searched for.
//
// The cookie holds the queries as a URL query string (q=a&q=b), newest first,
// because that encoding is already exactly what a cookie value may contain.
// It is scoped to the vault's path, so vaults keep separate histories, and it
// is HttpOnly: the pages need no script to show it.

const (
	historyCookie = "search-history"
	historyMax    = 10
	// Longer queries are not remembered. It bounds the cookie: browsers drop
	// cookies over about 4 KB, and escaping can triple a query's length.
	historyQueryMax = 100
)

func (h *Handler) history(r *http.Request) []string {
	cookie, err := r.Cookie(historyCookie)
	if err != nil {
		return nil
	}
	values, err := url.ParseQuery(cookie.Value)
	if err != nil {
		return nil // not ours, or damaged: start over
	}
	var out []string
	for _, q := range values["q"] {
		if q = strings.TrimSpace(q); q != "" && len(q) <= historyQueryMax && len(out) < historyMax {
			out = append(out, q)
		}
	}
	return out
}

// remember puts q at the front of the history and returns the new history.
func (h *Handler) remember(w http.ResponseWriter, r *http.Request, q string) []string {
	old := h.history(r)
	if len(q) > historyQueryMax {
		return old
	}
	queries := []string{q}
	for _, other := range old {
		if !strings.EqualFold(other, q) && len(queries) < historyMax {
			queries = append(queries, other)
		}
	}
	h.setHistory(w, r, url.Values{"q": queries}.Encode(), 365*24*60*60)
	return queries
}

func (h *Handler) setHistory(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     historyCookie,
		Value:    value,
		Path:     h.prefix + "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Behind a TLS proxy the request arrives as plain HTTP.
		Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
}

// clearHistory forgets the searches. It is a POST: a link would be followed
// by browsers that prefetch, and by anything that crawls the page.
func (h *Handler) clearHistory(w http.ResponseWriter, r *http.Request) {
	h.setHistory(w, r, "", -1)
	http.Redirect(w, r, h.prefix+"/-/search", http.StatusSeeOther)
}
