package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"markdown-webdav-backend/internal/index"
)

// browser keeps cookies between requests, as far as this test needs it:
// one cookie, replaced or deleted by Set-Cookie.
type browser struct {
	t          *testing.T
	h          http.Handler
	cookie     *http.Cookie
	cookieName string // which cookie it keeps; the search history's if unset
	headers    http.Header
}

func (b *browser) do(method, target string) (int, string, http.Header) {
	b.t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for k, v := range b.headers {
		req.Header[k] = v
	}
	if b.cookie != nil {
		req.AddCookie(b.cookie)
	}
	rec := httptest.NewRecorder()
	b.h.ServeHTTP(rec, req)
	res := rec.Result()
	name := b.cookieName
	if name == "" {
		name = historyCookie
	}
	for _, c := range res.Cookies() {
		if c.Name == name {
			if b.cookie = c; c.MaxAge < 0 {
				b.cookie = nil
			}
		}
	}
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(body), res.Header
}

func (b *browser) search(q string) string {
	b.t.Helper()
	_, body, _ := b.do("GET", "/v/-/search?q="+url.QueryEscape(q))
	return body
}

func (b *browser) remembered() string {
	if b.cookie == nil {
		return ""
	}
	values, _ := url.ParseQuery(b.cookie.Value)
	return strings.Join(values["q"], " | ")
}

func historyVault(t *testing.T) *browser {
	t.Helper()
	root := t.TempDir()
	for i := 0; i < 12; i++ {
		os.WriteFile(filepath.Join(root, fmt.Sprintf("Note %d.md", i)), []byte(fmt.Sprintf("word%d shared <b>&amp;\n", i)), 0o644)
	}
	idx := index.New(root, "/v")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	h, err := New(Config{Name: "v", Dir: root, Index: idx})
	if err != nil {
		t.Fatal(err)
	}
	return &browser{t: t, h: h}
}

func TestSearchHistory(t *testing.T) {
	b := historyVault(t)
	if body := b.search("word1"); strings.Contains(body, "Recent searches</h2>") == false || b.remembered() != "word1" {
		t.Fatalf("first search: remembered %q", b.remembered())
	}
	b.search("word2")
	b.search("shared")
	b.search("WORD1") // again, differently capitalised: moves to the front, once
	if got := b.remembered(); got != "WORD1 | shared | word2" {
		t.Errorf("order and duplicates: %q", got)
	}

	b.search("nothing matches this")      // found nothing
	b.search("   ")                       // empty
	long := strings.Repeat("shared ", 15) // finds the notes, but is too long to keep
	if body := b.search(long); len(strings.TrimSpace(long)) <= historyQueryMax || !strings.Contains(body, "12 results for") {
		t.Fatalf("the long query must match and exceed %d bytes", historyQueryMax)
	}
	if got := b.remembered(); got != "WORD1 | shared | word2" {
		t.Errorf("remembered what it should not: %q", got)
	}

	for i := 3; i < 12; i++ {
		b.search(fmt.Sprintf("word%d", i))
	}
	// twelve searches by now: the two oldest, word2 and shared, have dropped out
	if got := strings.Split(b.remembered(), " | "); len(got) != historyMax || got[0] != "word11" || got[historyMax-1] != "WORD1" {
		t.Errorf("limit of %d, newest first: %q", historyMax, got)
	}

	c := b.cookie
	if c.Path != "/v/" || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.MaxAge <= 0 || c.Secure {
		t.Errorf("cookie attributes: %+v", c)
	}
}

