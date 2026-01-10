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
  -cluster.tls
     Whether to use TLS for connections to -storageNode. See https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -cluster.tlsCAFile string
     Path to TLS CA file to use for verifying certificates provided by -storageNode if -cluster.tls flag is set. By default system CA is used. See https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -cluster.tlsCertFile string
     Path to client-side TLS certificate file to use when connecting to -storageNode if -cluster.tls flag is set. See https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -cluster.tlsInsecureSkipVerify
     Whether to skip verification of TLS certificates provided by -storageNode nodes if -cluster.tls flag is set. Note that disabled TLS certificate verification breaks security. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -cluster.tlsKeyFile string
     Path to client-side TLS key file to use when connecting to -storageNode if -cluster.tls flag is set. See https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/victoriametrics/enterprise/
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
  -search.denyPartialResponse
     Whether to deny partial responses if a part of -storageNode instances fail to perform queries; this trades availability over consistency; see also -search.maxQueryDuration
  -storageNode.discoveryInterval duration
     Interval for refreshing -storageNode list behind DNS SRV records. The minimum supported interval is 1s. See https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#automatic-vmstorage-discovery . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 2s)
  -storageNode.filter string
     An optional regexp filter for discovered -storageNode addresses according to https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#automatic-vmstorage-discovery. Discovered addresses matching the filter are retained, while other addresses are ignored. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -tlsAutocertCacheDir string
     Directory to store TLS certificates issued via Let's Encrypt. Certificates are lost on restarts if this flag isn't set. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -tlsAutocertEmail string
     Contact email for the issued Let's Encrypt TLS certificates. See also -tlsAutocertHosts and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -tlsAutocertHosts array
     Optional hostnames for automatic issuing of Let's Encrypt TLS certificates. These hostnames must be reachable at -httpListenAddr . The -httpListenAddr must listen tcp port 443 . The -tlsAutocertHosts overrides -tlsCertFile and -tlsKeyFile . See also -tlsAutocertEmail and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
```