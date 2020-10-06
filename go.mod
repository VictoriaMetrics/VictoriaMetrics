module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go v0.68.0 // indirect
	cloud.google.com/go/storage v1.12.0
	github.com/VictoriaMetrics/fastcache v1.5.7

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.0.7
	github.com/VictoriaMetrics/metrics v1.12.3
	github.com/VictoriaMetrics/metricsql v0.6.0
	github.com/aws/aws-sdk-go v1.35.3
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/go-kit/kit v0.10.0
	github.com/golang/snappy v0.0.2
	github.com/klauspost/compress v1.11.1
	github.com/prometheus/common v0.14.0 // indirect
	github.com/prometheus/procfs v0.2.0 // indirect
	github.com/prometheus/prometheus v1.8.2-0.20200911110723-e83ef207b6c2
	github.com/valyala/fastjson v1.6.1
	github.com/valyala/fastrand v1.0.0
	github.com/valyala/fasttemplate v1.2.1
	github.com/valyala/gozstd v1.8.3
	github.com/valyala/histogram v1.1.2
	github.com/valyala/quicktemplate v1.6.3
	go.uber.org/atomic v1.7.0 // indirect
	golang.org/x/net v0.0.0-20201002202402-0a1ea396d57c // indirect
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43
	golang.org/x/sync v0.0.0-20200930132711-30421366ff76 // indirect
	golang.org/x/sys v0.0.0-20201005172224-997123666555
	golang.org/x/tools v0.0.0-20201005185003-576e169c3de7 // indirect
	google.golang.org/api v0.32.0
	google.golang.org/genproto v0.0.0-20201006033701-bcad7cf615f2 // indirect
	gopkg.in/yaml.v2 v2.3.0
)

go 1.13
