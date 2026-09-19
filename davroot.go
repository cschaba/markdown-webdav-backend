package main

import (
	"encoding/xml"
	"net/http"
	"os"
	"strings"
	"time"
)

// davRoot answers for "/dav/" itself, above the vaults, which is where people
// point a file manager first. It lists every vault as a folder and nothing
// else: a read-only collection that no request can change. Each vault's own
// WebDAV handler serves "/dav/<name>/" and everything below it.
func davRoot(vaults []vaultSpec) http.Handler {
	const allow = "OPTIONS, PROPFIND"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := strings.TrimSuffix(r.URL.Path, "/"); p != davPrefix {
			http.NotFound(w, r) // "/dav/nosuchvault/..."
			return
		}
		switch r.Method {
		case http.MethodOptions:
			w.Header().Set("DAV", "1")
			w.Header().Set("Allow", allow)
		case "PROPFIND":
			entries := []davEntry{{href: davPrefix + "/", name: "dav"}}
			// Depth 0 asks about the folder only; 1 and infinity (the
			// default) about what is in it too, which is one level here.
			if r.Header.Get("Depth") != "0" {
				for _, v := range vaults {
					e := davEntry{href: davPrefix + "/" + v.name + "/", name: v.name}
					// Without it, file managers show a date from the year 2000.
					if info, err := os.Stat(v.dir); err == nil {
						e.modified = info.ModTime()
					}
					entries = append(entries, e)
				}
			}
			writeMultistatus(w, entries)
		default:
			w.Header().Set("Allow", allow)
			http.Error(w, "the list of vaults is read-only; open a vault: "+davPrefix+"/<vault>/", http.StatusMethodNotAllowed)
		}
	})
}

type davEntry struct {
	href, name string
	modified   time.Time // zero: not sent
}

// writeMultistatus answers a PROPFIND with folders, in the form RFC 4918
// gives: every property is sent whatever was asked for, which clients accept.
func writeMultistatus(w http.ResponseWriter, entries []davEntry) {
	type prop struct {
		DisplayName  string `xml:"D:displayname"`
		ResourceType struct {
			Collection struct{} `xml:"D:collection"`
		} `xml:"D:resourcetype"`
		ContentType  string `xml:"D:getcontenttype"`
		LastModified string `xml:"D:getlastmodified,omitempty"`
	}
	type response struct {
		Href     string `xml:"D:href"`
		Propstat struct {
			Prop   prop   `xml:"D:prop"`
			Status string `xml:"D:status"`
		} `xml:"D:propstat"`
	}
	multistatus := struct {
		XMLName   xml.Name   `xml:"D:multistatus"`
		NS        string     `xml:"xmlns:D,attr"`
		Responses []response `xml:"D:response"`
	}{NS: "DAV:"}
	for _, e := range entries {
		var resp response
		resp.Href = e.href
		resp.Propstat.Prop.DisplayName = e.name
		resp.Propstat.Prop.ContentType = "httpd/unix-directory"
		if !e.modified.IsZero() {
			resp.Propstat.Prop.LastModified = e.modified.UTC().Format(http.TimeFormat)
		}
		resp.Propstat.Status = "HTTP/1.1 200 OK"
		multistatus.Responses = append(multistatus.Responses, resp)
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(multistatus)
}
