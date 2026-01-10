---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
<!-- The file has to be manually updated during feature work in PR, make docs-update-flags command could be used peridically to ensure the flags in sync. -->
```shellhelp
  -clusterMode
     If clusterMode is enabled, then vmalert automatically adds the tenant specified in config groups to -datasource.url, -remoteWrite.url and -remoteRead.url. See https://docs.victoriametrics.com/victoriametrics/vmalert/#multitenancy . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -defaultTenant.graphite string
     Default tenant for Graphite alerting groups. See https://docs.victoriametrics.com/victoriametrics/vmalert/#multitenancy .This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -defaultTenant.prometheus string
     Default tenant for Prometheus alerting groups. See https://docs.victoriametrics.com/victoriametrics/vmalert/#multitenancy . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -eula
     Deprecated, please use -license or -licenseFile flags instead. By specifying this flag, you confirm that you have an enterprise license and accept the ESA https://victoriametrics.com/legal/esa/ . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -license string
     License key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/ . Trial Enterprise license can be obtained from https://victoriametrics.com/products/enterprise/trial/ . This flag is available only in Enterprise binaries. The license key can be also passed via file specified by -licenseFile command-line flag
  -license.forceOffline
     Whether to enable offline verification for VictoriaMetrics Enterprise license key, which has been passed either via -license or via -licenseFile command-line flag. The issued license key must support offline verification feature. Contact info@victoriametrics.com if you need offline license verification. This flag is available only in Enterprise binaries
  -licenseFile string
     Path to file with license key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/ . Trial Enterprise license can be obtained from https://victoriametrics.com/products/enterprise/trial/ . This flag is available only in Enterprise binaries. The license key can be also passed inline via -license command-line flag
  -licenseFile.reloadInterval duration
     Interval for reloading the license file specified via -licenseFile. See https://victoriametrics.com/products/enterprise/ . This flag is available only in Enterprise binaries (default 1h0m0s)
  -mtls array
     Whether to require valid client certificate for https requests to the corresponding -httpListenAddr . This flag works only if -tls flag is set. See also -mtlsCAFile . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -mtlsCAFile array
     Optional path to TLS Root CA for verifying client certificates at the corresponding -httpListenAddr when -mtls is enabled. By default the host system TLS Root CA is used for client certificate verification. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -rule.stripFilePath
     Whether to strip file path in responses from the api/v1/rules API for files configured via -rule cmd-line flag. For example, the file path '/path/to/tenant_id/rules.yml' will be stripped to just 'rules.yml'. This flag might be useful to hide sensitive information in file path such as tenant ID. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -s3.configFilePath string
     Path to file with S3 configs. Configs are loaded from default location if not set.
     See https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -s3.configProfile string
     Profile name for S3 configs. If no set, the value of the environment variable will be loaded (AWS_PROFILE or AWS_DEFAULT_PROFILE), or if both not set, DefaultSharedConfigProfile is used. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -s3.credsFilePath string
     Path to file with GCS or S3 credentials. Credentials are loaded from default locations if not set.
     See https://cloud.google.com/iam/docs/creating-managing-service-account-keys and https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -s3.customEndpoint string
     Custom S3 endpoint for use with S3-compatible storages (e.g. MinIO). S3 is used if not set. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -s3.forcePathStyle
     Prefixing endpoint with bucket name when set false, true by default. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default true)
  -tlsAutocertCacheDir string
     Directory to store TLS certificates issued via Let's Encrypt. Certificates are lost on restarts if this flag isn't set. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -tlsAutocertEmail string
     Contact email for the issued Let's Encrypt TLS certificates. See also -tlsAutocertHosts and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -tlsAutocertHosts array
     Optional hostnames for automatic issuing of Let's Encrypt TLS certificates. These hostnames must be reachable at -httpListenAddr . The -httpListenAddr must listen tcp port 443 . The -tlsAutocertHosts overrides -tlsCertFile and -tlsKeyFile . See also -tlsAutocertEmail and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
```