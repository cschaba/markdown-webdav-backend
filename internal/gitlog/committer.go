// Package gitlog versions the vault by committing changes to a git repository
// inside it. It shells out to git: the binary is already the reference
// implementation, and the history stays usable with every normal git tool.
package gitlog

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Files that clients drop into the vault and that carry no content worth
// versioning. They still sync over WebDAV; they just stay out of the history.
const defaultIgnore = `# written once by markdown-webdav-backend; edit freely
.DS_Store
._*
.trash/
.obsidian/workspace*.json
`

type Committer struct {
	dir     string
	quiet   time.Duration // commit after this long without a change
	maxWait time.Duration // but never later than this after the first change

	mu      sync.Mutex
	timer   *time.Timer
	first   time.Time // first uncommitted change; zero when nothing is pending
	lastErr error

	run sync.Mutex // serialises git invocations
}

// New prepares the repository, creating it on first start.
func New(dir string, quiet, maxWait time.Duration) (*Committer, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	c := &Committer{dir: abs, quiet: quiet, maxWait: maxWait}
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git is required for versioning: %w", err)
	}
	if _, err := os.Stat(filepath.Join(abs, ".git")); os.IsNotExist(err) {
		if _, err := c.git("init", "--quiet"); err != nil {
			return nil, err
		}
	}
	// Commits fail without an identity, and a container has none.
	for key, fallback := range map[string]string{
		"user.name":  "markdown-webdav-backend",
		"user.email": "vault@localhost",
	} {
		if _, err := c.git("config", "--get", key); err != nil {
			if _, err := c.git("config", key, fallback); err != nil {
				return nil, err
			}
		}
	}
	ignore := filepath.Join(abs, ".gitignore")
	if _, err := os.Stat(ignore); os.IsNotExist(err) {
		if err := os.WriteFile(ignore, []byte(defaultIgnore), 0o644); err != nil {
			return nil, err
		}
	}
	// Pick up whatever changed while the server was not running.
	return c, c.Commit()
}

// Touch records that the vault changed. The commit happens once the vault
// has been quiet for a while, so one sync run becomes one commit.
func (c *Committer) Touch() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if c.first.IsZero() {
		c.first = now
	}
	delay := c.quiet
	if remaining := c.maxWait - now.Sub(c.first); remaining < delay {
		delay = max(remaining, 0)
	}
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(delay, func() {
		if err := c.Commit(); err != nil {
			slog.Error("git commit failed", "err", err)
		}
	})
}

// Commit commits all pending changes now. It is a no-op on a clean tree.
func (c *Committer) Commit() (err error) {
	c.mu.Lock()
	if c.timer != nil {
		c.timer.Stop()
	}
	c.first = time.Time{}
	c.mu.Unlock()

	c.run.Lock()
	defer c.run.Unlock()
	defer func() {
		c.mu.Lock()
		c.lastErr = err
		c.mu.Unlock()
	}()

	if _, err := c.git("add", "--all"); err != nil {
		return err
	}
	status, err := c.git("status", "--porcelain")
	if err != nil {
		return err
	}
	if status == "" {
		return nil
	}
	if _, err := c.git("commit", "--quiet", "-m", message(status)); err != nil {
		return err
	}
	slog.Info("committed vault changes", "files", strings.Count(status, "\n")+1)
	return nil
}

// LastError returns the error of the most recent commit attempt, so the web
// UI can show that versioning is broken instead of failing silently.
func (c *Committer) LastError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr
}

func message(status string) string {
	lines := strings.Split(status, "\n")
	if len(lines) == 1 {
		return "vault: " + strings.TrimSpace(lines[0][2:])
	}
	return fmt.Sprintf("vault: %d files changed\n\n%s", len(lines), status)
}

func (c *Committer) git(args ...string) (string, error) {
	// safe.directory: a mounted volume is often owned by another uid, and git
	// refuses to touch such a repository unless told to on the command line.
	cmd := exec.Command("git", append([]string{"-c", "safe.directory=" + c.dir}, args...)...)
	cmd.Dir = c.dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}
