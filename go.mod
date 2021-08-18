module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go v0.91.1 // indirect
	cloud.google.com/go/storage v1.16.0
	github.com/VictoriaMetrics/fastcache v1.6.0

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.0.16
	github.com/VictoriaMetrics/metrics v1.17.3
	github.com/VictoriaMetrics/metricsql v0.18.0
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/aws/aws-sdk-go v1.40.22
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/fatih/color v1.12.0 // indirect
	github.com/go-kit/kit v0.11.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v0.0.4
	github.com/influxdata/influxdb v1.9.3
	github.com/klauspost/compress v1.13.4
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/prometheus/common v0.30.0 // indirect
	github.com/prometheus/procfs v0.7.2 // indirect
	github.com/prometheus/prometheus v1.8.2-0.20201119142752-3ad25a6dc3d9
	github.com/urfave/cli/v2 v2.3.0
	github.com/valyala/fastjson v1.6.3
	github.com/valyala/fastrand v1.0.0
	github.com/valyala/fasttemplate v1.2.1
	github.com/valyala/gozstd v1.12.0
	github.com/valyala/histogram v1.1.2
	github.com/valyala/quicktemplate v1.6.3
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/net v0.0.0-20210805182204-aaa1db679c0d
	golang.org/x/oauth2 v0.0.0-20210810183815-faf39c7919d5
	golang.org/x/sys v0.0.0-20210809222454-d867a43fc93e
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/api v0.54.0
	google.golang.org/grpc v1.40.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

go 1.16
