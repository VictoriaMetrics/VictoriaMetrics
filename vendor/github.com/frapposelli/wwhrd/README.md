# WWHRD? (What Would Henry Rollins Do?) [![Github Actions](https://github.com/frapposelli/wwhrd/workflows/ci/badge.svg)](https://github.com/frapposelli/wwhrd/actions?query=workflow%3Aci) [![codecov](https://codecov.io/gh/frapposelli/wwhrd/branch/master/graph/badge.svg)](https://codecov.io/gh/frapposelli/wwhrd)

![WWHRD?](./hack/wwhrd.svg)

Have Henry Rollins check vendored licenses in your Go project.

Please note that `wwhrd` **only checks** packages stored under `vendor/`, if you are using Go modules (`go mod`), you can add `go mod vendor` before running `wwhrd`, this will dump a copy of the vendored packages inside the local repo.

## Installation

```console
go get -u github.com/frapposelli/wwhrd
```

Using [Brew](https://brew.sh) on macOS:

```console
brew install frapposelli/tap/wwhrd
```

## Configuration file

Configuration for `wwhrd` is stored in `.wwhrd.yml` at the root of the repo you want to check.

The format is compatible with [Anderson](https://github.com/xoebus/anderson), just run `wwhrd check -f .anderson.yml`.

```yaml
---
denylist:
  - GPL-2.0

allowlist:
  - Apache-2.0
  - MIT

exceptions:
  - github.com/jessevdk/go-flags
  - github.com/pmezard/go-difflib/difflib
```

Having a license in the `blacklist` section will fail the check, unless the package is listed under `exceptions`.

`exceptions` can also be listed as wildcards:

```yaml
exceptions:
  - github.com/davecgh/go-spew/spew/...
```

Will make a blanket exception for all the packages under `github.com/davecgh/go-spew/spew`.

Use it in your CI!

```console
$ wwhrd check
INFO[0006] Found Approved license                        license=Apache-2.0 package="github.com/xanzy/ssh-agent"
INFO[0006] Found Approved license                        license=BSD-3-Clause package="golang.org/x/crypto/ed25519"
INFO[0006] Found Approved license                        license=Apache-2.0 package="gopkg.in/src-d/go-git.v4/internal/revision"
INFO[0006] Found Approved license                        license=Apache-2.0 package="gopkg.in/src-d/go-git.v4/plumbing/format/config"
INFO[0006] Found Approved license                        license=BSD-3-Clause package="golang.org/x/exp/rand"
INFO[0006] Found Approved license                        license=BSD-3-Clause package="gonum.org/v1/gonum/internal/cmplx64"
INFO[0006] Found Approved license                        license=Apache-2.0 package="gopkg.in/src-d/go-git.v4/plumbing/cache"
INFO[0006] Found Approved license                        license=MIT package="github.com/montanaflynn/stats"
INFO[0006] Found Approved license                        license=MIT package="github.com/ekzhu/minhash-lsh"
FATA[0006] Exiting: Non-Approved license found
$ echo $?
1
```

## Generate a dependency graph

Starting from version `v0.3.0`, `wwhrd graph` can be used to generate a graph in DOT language, the graph can then be parsed by Graphviz or other compatible tools.

To generate a PNG of the dependencies of your repository, you can run:

```console
$ wwhrd graph -o - | dot -Tpng > wwhrd-graph.png
```

The `-o -` option will print the DOT output to `STDOUT`.

## Usage

```console
$ wwhrd
Usage:
  wwhrd [OPTIONS] <check | graph | list>

What would Henry Rollins do?

Application Options:
  -v, --version  Show CLI version
  -q, --quiet    quiet mode, do not log accepted packages
  -d, --debug    verbose mode, log everything

Help Options:
  -h, --help     Show this help message

Available commands:
  check  Check licenses against config file (aliases: chk)
  graph  Generate dot graph dependency tree (aliases: dot)
  list   List licenses (aliases: ls)
```

## Acknowledgments

WWHRD? graphic by [Mitch Clem](http://mitchclem.tumblr.com/), used with permission, [support him!](https://store.silversprocket.net/collections/mitchclem).
