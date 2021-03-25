# Jump Consistent Hash

[![Build Status](https://github.com/lithammer/go-jump-consistent-hash/workflows/Go/badge.svg)](https://github.com/lithammer/go-jump-consistent-hash/actions)
[![Godoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/lithammer/go-jump-consistent-hash)

Go implementation of the jump consistent hash algorithm[1] by John Lamping and Eric Veach.

[1] http://arxiv.org/pdf/1406.2294v1.pdf

## Usage

```go
import jump "github.com/lithammer/go-jump-consistent-hash"

func main() {
    h := jump.Hash(256, 1024)  // h = 520
}
```

Includes a helper function for using a `string` as key instead of an `uint64`. This requires a hasher that computes the string into a format accepted by `Hash()`. Such a hasher that uses [CRC-64 (ECMA)](https://en.wikipedia.org/wiki/Cyclic_redundancy_check) is also included for convenience.

```go
h := jump.HashString("127.0.0.1", 8, jump.NewCRC64())  // h = 7
```

In reality though you probably want to use a `Hasher` so you won't have to repeat the bucket size and which key hasher used. It also uses more convenient types, like `int` instead of `int32`.

```go
hasher := jump.New(8, jump.NewCRC64())
h := hasher.Hash("127.0.0.1")  // h = 7
```

If you want to use your own algorithm, you must implement the `KeyHasher` interface, which is a subset of the `hash.Hash64` interface available in the standard library.

Here's an example of a custom `KeyHasher` that uses Google's [FarmHash](https://github.com/google/farmhash) algorithm (the successor of CityHash) to compute the final key.

```go
type FarmHash struct {
    buf bytes.Buffer
}

func (f *FarmHash) Write(p []byte) (n int, err error) {
    return f.buf.Write(p)
}

func (f *FarmHash) Reset() {
    f.buf.Reset()
}

func (f *FarmHash) Sum64() uint64 {
    // https://github.com/dgryski/go-farm
    return farm.Hash64(f.buf.Bytes())
}

hasher := jump.New(8, &FarmHash{})
h := hasher.Hash("127.0.0.1")  // h = 5
```

## License

MIT
