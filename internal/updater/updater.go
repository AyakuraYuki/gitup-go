// Copyright (c) 2026 Ayakura Yuki
// Released under the terms of the MIT License. See LICENSE for details.

package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Options holds the command-line switches that control how repositories are
// discovered and updated.
type Options struct {
	MaxDepth    int
	CurrentOnly bool
	FetchOnly   bool
	Prune       bool
}

// UpdateDirectories updates a list of directories supplied by command arguments.
func UpdateDirectories(paths []string, opts Options) {
	for _, path := range paths {
		dispatch(path, opts)
	}
}

// dispatch determines whether the path is a git repo on its own, a directory
// of git repositories, a shell glob pattern, or something invalid, then
// updates every repository it resolves to.
func dispatch(basePath string, opts Options) {
	base := expandUser(basePath)
	maxDepth := opts.MaxDepth
	if maxDepth >= 0 {
		maxDepth++
	}

	var valid []string
	if info, err := os.Stat(base); err == nil {
		switch {
		case isRepo(base):
			valid = []string{base}
		case info.IsDir() && opts.MaxDepth != 0:
			valid = collect([]string{base}, maxDepth)
		default:
			fmt.Printf("%s %s\n", errorLabel(), bold(base)+" is not a repository!!!")
			return
		}
	} else {
		if isComment(base) {
			if comment := getComment(base); comment != "" {
				fmt.Println(cyan(comment))
			}
			return
		}
		matches, _ := filepath.Glob(base)
		if len(matches) == 0 {
			fmt.Printf("%s %s\n", errorLabel(), bold(base)+" does not exist!!!")
			return
		}
		valid = collect(matches, maxDepth)
	}

	if abs, err := filepath.Abs(base); err == nil {
		base = abs
	}
	suffix := "s"
	if len(valid) == 1 {
		suffix = ""
	}
	fmt.Printf("%s (%d repo%s):\n", bold(base), len(valid), suffix)

	type entry struct{ name, path string }
	entries := make([]entry, 0, len(valid))
	for _, path := range valid {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		entries = append(entries, entry{getBasename(base, abs), abs})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].name != entries[j].name {
			return entries[i].name < entries[j].name
		}
		return entries[i].path < entries[j].path
	})
	for _, e := range entries {
		updateRepository(e.path, e.name, opts)
	}
}

// collect returns all valid repo paths in the given paths, recursively.
func collect(paths []string, maxDepth int) []string {
	if maxDepth == 0 {
		return nil
	}
	var valid []string
	for _, path := range paths {
		if isRepo(path) {
			valid = append(valid, path)
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		children, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		childPaths := make([]string, 0, len(children))
		for _, child := range children {
			childPaths = append(childPaths, filepath.Join(path, child.Name()))
		}
		valid = append(valid, collect(childPaths, maxDepth-1)...)
	}
	return valid
}

// isRepo reports whether the path itself is a git repository (a working tree
// with .git, or a bare repository).
func isRepo(path string) bool {
	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		return true
	}
	out, _, err := gitRun(path, "rev-parse", "--is-bare-repository", "--git-dir")
	if err != nil {
		return false
	}
	lines := strings.Split(out, "\n")
	return len(lines) >= 2 &&
		strings.TrimSpace(lines[0]) == "true" &&
		strings.TrimSpace(lines[1]) == "."
}

// getBasename returns a reasonable display name for a repo path in the given base.
func getBasename(base, path string) string {
	sep := string(os.PathSeparator)
	if strings.HasPrefix(path, base+sep) {
		return path[len(base+sep):]
	}
	prefix := commonPrefix(base, path)
	for !strings.HasPrefix(base, prefix+sep) {
		old := prefix
		prefix = filepath.Dir(prefix)
		if prefix == old {
			break
		}
	}
	parts := strings.SplitN(path, prefix+sep, 2)
	if len(parts) < 2 {
		return path
	}
	return parts[1]
}

func commonPrefix(a, b string) string {
	maxLen := min(len(b), len(a))
	i := 0
	for i < maxLen && a[i] == b[i] {
		i++
	}
	return a[:i]
}

func expandUser(path string) string {
	if path == "~" || strings.HasPrefix(path, "~"+string(os.PathSeparator)) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// isComment reports whether the line starts with a # symbol.
func isComment(path string) bool {
	return strings.HasPrefix(strings.TrimLeft(path, " \t"), "#")
}

// getComment returns the string minus the comment symbol.
func getComment(path string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.TrimLeft(path, " \t"), "#"))
}
