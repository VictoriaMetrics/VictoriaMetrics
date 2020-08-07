module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go/storage v1.10.0
	github.com/VictoriaMetrics/fastcache v1.5.7

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.0.4
	github.com/VictoriaMetrics/metrics v1.12.2
	github.com/VictoriaMetrics/metricsql v0.4.1
	github.com/aws/aws-sdk-go v1.33.19
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/golang/snappy v0.0.1
	github.com/klauspost/compress v1.10.10
	github.com/valyala/fastjson v1.5.4
	github.com/valyala/fastrand v1.0.0
	github.com/valyala/gozstd v1.7.0
	github.com/valyala/histogram v1.1.2
	github.com/valyala/quicktemplate v1.6.2
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.0.0-20200805065543-0cf7623e9dbd
	golang.org/x/tools v0.0.0-20200804234916-fec4f28ebb08 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/api v0.30.0
	google.golang.org/genproto v0.0.0-20200804151602-45615f50871c // indirect
	gopkg.in/yaml.v2 v2.3.0
)

go 1.13
