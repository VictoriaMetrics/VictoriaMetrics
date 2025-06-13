# qtc

Template compiler (converter) for [quicktemplate](https://github.com/valyala/quicktemplate).
Converts quicktemplate files into Go code. By default these files
have `.qtpl` extension.

# Usage

```
$ go get -u github.com/valyala/quicktemplate/qtc
$ qtc -h
```

`qtc` may be called either directly or via [go generate](https://blog.golang.org/generate).
The latter case is preffered. Just put the following line near the `main` function:

```go
package main

//go:generate qtc -dir=path/to/directory/with/templates

func main() {
    // main code here
}
```

Then run `go generate` whenever you need re-generating template code.
Directory with templates may contain arbirary number of subdirectories -
`qtc` generates template code recursively for each subdirectory.

Directories with templates may also contain arbitrary `.go` files - contents
of these files may be used inside templates. Such Go files usually contain
various helper functions and structs.
