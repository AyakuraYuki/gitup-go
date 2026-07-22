// Copyright (c) 2026 Ayakura Yuki
// Released under the terms of the MIT License. See LICENSE for details.

package updater

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/mattn/go-isatty"
)

func stdoutIsTerminal() bool {
	fd := os.Stdout.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// fetchLineRe matches the per-ref summary lines "git fetch" prints, e.g.
//
//   - [new branch]      feature    -> origin/feature
//   - [new tag]         v1.0       -> v1.0
//     abc1234..def5678  main       -> origin/main
var fetchLineRe = regexp.MustCompile(`^ (.) (\[[^\]]+\]|\S+)\s+(\S+)\s+->\s+(\S+)`)

// updateRepository updates a single git repository by fetching remotes and
// fast-forwarding branches, mirroring the behavior of the Python original.
func updateRepository(path, name string, opts Options) {
	fmt.Printf("%s %s\n", indent1, bold(name+":"))

	active, _ := gitOut(path, "symbolic-ref", "--quiet", "--short", "HEAD")

	var remotes []string
	if opts.CurrentOnly {
		if active == "" {
			fmt.Printf("%s %s --current-only does not make sense with a detached HEAD\n", indent2, errorLabel())
			return
		}
		remote, upstream := trackingBranch(path, active)
		if upstream == "" {
			fmt.Printf("%s %s no remote tracked by current branch\n", indent2, errorLabel())
			return
		}
		remotes = []string{remote}
	} else {
		if out, _ := gitOut(path, "remote"); out != "" {
			remotes = strings.Split(out, "\n")
		}
	}

	if len(remotes) == 0 {
		fmt.Printf("%s %s no remotes configured to fetch\n", indent2, errorLabel())
		return
	}
	fetchRemotes(path, remotes, opts.Prune)

	if !opts.FetchOnly {
		for _, branch := range localBranches(path) {
			updateBranch(path, branch, branch == active)
		}
	}
}

// trackingBranch resolves the configured upstream of a branch. It returns the
// remote name and the upstream's short name ("" if no upstream is configured).
func trackingBranch(path, branch string) (remote, upstreamShort string) {
	remote, errRemote := gitOut(path, "config", "branch."+branch+".remote")
	merge, errMerge := gitOut(path, "config", "branch."+branch+".merge")
	if errRemote != nil || errMerge != nil || remote == "" || merge == "" {
		return "", ""
	}
	mergeBranch := strings.TrimPrefix(merge, "refs/heads/")
	if remote == "." {
		return remote, mergeBranch
	}
	return remote, remote + "/" + mergeBranch
}

// fetchRemotes fetches a list of remotes, displaying progress and result info
// along the way. Live progress counts are only rendered when stdout is a
// terminal, so redirected output stays clean.
func fetchRemotes(path string, remotes []string, prune bool) {
	showProgress := stdoutIsTerminal()
	for _, remote := range remotes {
		fmt.Printf("%s Fetching %s", indent2, bold(remote))

		if _, err := gitOut(path, "config", "--get-all", "remote."+remote+".fetch"); err != nil {
			fmt.Printf(": %s no configured refspec\n", yellow("skipping"))
			continue
		}

		args := []string{"fetch"}
		if showProgress {
			args = append(args, "--progress")
		}
		if prune {
			args = append(args, "--prune")
		}
		args = append(args, remote)
		printer := &progressPrinter{w: os.Stdout, enabled: showProgress}
		stderr, err := runFetch(path, args, printer)
		if err != nil {
			msg := strings.TrimPrefix(collapseSpaces(stderr), "fatal: ")
			if msg == "" {
				msg = fmt.Sprintf("git %s failed with status %d.", strings.Join(args, " "), exitCode(err))
			} else if !strings.HasSuffix(msg, ".") {
				msg += "."
			}
			fmt.Printf(": %s %s\n", red("error:"), msg)
			return
		}

		if summary := parseFetchOutput(stderr, remote); summary != "" {
			fmt.Printf(": %s.\n", summary)
		} else {
			fmt.Printf(": %s.\n", blue("up to date"))
		}
	}
}

// parseFetchOutput turns the ref summary lines of "git fetch" into a short
// human-readable report like "new branch (feature), branch updates (main, dev)".
// An empty result means everything was already up to date.
func parseFetchOutput(output, remote string) string {
	var newBranches, newTags, updates []string
	for _, line := range strings.Split(output, "\n") {
		m := fetchLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		flag, summary, to := m[1], m[2], m[4]
		name := strings.TrimPrefix(to, remote+"/")
		switch {
		case flag == "*" && summary == "[new branch]":
			newBranches = append(newBranches, name)
		case flag == "*" && summary == "[new tag]":
			newTags = append(newTags, name)
		case flag == " " && strings.Contains(summary, ".."):
			updates = append(updates, name)
		}
	}

	var parts []string
	add := func(names []string, singular, plural string) {
		if len(names) == 0 {
			return
		}
		desc := singular
		if len(names) > 1 {
			desc = plural
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", green(desc), strings.Join(names, ", ")))
	}
	add(newBranches, "new branch", "new branches")
	add(newTags, "new tag", "new tags")
	add(updates, "branch update", "branch updates")
	return strings.Join(parts, ", ")
}

func localBranches(path string) []string {
	out, err := gitOut(path, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil || out == "" {
		return nil
	}
	branches := strings.Split(out, "\n")
	sort.Strings(branches)
	return branches
}

// updateBranch fast-forwards a single branch to its upstream if possible.
func updateBranch(path, branch string, isActive bool) {
	fmt.Printf("%s Updating %s: ", indent2, bold(branch))

	remote, upstreamShort := trackingBranch(path, branch)
	if upstreamShort == "" {
		fmt.Printf("%s no upstream is tracked.\n", yellow("skipped:"))
		return
	}
	upstreamRef := "refs/remotes/" + upstreamShort
	if remote == "." {
		upstreamRef = "refs/heads/" + upstreamShort
	}

	branchSHA, err := gitOut(path, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		fmt.Printf("%s branch has no revisions.\n", yellow("skipped:"))
		return
	}
	upstreamSHA, err := gitOut(path, "rev-parse", "--verify", "--quiet", upstreamRef)
	if err != nil {
		fmt.Printf("%s upstream does not exist.\n", yellow("skipped:"))
		return
	}

	mergeBase, err := gitOut(path, "merge-base", branchSHA, upstreamSHA)
	if err != nil {
		fmt.Printf("%s cannot find merge base with upstream.\n", yellow("skipped:"))
		return
	}

	if mergeBase == upstreamSHA {
		fmt.Printf("%s.\n", blue("up to date"))
		return
	}

	if isActive {
		_, stderr, err := gitRun(path, "merge", "--ff-only", upstreamShort)
		switch {
		case err == nil:
			fmt.Printf("%s.\n", green("done"))
		case strings.Contains(stderr, "local changes") && strings.Contains(stderr, "would be overwritten"):
			fmt.Printf("%s uncommitted changes.\n", yellow("skipped:"))
		default:
			fmt.Printf("%s not possible to fast-forward.\n", yellow("skipped:"))
		}
		return
	}

	if _, _, err := gitRun(path, "merge-base", "--is-ancestor", branchSHA, upstreamSHA); err != nil {
		fmt.Printf("%s not possible to fast-forward.\n", yellow("skipped:"))
		return
	}
	if _, _, err := gitRun(path, "branch", "--force", branch, upstreamShort); err != nil {
		fmt.Printf("%s not possible to fast-forward.\n", yellow("skipped:"))
		return
	}
	fmt.Printf("%s.\n", green("done"))
}
