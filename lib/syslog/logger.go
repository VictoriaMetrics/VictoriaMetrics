package syslog

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"os"
)

var (
	syslogNetwork = flag.String("syslog.network", "", "network connection to establish. Options are tcp, udp, tcp+tls. If network is empty, it will connect to the local syslog server")
	syslogAddress = flag.String("syslog.address", "", "Required: Network address of the syslog server.")
	syslogTag     = flag.String("syslog.tag", "", "tag for syslog. Used os.args[0] if empty")

	tlsCertFile           = flag.String("syslog.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -syslogAddress")
	tlsKeyFile            = flag.String("syslog.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -syslogAddress")
	tlsCAFile             = flag.String("syslog.tlsCAFile", "", "Optional path to TLS CA file to use for verifying connections to syslogAddress. By default, system CA is used")
	tlsServerName         = flag.String("syslog.tlsServerName", "", "Optional TLS server name to use for connections to -syslogAddress. By default, the server name from -syslogAddress is used")
	tlsInsecureSkipVerify = flag.Bool("syslog.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -syslogAddress")
)

var defaultPrioritySelector = map[string]Priority{
	"INFO":  LOG_INFO,
	"WARN":  LOG_WARNING,
	"ERROR": LOG_ERR,
	"FATAL": LOG_CRIT,
	"PANIC": LOG_EMERG,
}

func parsePriority(prior string) Priority {
	v, found := defaultPrioritySelector[prior]
	if !found {
		// default the facility level to LOG_LOCAL7
		return LOG_INFO
	}
	return v
}

// returns an instance of syslog writer based on given priority
func GetSyslogWriter(prior string) *Writer {
	prio := parsePriority(prior)
	switch *syslogNetwork {
	case "", "tcp", "udp":
		op, err := Dial(*syslogNetwork, *syslogAddress, prio, *syslogTag)
		if err != nil {
			panic(fmt.Errorf("error dialing syslog: %w", err))
		}
		return op
	case "tcp+tls":
		tc, err := TLSConfig(*tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
		if err != nil {
			panic(fmt.Errorf("error creating TLS config: %w", err))
		}
		op, err := DialWithTLSConfig(*syslogNetwork, *syslogAddress, prio, *syslogTag, tc)
		if err != nil {
			panic(fmt.Errorf("error dialing syslog with tcp+tls: %w", err))
		}
		return op
	default:
		panic(fmt.Errorf("FATAL: unsupported `syslogNetwork` value: %q; supported values are: tcp, udp, tcp+tls", *syslogNetwork))
	}
}

// TLSConfig creates tls.Config object from provided arguments
func TLSConfig(certFile, keyFile, CAFile, serverName string, insecureSkipVerify bool) (*tls.Config, error) {
	var certs []tls.Certificate
	if certFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("cannot load TLS certificate from `cert_file`=%q, `key_file`=%q: %w", certFile, keyFile, err)
		}

		certs = []tls.Certificate{cert}
	}

	var rootCAs *x509.CertPool
	if CAFile != "" {
		pem, err := os.ReadFile(CAFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read `ca_file` %q: %w", CAFile, err)
		}

		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("cannot parse data from `ca_file` %q", CAFile)
		}
	}

	return &tls.Config{
		Certificates:       certs,
		InsecureSkipVerify: insecureSkipVerify,
		RootCAs:            rootCAs,
		ServerName:         serverName,
	}, nil
}
