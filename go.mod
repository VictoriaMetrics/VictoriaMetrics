module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go/storage v1.15.0
	github.com/VictoriaMetrics/fastcache v1.5.8

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.0.15
	github.com/VictoriaMetrics/metrics v1.17.2
	github.com/VictoriaMetrics/metricsql v0.14.0
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/aws/aws-sdk-go v1.38.36
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v0.0.3
	github.com/influxdata/influxdb v1.8.5
	github.com/klauspost/compress v1.12.2
	github.com/prometheus/client_golang v1.10.0 // indirect
	github.com/prometheus/common v0.23.0 // indirect
	github.com/prometheus/prometheus v1.8.2-0.20201119142752-3ad25a6dc3d9
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/urfave/cli/v2 v2.3.0
	github.com/valyala/fastjson v1.6.3
	github.com/valyala/fastrand v1.0.0
	github.com/valyala/fasttemplate v1.2.1
	github.com/valyala/gozstd v1.10.0
	github.com/valyala/histogram v1.1.2
	github.com/valyala/quicktemplate v1.6.3
	golang.org/x/mod v0.4.2 // indirect
	golang.org/x/net v0.0.0-20210508051633-16afe75a6701
	golang.org/x/oauth2 v0.0.0-20210427180440-81ed05c6b58c
	golang.org/x/sys v0.0.0-20210507161434-a76c4d0a0096
	google.golang.org/api v0.46.0
	google.golang.org/genproto v0.0.0-20210506142907-4a47615972c2 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

go 1.14
