// Copyright (c) 2026 Ayakura Yuki
// Released under the terms of the MIT License. See LICENSE for details.

package updater

import (
	"bufio"
	"bytes"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

// progressRe matches the two progress phases worth displaying, mirroring the
// COMPRESSING | RECEIVING filter of the Python original, e.g.
//
//	Compressing objects:  45% (56/123)
//	Receiving objects:  45% (123/456), 1.2 MiB | 3.4 MiB/s
var progressRe = regexp.MustCompile(`^(Compressing|Receiving) objects:\s+\d+% \((\d+)/(\d+)\)`)

// progressNoisePrefixes are progress phases git reports that we neither
// display nor keep for summary/error parsing.
var progressNoisePrefixes = []string{
	"Enumerating objects",
	"Counting objects",
	"Compressing objects",
	"Receiving objects",
	"Resolving deltas",
	"Unpacking objects",
	"Total ",
}

// progressPrinter renders in-place object counts on the current output line
// while a fetch is running, e.g. "Fetching origin (56/123, 123/456)".
type progressPrinter struct {
	w       io.Writer
	enabled bool
	started bool
	phase   string
	lastLen int
}

func (p *progressPrinter) update(phase, cur, total string) {
	if !p.enabled {
		return
	}
	if phase != p.phase {
		if p.started {
			_, _ = io.WriteString(p.w, ", ")
		} else {
			_, _ = io.WriteString(p.w, " (")
			p.started = true
		}
		p.phase = phase
		p.lastLen = 0
	}
	text := cur + "/" + total
	_, _ = io.WriteString(p.w, strings.Repeat("\b", p.lastLen)+text)
	// blank out leftovers in case the new text is shorter than the previous one
	if pad := p.lastLen - len(text); pad > 0 {
		_, _ = io.WriteString(p.w, strings.Repeat(" ", pad)+strings.Repeat("\b", pad))
	}
	p.lastLen = len(text)
}

func (p *progressPrinter) finish() {
	if p.started {
		_, _ = io.WriteString(p.w, ")")
		p.started = false
	}
}

func parseProgressLine(line string) (phase, cur, total string, ok bool) {
	m := progressRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", "", false
	}
	return m[1], m[2], m[3], true
}

func isProgressNoise(line string) bool {
	for _, prefix := range progressNoisePrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// scanCRLFTokens splits on both \n and \r, because git redraws progress lines
// in place with bare carriage returns.
func scanCRLFTokens(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// consumeFetchStderr streams the stderr of a running fetch, feeding progress
// updates to the printer and returning every other non-empty line for
// summary/error parsing.
func consumeFetchStderr(r io.Reader, printer *progressPrinter) []string {
	var kept []string
	scanner := bufio.NewScanner(r)
	scanner.Split(scanCRLFTokens)
	for scanner.Scan() {
		line := scanner.Text()
		stripped := strings.TrimPrefix(line, "remote: ")
		if phase, cur, total, ok := parseProgressLine(stripped); ok {
			printer.update(phase, cur, total)
			continue
		}
		if isProgressNoise(stripped) || strings.TrimSpace(line) == "" {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

// runFetch executes git fetch in dir, rendering live progress to the printer's
// writer as the transfer goes. It returns the non-progress stderr output.
func runFetch(dir string, args []string, printer *progressPrinter) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	kept := consumeFetchStderr(stderrPipe, printer)
	printer.finish()
	return strings.Join(kept, "\n"), cmd.Wait()
}