func TestSearchHistoryIsShown(t *testing.T) {
	b := historyVault(t)
	b.search(`shared <b>`) // finds the notes, and is hostile to HTML
	b.search("word1")

	// on the search page, as links, newest first
	page := b.search("")
	first, second := strings.Index(page, `<a href="/v/-/search?q=word1">word1</a>`), strings.Index(page, `<a href="/v/-/search?q=shared%20%3cb%3e">shared &lt;b&gt;</a>`)
	if first < 0 || second < first {
		t.Errorf("history links missing or out of order (%d, %d)\n%s", first, second, page)
	}
	if !strings.Contains(page, `<form method="post" action="/v/-/search/clear"><button type="submit">Clear history</button>`) {
		t.Error("no way to clear the history")
	}
	// on every other page, as a list under the search box, newest on top
	_, note, _ := b.do("GET", "/v/Note%201")
	header := note[:strings.Index(note, "</header>")]
	if !strings.Contains(header, `<ul class="search-history" aria-label="Recent searches">`+
		`<li><a href="/v/-/search?q=word1">word1</a></li>`+
		`<li><a href="/v/-/search?q=shared%20%3cb%3e">shared &lt;b&gt;</a></li></ul>`) {
		t.Errorf("search box offers no history:\n%s", header)
	}
	// the magnifier opens it: it is the field's label, and shows a chevron when there is a history
	if !strings.Contains(header, `<label for="search-q" title="Recent searches">`) || !strings.Contains(header, `id="search-q"`) || !strings.Contains(header, `class="more"`) {
		t.Errorf("no magnifier to open the history:\n%s", header)
	}
	if !strings.Contains(header, `autocomplete="off"`) {
		t.Error("the browser's own suggestions would cover the list")
	}
	if strings.Contains(page+note, "<b>") {
		t.Error("a remembered query reached the page unescaped")
	}
	// the results page offers the way back to an empty search
	if results := b.search("word1"); !strings.Contains(results, `<a href="/v/-/search">Clear search</a>`) {
		t.Error("no clear search link on the results")
	}
}

func TestClearSearchHistory(t *testing.T) {
	b := historyVault(t)
	b.search("word1")
	if code, _, _ := b.do("GET", "/v/-/search/clear"); code == http.StatusSeeOther || b.cookie == nil {
		t.Errorf("a GET cleared the history (status %d): a prefetching browser would too", code)
	}
	code, _, header := b.do("POST", "/v/-/search/clear")
	if code != http.StatusSeeOther || header.Get("Location") != "/v/-/search" || b.cookie != nil {
		t.Errorf("clear: status %d, location %q, cookie %v", code, header.Get("Location"), b.cookie)
	}
	_, note, _ := b.do("GET", "/v/Note%201")
	if page := b.search(""); strings.Contains(page, "Recent searches") || strings.Contains(note, "search-history") {
		t.Error("history still shown after clearing")
	}
	if !strings.Contains(note, `<label for="search-q" title="Search">`) || strings.Contains(note, `class="more"`) {
		t.Error("without a history the magnifier stays, without its chevron")
	}
}

func TestSearchHistoryCookieIsNotTrusted(t *testing.T) {
	b := historyVault(t)
	for _, value := range []string{"%zz", "q=" + strings.Repeat("y", 500), "other=1", strings.Repeat("q=a&", 50)} {
		b.cookie = &http.Cookie{Name: historyCookie, Value: value}
		code, page, _ := b.do("GET", "/v/-/search")
		header, rest, _ := strings.Cut(page, "</header>") // the list under the box, and the section below
		const item = `<li><a href="/v/-/search?q=`
		if code != 200 || strings.Contains(page, strings.Repeat("y", 200)) ||
			strings.Count(header, item) > historyMax || strings.Count(rest, item) > historyMax {
			t.Errorf("cookie %.20q: status %d, %d + %d entries", value, code, strings.Count(header, item), strings.Count(rest, item))
		}
	}
}

func TestSearchHistoryBehindTLSProxy(t *testing.T) {
	b := historyVault(t)
	b.headers = http.Header{"X-Forwarded-Proto": {"https"}}
	b.search("word1")
	if !b.cookie.Secure {
		t.Error("cookie not marked Secure although the browser talks HTTPS")
	}
}
