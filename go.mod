module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go v0.61.0 // indirect
	cloud.google.com/go/storage v1.10.0
	github.com/VictoriaMetrics/fastcache v1.5.7

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.0.1
	github.com/VictoriaMetrics/metrics v1.12.0
	github.com/VictoriaMetrics/metricsql v0.2.9
	github.com/aws/aws-sdk-go v1.33.9
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/golang/snappy v0.0.1
	github.com/klauspost/compress v1.10.10
	github.com/lithammer/go-jump-consistent-hash v1.0.1
	github.com/valyala/fastjson v1.5.3
	github.com/valyala/fastrand v1.0.0
	github.com/valyala/gozstd v1.7.0
	github.com/valyala/histogram v1.1.2
	github.com/valyala/quicktemplate v1.5.1
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.0.0-20200720211630-cb9d2d5c5666
	golang.org/x/tools v0.0.0-20200721223218-6123e77877b2 // indirect
	google.golang.org/api v0.29.0
	google.golang.org/genproto v0.0.0-20200722002428-88e341933a54 // indirect
	gopkg.in/yaml.v2 v2.3.0
)

go 1.13
