module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go/storage v1.8.0
	github.com/VictoriaMetrics/fastcache v1.5.7

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.0.1
	github.com/VictoriaMetrics/metrics v1.11.3
	github.com/VictoriaMetrics/metricsql v0.2.3
	github.com/aws/aws-sdk-go v1.31.5
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/golang/snappy v0.0.1
	github.com/klauspost/compress v1.10.8
	github.com/valyala/fastjson v1.5.1
	github.com/valyala/fastrand v1.0.0
	github.com/valyala/gozstd v1.7.0
	github.com/valyala/histogram v1.0.1
	github.com/valyala/quicktemplate v1.5.0
	golang.org/x/mod v0.3.0 // indirect
	golang.org/x/net v0.0.0-20200520182314-0ba52f642ac2 // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.0.0-20200523222454-059865788121
	golang.org/x/tools v0.0.0-20200527150044-688b3c5d9fa5 // indirect
	google.golang.org/api v0.25.0
	google.golang.org/genproto v0.0.0-20200527145253-8367513e4ece // indirect
	gopkg.in/yaml.v2 v2.3.0
	honnef.co/go/tools v0.0.1-2020.1.4 // indirect
)

go 1.13
