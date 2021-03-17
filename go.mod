module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go/storage v1.14.0
	github.com/VictoriaMetrics/fastcache v1.5.8

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.0.14
	github.com/VictoriaMetrics/metrics v1.16.0
	github.com/VictoriaMetrics/metricsql v0.14.0
	github.com/aws/aws-sdk-go v1.37.31
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/cheggaaa/pb/v3 v3.0.6
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/fatih/color v1.10.0 // indirect
	github.com/golang/snappy v0.0.3
	github.com/influxdata/influxdb v1.8.4
	github.com/klauspost/compress v1.11.12
	github.com/lithammer/go-jump-consistent-hash v1.0.1
	github.com/mattn/go-runewidth v0.0.10 // indirect
	github.com/prometheus/client_golang v1.9.0 // indirect
	github.com/prometheus/common v0.19.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
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
	golang.org/x/net v0.0.0-20210316092652-d523dce5a7f4 // indirect
	golang.org/x/oauth2 v0.0.0-20210313182246-cd4f82c27b84
	golang.org/x/sys v0.0.0-20210316164454-77fc1eacc6aa
	google.golang.org/api v0.42.0
	google.golang.org/genproto v0.0.0-20210315173758-2651cd453018 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

go 1.14
