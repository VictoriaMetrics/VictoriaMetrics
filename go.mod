module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go v0.57.0 // indirect
	cloud.google.com/go/storage v1.7.0
	github.com/VictoriaMetrics/fastcache v1.5.7

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.0.1
	github.com/VictoriaMetrics/metrics v1.11.2
	github.com/VictoriaMetrics/metricsql v0.2.0
	github.com/aws/aws-sdk-go v1.30.25
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/golang/protobuf v1.4.1 // indirect
	github.com/golang/snappy v0.0.1
	github.com/klauspost/compress v1.10.5
	github.com/valyala/fastjson v1.5.1
	github.com/valyala/fastrand v1.0.0
	github.com/valyala/gozstd v1.7.0
	github.com/valyala/histogram v1.0.1
	github.com/valyala/quicktemplate v1.5.0
	golang.org/x/net v0.0.0-20200506145744-7e3656a0809f // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.0.0-20200511232937-7e40ca221e25
	golang.org/x/tools v0.0.0-20200512001501-aaeff5de670a // indirect
	google.golang.org/api v0.24.0
	google.golang.org/genproto v0.0.0-20200511104702-f5ebc3bea380 // indirect
	gopkg.in/yaml.v2 v2.2.8
)

go 1.13
