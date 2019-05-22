[![Build Status](https://travis-ci.org/VictoriaMetrics/fastcache.svg)](https://travis-ci.org/VictoriaMetrics/fastcache)
[![GoDoc](https://godoc.org/github.com/VictoriaMetrics/fastcache?status.svg)](http://godoc.org/github.com/VictoriaMetrics/fastcache)
[![Go Report](https://goreportcard.com/badge/github.com/VictoriaMetrics/fastcache)](https://goreportcard.com/report/github.com/VictoriaMetrics/fastcache)
[![codecov](https://codecov.io/gh/VictoriaMetrics/fastcache/branch/master/graph/badge.svg)](https://codecov.io/gh/VictoriaMetrics/fastcache)

# fastcache - fast thread-safe inmemory cache for big number of entries in Go

### Features

* Fast. Performance scales on multi-core CPUs. See benchmark results below.
* Thread-safe. Concurrent goroutines may read and write into a single
  cache instance.
* The fastcache is designed for storing big number of entries without
  [GC overhead](https://syslog.ravelin.com/further-dangers-of-large-heaps-in-go-7a267b57d487).
* Fastcache automatically evicts old entries when reaching the maximum cache size
  set on its creation.
* [Simple API](http://godoc.org/github.com/VictoriaMetrics/fastcache).
* Simple source code.
* Cache may be [saved to file](https://godoc.org/github.com/VictoriaMetrics/fastcache#Cache.SaveToFile)
  and [loaded from file](https://godoc.org/github.com/VictoriaMetrics/fastcache#LoadFromFile).
* Works on [Google AppEngine](https://cloud.google.com/appengine/docs/go/).


### Benchmarks

`Fastcache` performance is compared with [BigCache](https://github.com/allegro/bigcache), standard Go map
and [sync.Map](https://golang.org/pkg/sync/#Map).

```
GOMAXPROCS=4 go test github.com/VictoriaMetrics/fastcache -bench='Set|Get' -benchtime=10s
goos: linux
goarch: amd64
pkg: github.com/VictoriaMetrics/fastcache
BenchmarkBigCacheSet-4      	    2000	  10566656 ns/op	   6.20 MB/s	 4660369 B/op	       6 allocs/op
BenchmarkBigCacheGet-4      	    2000	   6902694 ns/op	   9.49 MB/s	  684169 B/op	  131076 allocs/op
BenchmarkBigCacheSetGet-4   	    1000	  17579118 ns/op	   7.46 MB/s	 5046744 B/op	  131083 allocs/op
BenchmarkCacheSet-4         	    5000	   3808874 ns/op	  17.21 MB/s	    1142 B/op	       2 allocs/op
BenchmarkCacheGet-4         	    5000	   3293849 ns/op	  19.90 MB/s	    1140 B/op	       2 allocs/op
BenchmarkCacheSetGet-4      	    2000	   8456061 ns/op	  15.50 MB/s	    2857 B/op	       5 allocs/op
BenchmarkStdMapSet-4        	    2000	  10559382 ns/op	   6.21 MB/s	  268413 B/op	   65537 allocs/op
BenchmarkStdMapGet-4        	    5000	   2687404 ns/op	  24.39 MB/s	    2558 B/op	      13 allocs/op
BenchmarkStdMapSetGet-4     	     100	 154641257 ns/op	   0.85 MB/s	  387405 B/op	   65558 allocs/op
BenchmarkSyncMapSet-4       	     500	  24703219 ns/op	   2.65 MB/s	 3426543 B/op	  262411 allocs/op
BenchmarkSyncMapGet-4       	    5000	   2265892 ns/op	  28.92 MB/s	    2545 B/op	      79 allocs/op
BenchmarkSyncMapSetGet-4    	    1000	  14595535 ns/op	   8.98 MB/s	 3417190 B/op	  262277 allocs/op
```

`MB/s` column here actually means `millions of operations per second`.
As you can see, `fastcache` is faster than the `BigCache` in all the cases.
`fastcache` is faster than the standard Go map and `sync.Map` on workloads
with inserts.


### Limitations

* Keys and values must be byte slices. Other types must be marshaled before
  storing them in the cache.
* Big entries with sizes exceeding 64KB must be stored via [distinct API](http://godoc.org/github.com/VictoriaMetrics/fastcache#Cache.SetBig).
* There is no cache expiration. Entries are evicted from the cache only
  on cache size overflow. Entry deadline may be stored inside the value in order
  to implement cache expiration.


### Architecture details

The cache uses ideas from [BigCache](https://github.com/allegro/bigcache):

* The cache consists of many buckets, each with its own lock.
  This helps scaling the performance on multi-core CPUs, since multiple
  CPUs may concurrently access distinct buckets.
* Each bucket consists of a `hash(key) -> (key, value) position` map
  and 64KB-sized byte slices (chunks) holding encoded `(key, value)` entries.
  Each bucket contains only `O(chunksCount)` pointers. For instance, 64GB cache
  would contain ~1M pointers, while similarly-sized `map[string][]byte`
  would contain ~1B pointers for short keys and values. This would lead to
  [huge GC overhead](https://syslog.ravelin.com/further-dangers-of-large-heaps-in-go-7a267b57d487).

64KB-sized chunks reduce memory fragmentation and the total memory usage comparing
to a single big chunk per bucket.
Chunks are allocated off-heap if possible. This reduces total memory usage because
GC collects unused memory more frequently without the need in `GOGC` tweaking.


### Users

* `Fastcache` has been extracted from [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) sources.
  See [this article](https://medium.com/devopslinks/victoriametrics-creating-the-best-remote-storage-for-prometheus-5d92d66787ac)
  for more info about `VictoriaMetrics`.


### FAQ

#### What is the difference between `fastcache` and other similar caches like [BigCache](https://github.com/allegro/bigcache) or [FreeCache](https://github.com/coocood/freecache)?

* `Fastcache` is faster. See benchmark results above.
* `Fastcache` uses less memory due to lower heap fragmentation. This allows
  saving many GBs of memory on multi-GB caches.
* `Fastcache` API [is simpler](http://godoc.org/github.com/VictoriaMetrics/fastcache).
  The API is designed to be used in zero-allocation mode.


#### Why `fastcache` doesn't support cache expiration?

Because we don't need cache expiration in [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics).
Cached entries inside `VictoriaMetrics` never expire. They are automatically evicted on cache size overflow.

It is easy to implement cache expiration on top of `fastcache` by caching values
with marshaled deadlines and verifying deadlines after reading these values
from the cache.


#### Why `fastcache` doesn't support advanced features such as [thundering herd protection](https://en.wikipedia.org/wiki/Thundering_herd_problem) or callbacks on entries' eviction?

Because these features would complicate the code and would make it slower.
`Fastcache` source code is simple - just copy-paste it and implement the feature you want
on top of it.
