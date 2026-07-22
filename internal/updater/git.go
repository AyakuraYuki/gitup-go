// Copyright (c) 2026 Ayakura Yuki
// Released under the terms of the MIT License. See LICENSE for details.

package updater

import (
	"bytes"
	"errors"
	"os/exec"
	"regexp"
	"strings"
)

var spaceRe = regexp.MustCompile(`\s+`)

// gitRun executes git in the given directory and returns trimmed stdout and
// stderr. A non-nil error means git exited unsuccessfully (or failed to run).
func gitRun(dir string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String()), err
}

// gitOut is gitRun for callers that only care about stdout.
func gitOut(dir string, args ...string) (string, error) {
	stdout, _, err := gitRun(dir, args...)
	return stdout, err
}

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func collapseSpaces(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}
