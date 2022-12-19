module github.com/VictoriaMetrics/VictoriaMetrics

go 1.19

require (
	cloud.google.com/go/storage v1.28.1
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.2.0
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v0.6.1
	github.com/VictoriaMetrics/fastcache v1.12.0

	// Do not use the original github.com/valyala/fasthttp because of issues
	// like https://github.com/valyala/fasthttp/commit/996610f021ff45fdc98c2ce7884d5fa4e7f9199b
	github.com/VictoriaMetrics/fasthttp v1.1.0
	github.com/VictoriaMetrics/metrics v1.23.0
	github.com/VictoriaMetrics/metricsql v0.50.0
	github.com/aws/aws-sdk-go-v2 v1.17.3
	github.com/aws/aws-sdk-go-v2/config v1.18.4
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.11.43
	github.com/aws/aws-sdk-go-v2/service/s3 v1.29.5
	github.com/cespare/xxhash/v2 v2.2.0
	github.com/cheggaaa/pb/v3 v3.1.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/snappy v0.0.4
	github.com/googleapis/gax-go/v2 v2.7.0
	github.com/influxdata/influxdb v1.11.0
	github.com/klauspost/compress v1.15.13
	github.com/prometheus/prometheus v0.40.7
	github.com/urfave/cli/v2 v2.23.7
	github.com/valyala/fastjson v1.6.3
	github.com/valyala/fastrand v1.1.0
	github.com/valyala/fasttemplate v1.2.2
	github.com/valyala/gozstd v1.17.0
	github.com/valyala/quicktemplate v1.7.0
	golang.org/x/net v0.4.0
	golang.org/x/oauth2 v0.3.0
	golang.org/x/sys v0.3.0
	google.golang.org/api v0.104.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	cloud.google.com/go v0.107.0 // indirect
	cloud.google.com/go/compute v1.14.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.2 // indirect
	cloud.google.com/go/iam v0.8.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.1.2 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/aws/aws-sdk-go v1.44.160 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.4.10 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.26 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.0.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.9.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.1.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.13.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.26 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.13.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.17.6 // indirect
	github.com/aws/smithy-go v1.13.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.1 // indirect
	github.com/grafana/regexp v0.0.0-20221122212121-6b5c0a4cb7fd // indirect
	github.com/hashicorp/go-hclog v0.16.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.39.0 // indirect
	github.com/prometheus/common/sigv4 v0.1.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/rivo/uniseg v0.4.3 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/stretchr/testify v1.8.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/histogram v1.2.0 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.37.0 // indirect
	go.opentelemetry.io/otel v1.11.2 // indirect
	go.opentelemetry.io/otel/metric v0.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.11.2 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/goleak v1.2.0 // indirect
	golang.org/x/exp v0.0.0-20221212164502-fae10dda9338 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/text v0.5.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20221207170731-23e4bf6bdc37 // indirect
	google.golang.org/grpc v1.51.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
