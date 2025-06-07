readline
========

[![Godoc](https://godoc.org/github.com/ergochat/readline?status.svg)](https://godoc.org/github.com/ergochat/readline)

This is a pure Go implementation of functionality comparable to [GNU Readline](https://en.wikipedia.org/wiki/GNU_Readline), i.e. line editing and command history for simple TUI programs.

It is a fork of [chzyer/readline](https://github.com/chzyer/readline).

* Relative to the upstream repository, it is actively maintained and has numerous bug fixes
   - See our [changelog](docs/CHANGELOG.md) for details on fixes and improvements
   - See our [migration guide](docs/MIGRATING.md) for advice on how to migrate from upstream
* Relative to [x/term](https://pkg.go.dev/golang.org/x/term), it has more features (e.g. tab-completion)
* In use by multiple projects: [gopass](https://github.com/gopasspw/gopass), [fq](https://github.com/wader/fq), and [ircdog](https://github.com/ergochat/ircdog)


```go
package main

import (
	"fmt"
	"log"

	"github.com/ergochat/readline"
)

func main() {
	// see readline.NewFromConfig for advanced options:
	rl, err := readline.New("> ")
	if err != nil {
		log.Fatal(err)
	}
	defer rl.Close()
	log.SetOutput(rl.Stderr()) // redraw the prompt correctly after log output

	for {
		line, err := rl.ReadLine()
		// `err` is either nil, io.EOF, readline.ErrInterrupt, or an unexpected
		// condition in stdin:
		if err != nil {
			return
		}
		// `line` is returned without the terminating \n or CRLF:
		fmt.Fprintf(rl, "you wrote: %s\n", line)
	}
}
```
