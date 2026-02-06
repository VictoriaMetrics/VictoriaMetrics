# unixtransport [![Go Reference](https://pkg.go.dev/badge/github.com/peterbourgon/unixtransport.svg)](https://pkg.go.dev/github.com/peterbourgon/unixtransport) ![Latest Release](https://img.shields.io/github/v/release/peterbourgon/unixtransport?style=flat-square) ![Tests](https://github.com/peterbourgon/unixtransport/actions/workflows/test.yaml/badge.svg?branch=main)

This package adds support for Unix sockets to Go HTTP clients and servers.


## Clients

Register the Unix protocol in the default HTTP client transport like this:

```go
unixtransport.RegisterDefault()
```

Now you can make HTTP requests with URLs like this:

```go
resp, err := http.Get("http+unix:///path/to/socket:/request/path?a=b#fragment")
```

Use scheme `http+unix` or `https+unix`, and use `:` to separate the socket file
path (host) from the URL request path.

See e.g. [Register][register] and [RegisterDefault][registerdef] for more info.


## Servers

If you have this:

```go
fs := flag.NewFlagSet("myserver", flag.ContinueOnError)
addr := fs.String("addr", ":8080", "listen address")
...
http.ListenAndServe(*addr, nil)
```

You can change it like this:

```diff
 fs := flag.NewFlagSet("myserver", flag.ContinueOnError)
 addr := fs.String("addr", ":8080", "listen address")
 ...
-http.ListenAndServe(*addr, nil)
+ln, err := unixtransport.ListenURI(context.TODO(), *addr)
+// handle err
+http.Serve(ln, nil)
```

Which lets you specify addrs like this:

```shell
myserver -addr=:8080                # equivalent to `tcp://:8080`
myserver -addr=tcp://:8080          # listen on all interfaces, TCP port 8080
myserver -addr=udp://0.0.0.0:12345  # listen on all IPv4 interfaces, UDP port 12345
myserver -addr=unix:///tmp/mysocket # listen on Unix socket path /tmp/mysocket
```

See [ParseURI][parseuri] and [ListenURI][listenuri] for more info.


## Acknowledgements

Inspiration taken from, and thanks given to, both [tv42/httpunix][tv42] and
[agorman/httpunix][agorman].


[register]: https://pkg.go.dev/github.com/peterbourgon/unixtransport#Register
[registerdef]: https://pkg.go.dev/github.com/peterbourgon/unixtransport#RegisterDefault
[parseuri]: https://pkg.go.dev/github.com/peterbourgon/unixtransport#ParseURI
[listenuri]: https://pkg.go.dev/github.com/peterbourgon/unixtransport#ListenURI
[tv42]: https://github.com/tv42/httpunix
[agorman]: https://github.com/agorman/httpunix
