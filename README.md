# gitup-go

A Go rewrite of [gitup-no-bookmark](https://github.com/AyakuraYuki/gitup-no-bookmark),
which itself is [earwig/git-repo-updater](https://github.com/earwig/git-repo-updater)
without bookmark support.

`gitup-go` easily updates multiple git repositories at once: it fetches remotes
and fast-forwards every branch that tracks a valid upstream, and knows how to
handle several remotes, dirty working directories, diverged local branches,
detached HEADs, and more.

Compared to the Python version, this rewrite ships as a single static binary —
no Python runtime, no PyInstaller packaging.

## Requirements

- `git` available on `PATH` (all repository operations shell out to the git CLI,
  so your credential helpers, SSH config, and `insteadOf` rules just work)

## Usage

```text
usage: git-updater [-t n] [-c] [-f] [-p] [-h] [-v] [path ...]

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

Both relative and absolute paths are accepted by all arguments.
```

Paths starting with `#` are treated as comments and printed as-is, so a list of
directories fed from a file can carry section headers.

When stdout is a terminal, fetches show live object counts in place, e.g.
`Fetching origin (123/456)`, during the compressing and receiving phases —
handy for big repositories or ones that have not been synced for a while.
When output is piped or redirected, progress is disabled automatically and the
output stays clean.

## Build

```shell
go build -o git-updater .
```

Cross-compile, e.g. for Windows:

```shell
GOOS=windows GOARCH=amd64 go build -o git-updater.exe .
```

## Test

```shell
go test ./...
```

The integration tests create throwaway repositories under a temp directory and
require a real `git` binary.

## License

MIT License, see [LICENSE](LICENSE).
