# Jump Consistent Hash

[![Build Status](https://travis-ci.org/renstrom/go-jump-consistent-hash.svg?branch=master)](https://travis-ci.org/renstrom/go-jump-consistent-hash)
[![Godoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/renstrom/go-jump-consistent-hash)

Go implementation of the jump consistent hash algorithm[1] by John Lamping and Eric Veach.

[1] http://arxiv.org/pdf/1406.2294v1.pdf

## Usage

```go
import jump "github.com/renstrom/go-jump-consistent-hash"

func main() {
    jump.Hash(256, 1024)  // 520
}
```

## License

MIT
