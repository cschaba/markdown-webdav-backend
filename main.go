// markdown-webdav-backend serves directories of Markdown notes ("vaults") two
// ways: over WebDAV for syncing from Obsidian, and as a rendered read-only
// website. Every change is committed to a git repository inside the vault.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/webdav"

	"github.com/cschaba/markdown-webdav-backend/internal/gitlog"
	"github.com/cschaba/markdown-webdav-backend/internal/index"
	"github.com/cschaba/markdown-webdav-backend/internal/vault"
	"github.com/cschaba/markdown-webdav-backend/internal/web"
)

// davPrefix is where sync clients connect: "/dav/<vault>/". The web view of a
// vault lives at "/<vault>/", so "dav" and "-" cannot be vault names.
const davPrefix = "/dav"

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	var specs vaultSpecs
	flag.Var(&specs, "vault", "vault to serve, as [name=]directory[,nogit]; repeat for several (default ./vault)")
	listen := flag.String("listen", env("MDWEBDAV_LISTEN", ":8080"), "address to listen on")
	user := flag.String("user", env("MDWEBDAV_USER", "vault"), "login name")
	quiet := flag.Duration("commit-after", 30*time.Second, "commit once a vault was unchanged for this long")
	maxWait := flag.Duration("commit-max-wait", 5*time.Minute, "commit at the latest this long after the first change")
	noAuth := flag.Bool("no-auth", false, "disable the login; only for use behind something that authenticates")
	flag.Parse()
	if len(specs) == 0 {
		// Flags replace the environment rather than add to it.
		for _, spec := range strings.Split(env("MDWEBDAV_VAULT", "./vault"), ";") {
			if err := specs.Set(spec); err != nil {
				fatal("bad MDWEBDAV_VAULT", "err", err)
			}
		}
	}

	// The password has no flag on purpose: flags show up in process listings.
	password := os.Getenv("MDWEBDAV_PASSWORD")
	if file := os.Getenv("MDWEBDAV_PASSWORD_FILE"); file != "" {
		content, err := os.ReadFile(file)
		if err != nil {
			fatal("cannot read password file", "err", err)
		}
		password = strings.TrimRight(string(content), "\r\n")
	}
	if password != "" && len(password) < 12 {
		slog.Warn("the password is short; it is all that protects the vault, and it can be guessed over the network")
	}
	if password == "" && !*noAuth {
		fatal("set MDWEBDAV_PASSWORD or MDWEBDAV_PASSWORD_FILE, or pass -no-auth if another layer authenticates")
	}

	var links []web.Vault
	for _, spec := range specs {
		links = append(links, web.Vault{Name: spec.name, URL: "/" + spec.name + "/"})
	}
	mux := http.NewServeMux()
	var committers []*gitlog.Committer
	for _, spec := range specs {
		committer, err := mount(mux, spec, links, *quiet, *maxWait)
		if err != nil {
			fatal("cannot serve vault", "vault", spec.name, "err", err)
		}
		if committer != nil {
			committers = append(committers, committer)
		}
		slog.Info("vault", "name", spec.name, "dir", spec.dir, "versioned", spec.git,
			"web", "/"+spec.name+"/", "webdav", davPrefix+"/"+spec.name+"/")
	}
	assets, err := web.Assets()
	if err != nil {
		fatal("cannot build assets", "err", err)
	}
	mux.Handle("GET "+web.AssetsPath, assets)
	mux.Handle("GET /{$}", web.Home(links))

	var handler http.Handler = mux
	if !*noAuth {
		handler = basicAuth(*user, password, mux)
	}
	handler = secureHeaders(handler)

	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		slog.Info("serving", "listen", *listen)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			fatal("server failed", "err", err)
		}
	}()
	<-ctx.Done()

	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
	// Do not leave the last edits uncommitted until the next start.
	for _, committer := range committers {
		if err := committer.Commit(); err != nil {
			slog.Error("final commit failed", "err", err)
		}
	}
}

// mount wires one vault into mux: a WebDAV change updates the index and arms
// the committer; the web view reads through the index. The returned committer
// is nil for an unversioned vault.
func mount(mux *http.ServeMux, spec vaultSpec, links []web.Vault, quiet, maxWait time.Duration) (*gitlog.Committer, error) {
	if err := os.MkdirAll(spec.dir, 0o755); err != nil {
		return nil, err
	}
	var committer *gitlog.Committer
	if spec.git {
		var err error
		if committer, err = gitlog.New(spec.dir, quiet, maxWait); err != nil {
			return nil, err
		}
	}
	idx := index.New(spec.dir, "/"+spec.name)
	if err := idx.Rebuild(); err != nil {
		return nil, err
	}

	fs, err := vault.New(spec.dir, func(c vault.Change) {
		if c.FileWrite {
			idx.Update(c.Path)
		} else if err := idx.Rebuild(); err != nil {
			slog.Error("reindex failed", "vault", spec.name, "err", err)
		}
		if committer != nil {
			committer.Touch()
		}
	})
	if err != nil {
		return nil, err
	}
	dav := &webdav.Handler{
		Prefix:     davPrefix + "/" + spec.name,
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				slog.Warn("webdav", "method", r.Method, "path", r.URL.Path, "err", err)
			}
		},
	}
	mux.Handle(davPrefix+"/"+spec.name+"/", sandboxed(dav))
	mux.Handle(davPrefix+"/"+spec.name, sandboxed(dav))

	cfg := web.Config{Name: spec.name, Dir: spec.dir, Index: idx, Vaults: links}
	if committer != nil {
		cfg.Warning = committer.LastError
	}
	site, err := web.New(cfg)
	if err != nil {
		return nil, err
	}
	mux.Handle("/"+spec.name+"/", site)
	return committer, nil
}

func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
