package gitlog

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDebouncedCommit(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 150*time.Millisecond, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	count := func() int {
		out, err := c.git("rev-list", "--count", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		n, err := strconv.Atoi(out)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	base := count() // the .gitignore commit

	// A burst of writes, each inside the quiet period, is one commit.
	for i := 0; i < 3; i++ {
		os.WriteFile(filepath.Join(dir, "Note.md"), []byte(strings.Repeat("x", i+1)), 0o644)
		c.Touch()
		time.Sleep(50 * time.Millisecond)
	}
	if got := count(); got != base {
		t.Fatalf("committed during the burst: %d commits, want %d", got, base)
	}
	time.Sleep(500 * time.Millisecond)
	if got := count(); got != base+1 {
		t.Fatalf("%d commits after the burst, want %d", got, base+1)
	}
	if msg, _ := c.git("log", "-1", "--format=%s"); msg != "vault: Note.md" {
		t.Errorf("message = %q", msg)
	}
	if status, _ := c.git("status", "--porcelain"); status != "" {
		t.Errorf("tree not clean: %q", status)
	}

	// Nothing changed: no empty commit.
	if err := c.Commit(); err != nil || count() != base+1 {
		t.Errorf("clean commit: err=%v count=%d", err, count())
	}
}

func TestMaxWaitBeatsConstantEditing(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 200*time.Millisecond, 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := c.git("rev-parse", "HEAD")
	deadline := time.Now().Add(1200 * time.Millisecond)
	for i := 0; time.Now().Before(deadline); i++ {
		os.WriteFile(filepath.Join(dir, "Note.md"), []byte(strings.Repeat("x", i+1)), 0o644)
		c.Touch()
		time.Sleep(50 * time.Millisecond)
	}
	if after, _ := c.git("rev-parse", "HEAD"); after == before {
		t.Error("no commit although edits never paused for the quiet period")
	}
}

func TestIgnoresClientLitter(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "._Note.md"), []byte("x"), 0o644)
	if status, _ := c.git("status", "--porcelain"); status != "" {
		t.Errorf("litter is not ignored: %q", status)
	}
}
