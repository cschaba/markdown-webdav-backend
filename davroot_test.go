package main

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDavRoot(t *testing.T) {
	h := davRoot([]vaultSpec{{name: "notes", dir: t.TempDir()}, {name: "test", dir: "testdata/vault"}})
	do := func(method, path, depth string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		if depth != "" {
			r.Header.Set("Depth", depth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}
	hrefs := func(rec *httptest.ResponseRecorder) []string {
		t.Helper()
		if rec.Code != http.StatusMultiStatus {
			t.Fatalf("PROPFIND: %d", rec.Code)
		}
		// Parsed the way a client would, by namespace, not by prefix.
		var ms struct {
			Responses []struct {
				Href       string    `xml:"DAV: href"`
				Collection *struct{} `xml:"DAV: propstat>prop>resourcetype>collection"`
				Modified   string    `xml:"DAV: propstat>prop>getlastmodified"`
			} `xml:"DAV: response"`
		}
		if err := xml.Unmarshal(rec.Body.Bytes(), &ms); err != nil {
			t.Fatalf("%v\n%s", err, rec.Body)
		}
		var out []string
		for _, resp := range ms.Responses {
			if resp.Collection == nil {
				t.Errorf("%s is not a folder", resp.Href)
			}
			if _, err := http.ParseTime(resp.Modified); resp.Href != "/dav/" && err != nil {
				t.Errorf("%s: last modified %q: %v", resp.Href, resp.Modified, err)
			}
			out = append(out, resp.Href)
		}
		return out
	}
	for _, path := range []string{"/dav/", "/dav"} {
		if got := strings.Join(hrefs(do("PROPFIND", path, "1")), " "); got != "/dav/ /dav/notes/ /dav/test/" {
			t.Errorf("PROPFIND %s, depth 1: %s", path, got)
		}
	}
	if got := strings.Join(hrefs(do("PROPFIND", "/dav/", "")), " "); got != "/dav/ /dav/notes/ /dav/test/" {
		t.Errorf("PROPFIND without depth: %s", got)
	}
	if got := strings.Join(hrefs(do("PROPFIND", "/dav/", "0")), " "); got != "/dav/" {
		t.Errorf("PROPFIND depth 0: %s", got)
	}
	if rec := do("OPTIONS", "/dav/", ""); rec.Code != 200 || rec.Header().Get("DAV") == "" {
		t.Errorf("OPTIONS: %d, DAV %q", rec.Code, rec.Header().Get("DAV"))
	}
	for _, method := range []string{"PUT", "DELETE", "MKCOL", "MOVE", "LOCK", "PROPPATCH", "GET"} {
		if rec := do(method, "/dav/", ""); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /dav/: %d, want 405", method, rec.Code)
		}
	}
	if rec := do("PROPFIND", "/dav/nosuchvault/", "1"); rec.Code != http.StatusNotFound {
		t.Errorf("an unknown vault: %d", rec.Code)
	}
}
