# tenv

<img title="Gopher" alt="tenv Gopher" src="./tenv.png" width="400">


[![test_and_lint](https://github.com/sivchari/tenv/actions/workflows/workflows.yml/badge.svg?branch=main)](https://github.com/sivchari/tenv/actions/workflows/workflows.yml)

tenv is analyzer that detects using os.Setenv instead of t.Setenv since Go1.17

## Instruction

```sh
go install github.com/sivchari/tenv/cmd/tenv@latest
```

## Usage

```go
package main

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(t *testing.T) {
	fmt.Println(os.Getenv("GO"))
	os.Setenv("GO", "HACKING GOPHER")
}

func TestMain2(t *testing.T) {
	fmt.Println(os.Getenv("GO"))
}

func helper() {
	os.Setenv("GO", "HACKING GOPHER")
}
```

```console
go vet -vettool=$(which tenv) ./...

# a
./main_test.go:11:2: os.Setenv() can be replaced by `t.Setenv()` in TestMain
```

### option

The option `all` will run against whole test files (`_test.go`) regardless of method/function signatures.  

By default, only methods that take `*testing.T`, `*testing.B`, and `testing.TB` as arguments are checked.

```go
package main

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(t *testing.T) {
	fmt.Println(os.Getenv("GO"))
	os.Setenv("GO", "HACKING GOPHER")
}

func TestMain2(t *testing.T) {
	fmt.Println(os.Getenv("GO"))
}

func helper() {
	os.Setenv("GO", "HACKING GOPHER")
}
```

```console
go vet -vettool=$(which tenv) -tenv.all ./...

# a
./main_test.go:11:2: os.Setenv() can be replaced by `t.Setenv()` in TestMain
./main_test.go:19:2: os.Setenv() can be replaced by `testing.Setenv()` in helper
```

## CI

### CircleCI

```yaml
- run:
    name: install tenv
    command: go install github.com/sivchari/tenv@latest

- run:
    name: run tenv
    command: go vet -vettool=`which tenv` ./...
```

### GitHub Actions

```yaml
- name: install tenv
  run: go install github.com/sivchari/tenv@latest

- name: run tenv
  run: go vet -vettool=`which tenv` ./...
```
