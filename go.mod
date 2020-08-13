module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go v0.63.0 // indirect
	cloud.google.com/go/storage v1.10.0
	github.com/VictoriaMetrics/fastcache v1.5.7

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.0.5
	github.com/VictoriaMetrics/metrics v1.12.3
	github.com/VictoriaMetrics/metricsql v0.4.1
	github.com/aws/aws-sdk-go v1.34.0
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/golang/snappy v0.0.1
	github.com/klauspost/compress v1.10.11
	github.com/lithammer/go-jump-consistent-hash v1.0.1
	github.com/valyala/fastjson v1.5.4
	github.com/valyala/fastrand v1.0.0
	github.com/valyala/fasttemplate v1.2.1
	github.com/valyala/gozstd v1.7.0
	github.com/valyala/histogram v1.1.2
	github.com/valyala/quicktemplate v1.6.2
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.0.0-20200808120158-1030fc2bf1d9
	golang.org/x/tools v0.0.0-20200809012840-6f4f008689da // indirect
	google.golang.org/api v0.30.0
	google.golang.org/genproto v0.0.0-20200808173500-a06252235341 // indirect
	gopkg.in/yaml.v2 v2.3.0
)

go 1.13
