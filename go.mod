module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go v0.95.0 // indirect
	cloud.google.com/go/storage v1.16.1
	github.com/VictoriaMetrics/fastcache v1.7.0

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.1.0
	github.com/VictoriaMetrics/metrics v1.18.0
	github.com/VictoriaMetrics/metricsql v0.24.0
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/aws/aws-sdk-go v1.40.47
	github.com/cespare/xxhash/v2 v2.1.2
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/go-kit/kit v0.11.0
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v0.0.4
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	github.com/influxdata/influxdb v1.9.4
	github.com/klauspost/compress v1.13.6
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/oklog/ulid v1.3.1
	github.com/prometheus/common v0.30.0 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/prometheus/prometheus v1.8.2-0.20201119142752-3ad25a6dc3d9
	github.com/urfave/cli/v2 v2.3.0
	github.com/valyala/fastjson v1.6.3
	github.com/valyala/fastrand v1.1.0
	github.com/valyala/fasttemplate v1.2.1
	github.com/valyala/gozstd v1.13.0
	github.com/valyala/histogram v1.2.0
	github.com/valyala/quicktemplate v1.7.0
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/net v0.0.0-20210917221730-978cfadd31cf
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f
	golang.org/x/sys v0.0.0-20210923061019-b8560ed6a9b7
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/api v0.57.0
	gopkg.in/yaml.v2 v2.4.0
)

// This is needed until https://github.com/googleapis/google-cloud-go/issues/4783 is resolved
replace cloud.google.com/go v0.94.1 => cloud.google.com/go v0.93.3

go 1.16
