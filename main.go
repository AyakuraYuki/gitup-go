// Copyright (c) 2026 Ayakura Yuki
// Released under the terms of the MIT License. See LICENSE for details.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"

	"github.com/fatih/color"
	flag "github.com/spf13/pflag"

	"github.com/AyakuraYuki/gitup-go/internal/updater"
)

const binaryName = "git-updater"

const usage = `usage: git-updater [-t n] [-c] [-f] [-p] [-h] [-v] [path ...]

easily update multiple git repositories at once

updating repositories:
  path                update this repository, or all repositories it contains
                      if not a repo directly
  -t n, --depth n     max recursion depth when searching for repositories in
                      subdirectories, default is 3, use 0 for no recursion,
                      or -1 for unlimited
  -c, --current-only  only fetch the remote tracked by the current branch
                      instead of all remotes
  -f, --fetch-only    only fetch remotes, don't try to fast-forward any
                      branches
  -p, --prune         after fetching, delete remote-tracking branches that no
                      longer exist on their remote

miscellaneous:
  -h, --help          show this help message and exit
  -v, --version       show program's version number and exit

Both relative and absolute paths are accepted by all arguments.`

func main() {
	var opts updater.Options
	var showVersion, showHelp bool

	flags := flag.NewFlagSet(binaryName, flag.ContinueOnError)
	flags.Usage = func() { fmt.Println(usage) }
	flags.IntVarP(&opts.MaxDepth, "depth", "t", 3, "max recursion depth")
	flags.BoolVarP(&opts.CurrentOnly, "current-only", "c", false, "only fetch the remote tracked by the current branch")
	flags.BoolVarP(&opts.FetchOnly, "fetch-only", "f", false, "only fetch remotes")
	flags.BoolVarP(&opts.Prune, "prune", "p", false, "prune remote-tracking branches")
	flags.BoolVarP(&showHelp, "help", "h", false, "show help")
	flags.BoolVarP(&showVersion, "version", "v", false, "show version")

	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if showHelp {
		flags.Usage()
		return
	}
	if showVersion {
		fmt.Printf("%s %s (%s)\n", binaryName, version, runtime.Version())
		return
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		<-interrupt
		fmt.Println("\nstopped by user")
		os.Exit(130)
	}()

	bold := color.New(color.Bold).SprintFunc()
	fmt.Printf("%s: the git-repo-updater without bookmark\n\n", bold("["+binaryName+"]"))

	updater.UpdateDirectories(flags.Args(), opts)
}
