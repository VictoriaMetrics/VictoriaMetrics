# noctx

![](https://github.com/sonatard/noctx/workflows/CI/badge.svg)

`noctx` finds sending http request without context.Context.

You should use `noctx` if sending http request in your library.
Passing `context.Context` enables library user to cancel http request, getting trace information and so on.

## Usage


### noctx with go vet

go vet is a Go standard tool for analyzing source code.

1. Install noctx.
```sh
$ go install github.com/sonatard/noctx/cmd/noctx@latest
```

2. noctx execute
```sh
$ go vet -vettool=`which noctx` main.go
./main.go:6:11: net/http.Get must not be called
```

### noctx with golangci-lint

golangci-lint is a fast Go linters runner.

1. Install golangci-lint.
[golangci-lint - Install](https://golangci-lint.run/usage/install/)

2. Setup .golangci.yml
```yaml:
# Add noctx to enable linters.
linters:
  enable:
    - noctx

# Or enable-all is true.
linters:
  enable-all: true
  disable:
   - xxx # Add unused linter to disable linters.
```

3. noctx execute
```sh
# Use .golangci.yml
$ golangci-lint run

# Only noctx execute
golangci-lint run --enable-only noctx
```

## Detection rules

- Executing following functions
  - `net/http.Get`
  - `net/http.Head`
  - `net/http.Post`
  - `net/http.PostForm`
  - `(*net/http.Client).Get`
  - `(*net/http.Client).Head`
  - `(*net/http.Client).Post`
  - `(*net/http.Client).PostForm`
- `http.Request` returned by `http.NewRequest` function and passes it to other function.

## How to fix

- Send http request using `(*http.Client).Do(*http.Request)` method.
- In Go 1.13 and later, use `http.NewRequestWithContext` function instead of using `http.NewRequest` function.
- In Go 1.12 and earlier, call `(http.Request).WithContext(ctx)` after `http.NewRequest`.

`(http.Request).WithContext(ctx)` has a disadvantage of performance because it returns a copy of `http.Request`. Use `http.NewRequestWithContext` function if you only support Go1.13 or later.


If your library already provides functions that don't accept context, you define a new function that accepts context and make the existing function a wrapper for a new function.


```go
// Before fix code
// Sending an HTTP request but not accepting context
func Send(body io.Reader)  error {
    req,err := http.NewRequest(http.MethodPost, "http://example.com", body)
    if err != nil {
        return err
    }
    _, err = http.DefaultClient.Do(req)
    if err != nil{
        return err
    }

    return nil
}
```

```go
// After fix code
func Send(body io.Reader) error {
    // Pass context.Background() to SendWithContext
    return SendWithContext(context.Background(), body)
}

// Sending an HTTP request and accepting context
func SendWithContext(ctx context.Context, body io.Reader) error {
    // Change NewRequest to NewRequestWithContext and pass context it
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://example.com", body)
    if err != nil {
        return err
    }
    _, err = http.DefaultClient.Do(req)
    if err != nil {
        return err
    }

    return nil
}
```

## Detection sample

```go
package main

import (
	"context"
	"net/http"
)

func main() {
	const url = "http://example.com"
	http.Get(url) // want `net/http\.Get must not be called`
	http.Head(url)          // want `net/http\.Head must not be called`
	http.Post(url, "", nil) // want `net/http\.Post must not be called`
	http.PostForm(url, nil) // want `net/http\.PostForm must not be called`

	cli := &http.Client{}
	cli.Get(url) // want `\(\*net/http\.Client\)\.Get must not be called`
	cli.Head(url)          // want `\(\*net/http\.Client\)\.Head must not be called`
	cli.Post(url, "", nil) // want `\(\*net/http\.Client\)\.Post must not be called`
	cli.PostForm(url, nil) // want `\(\*net/http\.Client\)\.PostForm must not be called`

	req, _ := http.NewRequest(http.MethodPost, url, nil) // want `should rewrite http.NewRequestWithContext or add \(\*Request\).WithContext`
	cli.Do(req)

	ctx := context.Background()
	req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, nil) // OK
	cli.Do(req2)

	req3, _ := http.NewRequest(http.MethodPost, url, nil) // OK
	req3 = req3.WithContext(ctx)
	cli.Do(req3)

	f2 := func(req *http.Request, ctx context.Context) *http.Request {
		return req
	}
	req4, _ := http.NewRequest(http.MethodPost, url, nil) // want `should rewrite http.NewRequestWithContext or add \(\*Request\).WithContext`
	req4 = f2(req4, ctx)
	cli.Do(req4)

	req5, _ := func() (*http.Request, error) {
		return http.NewRequest(http.MethodPost, url, nil) // want `should rewrite http.NewRequestWithContext or add \(\*Request\).WithContext`
	}()
	cli.Do(req5)

}
```

## Reference

- [net/http - NewRequest](https://golang.org/pkg/net/http/#NewRequest)
- [net/http - NewRequestWithContext](https://golang.org/pkg/net/http/#NewRequestWithContext)
- [net/http - Request.WithContext](https://golang.org/pkg/net/http/#Request.WithContext)

