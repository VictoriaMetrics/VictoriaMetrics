module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go/storage v1.14.0
	github.com/VictoriaMetrics/fastcache v1.5.8

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.0.14
	github.com/VictoriaMetrics/metrics v1.17.2
	github.com/VictoriaMetrics/metricsql v0.14.0
	github.com/aws/aws-sdk-go v1.38.15
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/cheggaaa/pb/v3 v3.0.7
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/fatih/color v1.10.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v0.0.3
	github.com/influxdata/influxdb v1.8.4
	github.com/klauspost/compress v1.11.13
	github.com/mattn/go-runewidth v0.0.12 // indirect
	github.com/prometheus/client_golang v1.10.0 // indirect
	github.com/prometheus/common v0.20.0 // indirect
	github.com/prometheus/prometheus v1.8.2-0.20201119142752-3ad25a6dc3d9
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/urfave/cli/v2 v2.3.0
	github.com/valyala/fastjson v1.6.3
	github.com/valyala/fastrand v1.0.0
	github.com/valyala/fasttemplate v1.2.1
	github.com/valyala/gozstd v1.9.0
	github.com/valyala/histogram v1.1.2
	github.com/valyala/quicktemplate v1.6.3
	golang.org/x/mod v0.4.2 // indirect
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4
	golang.org/x/oauth2 v0.0.0-20210402161424-2e8d93401602
	golang.org/x/sys v0.0.0-20210403161142-5e06dd20ab57
	golang.org/x/text v0.3.6 // indirect
	google.golang.org/api v0.44.0
	google.golang.org/genproto v0.0.0-20210406143921-e86de6bf7a46 // indirect
	google.golang.org/grpc v1.37.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

go 1.14
