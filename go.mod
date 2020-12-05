module github.com/VictoriaMetrics/VictoriaMetrics

require (
	cloud.google.com/go v0.73.0 // indirect
	cloud.google.com/go/storage v1.12.0
	github.com/VictoriaMetrics/fastcache v1.5.7

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.0.9
	github.com/VictoriaMetrics/metrics v1.12.3
	github.com/VictoriaMetrics/metricsql v0.9.0
	github.com/aws/aws-sdk-go v1.36.2
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/golang/snappy v0.0.2
	github.com/klauspost/compress v1.11.3
	github.com/lithammer/go-jump-consistent-hash v1.0.1
	github.com/valyala/fastjson v1.6.3
	github.com/valyala/fastrand v1.0.0
	github.com/valyala/fasttemplate v1.2.1
	github.com/valyala/gozstd v1.8.3
	github.com/valyala/histogram v1.1.2
	github.com/valyala/quicktemplate v1.6.3
	golang.org/x/net v0.0.0-20201202161906-c7110b5ffcbb // indirect
	golang.org/x/oauth2 v0.0.0-20201203001011-0b49973bad19
	golang.org/x/sys v0.0.0-20201204225414-ed752295db88
	golang.org/x/tools v0.0.0-20201204222352-654352759326 // indirect
	google.golang.org/api v0.36.0
	google.golang.org/genproto v0.0.0-20201204160425-06b3db808446 // indirect
	google.golang.org/grpc v1.34.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

go 1.13
