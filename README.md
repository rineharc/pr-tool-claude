# prt

`prt` shows a GitHub pull request in the terminal: the tree of changed files next to a unified diff, driven from the keyboard. It is one binary for macOS, Linux and Windows.

This is the first milestone: it reads one pull request. Comments, reviews and the pull request list come later.

## Install

Build it from source with Go 1.26.7 or newer:

```sh
go install github.com/rineharc/pr-tool-claude/cmd/prt@latest
```

Or clone the repository and run `go build ./cmd/prt`.

Every pull request to this repository also builds binaries for all six targets (macOS, Linux and Windows, each on amd64 and arm64). They are attached to the "Snapshot build" job of the CI run as the `prt-snapshot` artifact.

## Run

```sh
prt 482                                      # a number, in the repository of the current directory
prt 482 --repo owner/repo                    # a number in another repository
prt owner/repo#482
prt https://github.com/owner/repo/pull/482
prt --demo                                   # a bundled pull request; needs no network and no token
```

`prt` needs a GitHub token. It uses `GH_TOKEN` or `GITHUB_TOKEN` when one is set, and otherwise the account the [GitHub CLI](https://cli.github.com) is signed in to (`gh auth login`).

## Keys

| Key | Action |
|---|---|
| `j` / `k`, `↓` / `↑` | Down / up |
| `g` / `G`, `home` / `end` | Top / bottom |
| `ctrl+d` / `ctrl+u` | Half a page down / up |
| `pgdn` / `pgup` | Page down / up |
| `[` / `]` | Previous / next file |
| `{` / `}` | Previous / next hunk |
| `tab` / `shift+tab` | Switch between the file tree and the diff |
| `enter` or `l` / `h` | In the tree: open the file or expand the folder / collapse the folder |
| `t` | Show / hide the file tree |
| `?` | Help |
| `q`, `ctrl+c` | Quit |

In a terminal narrower than 100 columns the tree and the diff take turns: `t` or `tab` switches between them.

## Develop

```sh
go test ./...
PRT_UPDATE_GOLDEN=1 go test ./internal/ui/...   # rewrite the screen snapshots after an intended change
```

## License

[MIT](LICENSE)
