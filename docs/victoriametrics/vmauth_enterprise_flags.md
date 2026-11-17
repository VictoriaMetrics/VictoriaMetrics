---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
<!-- The file should not be updated manually. Run make docs-update-flags while preparing a new release to sync flags in docs from actual binaries. -->
```shellhelp
  -eula
     By specifying this flag, you accept the ESA https://victoriametrics.com/legal/esa/ . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -httpRealIPHeader string
     HTTP request header to use for obtaining IP address of client for applying 'ip_filters'. By default vmauth uses IP address of TCP the client. Useful if vmauth is behind reverse-proxy. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -license string
     This flag is deprecated and has no effect. Use -eula if you intended to run the Enterprise version.
  -license.forceOffline
     This flag is deprecated and has no effect. Use -eula if you intended to run the Enterprise version.
  -licenseFile string
     This flag is deprecated and has no effect. Use -eula if you intended to run the Enterprise version.
  -licenseFile.reloadInterval duration
     This flag is deprecated and has no effect. Use -eula if you intended to run the Enterprise version. (default 1h0m0s)
  -mtls array
     Whether to require valid client certificate for https requests to the corresponding -httpListenAddr . This flag works only if -tls flag is set. See also -mtlsCAFile . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -mtlsCAFile array
     Optional path to TLS Root CA for verifying client certificates at the corresponding -httpListenAddr when -mtls is enabled. By default the host system TLS Root CA is used for client certificate verification. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsAutocertCacheDir string
     Directory to store TLS certificates issued via Let's Encrypt. Certificates are lost on restarts if this flag isn't set. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -tlsAutocertEmail string
     Contact email for the issued Let's Encrypt TLS certificates. See also -tlsAutocertHosts and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -tlsAutocertHosts array
     Optional hostnames for automatic issuing of Let's Encrypt TLS certificates. These hostnames must be reachable at -httpListenAddr . The -httpListenAddr must listen tcp port 443 . The -tlsCertFile and -tlsKeyFile must be unset when -tlsAutocertHosts is used . See also -tlsAutocertEmail and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
```