// Copyright (c) 2026 Ayakura Yuki
// Released under the terms of the MIT License. See LICENSE for details.

package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestMain(m *testing.M) {
	color.NoColor = true
	os.Exit(m.Run())
}

func TestIsComment(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"# a comment", true},
		{"   # indented", true},
		{"/some/path", false},
		{"path # not leading", false},
	}
	for _, c := range cases {
		if got := isComment(c.in); got != c.want {
			t.Errorf("isComment(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestGetComment(t *testing.T) {
	if got := getComment("  ## hello world "); got != "hello world" {
		t.Errorf("getComment = %q, want %q", got, "hello world")
	}
}

func TestGetBasename(t *testing.T) {
	sep := string(os.PathSeparator)
	base := sep + filepath.Join("home", "user", "repos")
	cases := []struct {
		path string
		want string
	}{
		{filepath.Join(base, "project"), "project"},
		{filepath.Join(base, "group", "project"), filepath.Join("group", "project")},
		{base, "repos"},
	}
	for _, c := range cases {
		if got := getBasename(base, c.path); got != c.want {
			t.Errorf("getBasename(%q, %q) = %q, want %q", base, c.path, got, c.want)
		}
	}
}

func TestParseFetchOutput(t *testing.T) {
	output := `From github.com:user/repo
 * [new branch]      feature    -> origin/feature
 * [new tag]         v1.0       -> v1.0
   abc1234..def5678  main       -> origin/main
   abc1234..def5678  dev        -> origin/dev
 + abc1234...def5678 force      -> origin/force  (forced update)
 - [deleted]         (none)     -> origin/gone`
	got := parseFetchOutput(output, "origin")
	want := "new branch (feature), new tag (v1.0), branch updates (main, dev)"
	if got != want {
		t.Errorf("parseFetchOutput = %q, want %q", got, want)
	}
}

func TestParseFetchOutputUpToDate(t *testing.T) {
	if got := parseFetchOutput("", "origin"); got != "" {
		t.Errorf("parseFetchOutput(empty) = %q, want empty", got)
	}
}

func TestParseProgressLine(t *testing.T) {
	cases := []struct {
		in        string
		phase     string
		cur, tot  string
		wantMatch bool
	}{
		{"Receiving objects:  45% (123/456), 1.2 MiB | 3.4 MiB/s", "Receiving", "123", "456", true},
		{"Receiving objects: 100% (456/456), 5.6 MiB | 2.1 MiB/s, done.", "Receiving", "456", "456", true},
		{"Compressing objects:  45% (56/123)", "Compressing", "56", "123", true},
		{"Enumerating objects: 123, done.", "", "", "", false},
		{"Resolving deltas: 100% (78/78), done.", "", "", "", false},
		{"   abc1234..def5678  main       -> origin/main", "", "", "", false},
	}
	for _, c := range cases {
		phase, cur, tot, ok := parseProgressLine(c.in)
		if ok != c.wantMatch || phase != c.phase || cur != c.cur || tot != c.tot {
			t.Errorf("parseProgressLine(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
				c.in, phase, cur, tot, ok, c.phase, c.cur, c.tot, c.wantMatch)
		}
	}
}

func TestProgressPrinterRendering(t *testing.T) {
	var buf strings.Builder
	p := &progressPrinter{w: &buf, enabled: true}
	p.update("Compressing", "5", "123")
	p.update("Compressing", "56", "123")
	p.update("Receiving", "123", "456")
	p.finish()

	want := " (5/123" + "\b\b\b\b\b" + "56/123" + ", " + "123/456" + ")"
	if got := buf.String(); got != want {
		t.Errorf("progressPrinter output = %q, want %q", got, want)
	}
}

func TestProgressPrinterDisabled(t *testing.T) {
	var buf strings.Builder
	p := &progressPrinter{w: &buf, enabled: false}
	p.update("Receiving", "1", "2")
	p.finish()
	if buf.String() != "" {
		t.Errorf("disabled printer wrote %q, want nothing", buf.String())
	}
}

func TestConsumeFetchStderr(t *testing.T) {
	// simulate a real fetch stream: progress lines are separated by \r as git
	// redraws them in place, everything else by \n
	stream := "remote: Enumerating objects: 5, done.\n" +
		"remote: Compressing objects:  50% (1/2)\r" +
		"remote: Compressing objects: 100% (2/2), done.\n" +
		"Receiving objects:  40% (2/5)\r" +
		"Receiving objects: 100% (5/5), 1.2 KiB | 1.2 MiB/s, done.\n" +
		"Resolving deltas: 100% (1/1), done.\n" +
		"From /tmp/origin\n" +
		"   abc1234..def5678  main       -> origin/main\n"

	var buf strings.Builder
	p := &progressPrinter{w: &buf, enabled: true}
	kept := consumeFetchStderr(strings.NewReader(stream), p)
	p.finish()

	wantKept := []string{
		"From /tmp/origin",
		"   abc1234..def5678  main       -> origin/main",
	}
	if len(kept) != len(wantKept) || kept[0] != wantKept[0] || kept[1] != wantKept[1] {
		t.Errorf("kept lines = %q, want %q", kept, wantKept)
	}

	if got := parseFetchOutput(strings.Join(kept, "\n"), "origin"); got != "branch update (main)" {
		t.Errorf("parseFetchOutput on kept lines = %q, want %q", got, "branch update (main)")
	}

	want := " (1/2" + "\b\b\b" + "2/2" + ", " + "2/5" + "\b\b\b" + "5/5" + ")"
	if got := buf.String(); got != want {
		t.Errorf("progress rendering = %q, want %q", got, want)
	}
}

// --- integration tests below need a real git binary ---

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{
		"-C", dir,
		"-c", "user.name=test", "-c", "user.email=test@example.com",
		"-c", "commit.gpgsign=false", "-c", "protocol.file.allow=always",
	}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupRepos creates an origin repository with one commit and a clone of it,
// then adds one more commit to origin so that the clone is behind.
func setupRepos(t *testing.T) (origin, clone string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	tmp := t.TempDir()
	origin = filepath.Join(tmp, "origin")
	clone = filepath.Join(tmp, "clone")

	mustGit(t, tmp, "init", "-q", "-b", "main", origin)
	writeFile(t, filepath.Join(origin, "a.txt"), "one\n")
	mustGit(t, origin, "add", ".")
	mustGit(t, origin, "commit", "-q", "-m", "c1")

	mustGit(t, tmp, "clone", "-q", origin, clone)

	writeFile(t, filepath.Join(origin, "b.txt"), "two\n")
	mustGit(t, origin, "add", ".")
	mustGit(t, origin, "commit", "-q", "-m", "c2")
	return origin, clone
}

func headSHA(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := gitOut(dir, "rev-parse", ref)
	if err != nil {
		t.Fatalf("rev-parse %s in %s: %v", ref, dir, err)
	}
	return out
}

func TestUpdateFastForwardsActiveBranch(t *testing.T) {
	origin, clone := setupRepos(t)

	UpdateDirectories([]string{clone}, Options{MaxDepth: 3})

	if got, want := headSHA(t, clone, "main"), headSHA(t, origin, "main"); got != want {
		t.Errorf("clone main = %s, want %s", got, want)
	}
}

func TestFetchOnlyDoesNotMoveBranch(t *testing.T) {
	origin, clone := setupRepos(t)
	before := headSHA(t, clone, "main")

	UpdateDirectories([]string{clone}, Options{MaxDepth: 3, FetchOnly: true})

	if got := headSHA(t, clone, "main"); got != before {
		t.Errorf("clone main moved to %s, want %s", got, before)
	}
	// but the remote-tracking ref must have been fetched
	if got, want := headSHA(t, clone, "origin/main"), headSHA(t, origin, "main"); got != want {
		t.Errorf("clone origin/main = %s, want %s", got, want)
	}
}

func TestUpdateFastForwardsNonActiveBranch(t *testing.T) {
	origin, clone := setupRepos(t)
	// park HEAD on another branch so main is not active
	mustGit(t, clone, "checkout", "-q", "-b", "parked")

	UpdateDirectories([]string{clone}, Options{MaxDepth: 3})

	if got, want := headSHA(t, clone, "main"), headSHA(t, origin, "main"); got != want {
		t.Errorf("clone main = %s, want %s", got, want)
	}
}

func TestUpdateScansDirectoryOfRepos(t *testing.T) {
	origin, clone := setupRepos(t)
	parent := filepath.Dir(clone)

	UpdateDirectories([]string{parent}, Options{MaxDepth: 3})

	if got, want := headSHA(t, clone, "main"), headSHA(t, origin, "main"); got != want {
		t.Errorf("clone main = %s, want %s", got, want)
	}
}

func TestDirtyWorktreeIsSkipped(t *testing.T) {
	origin, clone := setupRepos(t)
	before := headSHA(t, clone, "main")
	// conflicting uncommitted change: origin's c2 adds b.txt, so a local
	// untracked b.txt with different content blocks the merge
	writeFile(t, filepath.Join(clone, "b.txt"), "local\n")

	UpdateDirectories([]string{clone}, Options{MaxDepth: 3})

	if got := headSHA(t, clone, "main"); got != before {
		t.Errorf("clone main moved to %s despite dirty worktree", got)
	}
	_ = origin
}
