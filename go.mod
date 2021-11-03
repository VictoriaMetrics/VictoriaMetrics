module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go/storage v1.18.2
	github.com/VictoriaMetrics/fastcache v1.7.0

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.1.0
	github.com/VictoriaMetrics/metrics v1.18.1
	github.com/VictoriaMetrics/metricsql v0.28.0
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/aws/aws-sdk-go v1.41.14
	github.com/census-instrumentation/opencensus-proto v0.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/cncf/udpa/go v0.0.0-20210930031921-04548b0d99d4 // indirect
	github.com/cncf/xds/go v0.0.0-20211011173535-cb28da3451f1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/envoyproxy/go-control-plane v0.10.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v0.6.2 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/golang/snappy v0.0.4
	github.com/influxdata/influxdb v1.9.5
	github.com/klauspost/compress v1.13.6
	github.com/lithammer/go-jump-consistent-hash v1.0.2
	github.com/mattn/go-colorable v0.1.11 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/prometheus v1.8.2-0.20201119142752-3ad25a6dc3d9
	github.com/urfave/cli/v2 v2.3.0
	github.com/valyala/fastjson v1.6.3
	github.com/valyala/fastrand v1.1.0
	github.com/valyala/fasttemplate v1.2.1
	github.com/valyala/gozstd v1.14.2
	github.com/valyala/quicktemplate v1.7.0
	golang.org/x/net v0.0.0-20211029224645-99673261e6eb
	golang.org/x/oauth2 v0.0.0-20211028175245-ba495a64dcb5
	golang.org/x/sys v0.0.0-20211031064116-611d5d643895
	google.golang.org/api v0.60.0
	google.golang.org/genproto v0.0.0-20211029142109-e255c875f7c7 // indirect
	google.golang.org/grpc v1.41.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

go 1.16
