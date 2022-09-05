module github.com/VictoriaMetrics/VictoriaMetrics

go 1.18

require (
	cloud.google.com/go/storage v1.26.0
	github.com/VictoriaMetrics/fastcache v1.10.0

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.1.0
	github.com/VictoriaMetrics/metrics v1.22.2
	github.com/VictoriaMetrics/metricsql v0.44.1
	github.com/aws/aws-sdk-go v1.44.91
	github.com/cespare/xxhash/v2 v2.1.2

	// TODO: switch back to https://github.com/cheggaaa/pb/v3 when v3-pooling branch
	// is merged into main branch.
	// See https://github.com/cheggaaa/pb/pull/192#issuecomment-1121285954 for details.
	github.com/dmitryk-dk/pb/v3 v3.0.9
	github.com/golang/snappy v0.0.4
	github.com/influxdata/influxdb v1.10.0
	github.com/klauspost/compress v1.15.9
	github.com/prometheus/prometheus v1.8.2-0.20201119142752-3ad25a6dc3d9
	github.com/urfave/cli/v2 v2.14.0
	github.com/valyala/fastjson v1.6.3
	github.com/valyala/fastrand v1.1.0
	github.com/valyala/fasttemplate v1.2.1
	github.com/valyala/gozstd v1.17.0
	github.com/valyala/quicktemplate v1.7.0
	golang.org/x/net v0.0.0-20220826154423-83b083e8dc8b
	golang.org/x/oauth2 v0.0.0-20220822191816-0ebed06d0094
	golang.org/x/sys v0.0.0-20220829200755-d48e67d00261
	google.golang.org/api v0.94.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	cloud.google.com/go v0.104.0 // indirect
	cloud.google.com/go/compute v1.9.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/googleapis/gax-go/v2 v2.5.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/prometheus/client_golang v1.13.0 // indirect
	github.com/rivo/uniseg v0.3.4 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/sync v0.0.0-20220819030929-7fc1605a5dde // indirect
	google.golang.org/genproto v0.0.0-20220902135211-223410557253 // indirect
	google.golang.org/grpc v1.49.0 // indirect
)
