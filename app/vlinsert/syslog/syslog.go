package syslog

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	syslogTimezone = flag.String("syslog.timezone", "Local", "Timezone to use when parsing timestamps in RFC3164 syslog messages. Timezone must be a valid IANA Time Zone. "+
		"For example: America/New_York, Europe/Berlin, Etc/GMT+3 . See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/")

	streamFieldsTCP = flagutil.NewArrayString("syslog.streamFields.tcp", "Fields to use as log stream labels for logs ingested via the corresponding -syslog.listenAddr.tcp. "+
		`See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#stream-fields`)
	streamFieldsUDP = flagutil.NewArrayString("syslog.streamFields.udp", "Fields to use as log stream labels for logs ingested via the corresponding -syslog.listenAddr.udp. "+
		`See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#stream-fields`)

	ignoreFieldsTCP = flagutil.NewArrayString("syslog.ignoreFields.tcp", "Fields to ignore at logs ingested via the corresponding -syslog.listenAddr.tcp. "+
		`See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#dropping-fields`)
	ignoreFieldsUDP = flagutil.NewArrayString("syslog.ignoreFields.udp", "Fields to ignore at logs ingested via the corresponding -syslog.listenAddr.udp. "+
		`See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#dropping-fields`)

	decolorizeFieldsTCP = flagutil.NewArrayString("syslog.decolorizeFields.tcp", "Fields to remove ANSI color codes across logs ingested via the corresponding -syslog.listenAddr.tcp. "+
		`See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#decolorizing-fields`)
	decolorizeFieldsUDP = flagutil.NewArrayString("syslog.decolorizeFields.udp", "Fields to remove ANSI color codes across logs ingested via the corresponding -syslog.listenAddr.udp. "+
		`See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#decolorizing-fields`)

	extraFieldsTCP = flagutil.NewArrayString("syslog.extraFields.tcp", "Fields to add to logs ingested via the corresponding -syslog.listenAddr.tcp. "+
		`See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#adding-extra-fields`)
	extraFieldsUDP = flagutil.NewArrayString("syslog.extraFields.udp", "Fields to add to logs ingested via the corresponding -syslog.listenAddr.udp. "+
		`See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#adding-extra-fields`)

	tenantIDTCP = flagutil.NewArrayString("syslog.tenantID.tcp", "TenantID for logs ingested via the corresponding -syslog.listenAddr.tcp. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#multitenancy")
	tenantIDUDP = flagutil.NewArrayString("syslog.tenantID.udp", "TenantID for logs ingested via the corresponding -syslog.listenAddr.udp. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#multitenancy")

	listenAddrTCP = flagutil.NewArrayString("syslog.listenAddr.tcp", "Comma-separated list of TCP addresses to listen to for Syslog messages. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/")
	listenAddrUDP = flagutil.NewArrayString("syslog.listenAddr.udp", "Comma-separated list of UDP address to listen to for Syslog messages. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/")

	tlsEnable = flagutil.NewArrayBool("syslog.tls", "Whether to enable TLS for receiving syslog messages at the corresponding -syslog.listenAddr.tcp. "+
		"The corresponding -syslog.tlsCertFile and -syslog.tlsKeyFile must be set if -syslog.tls is set. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#security")
	tlsCertFile = flagutil.NewArrayString("syslog.tlsCertFile", "Path to file with TLS certificate for the corresponding -syslog.listenAddr.tcp if the corresponding -syslog.tls is set. "+
		"Prefer ECDSA certs instead of RSA certs as RSA certs are slower. The provided certificate file is automatically re-read every second, so it can be dynamically updated. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#security")
	tlsKeyFile = flagutil.NewArrayString("syslog.tlsKeyFile", "Path to file with TLS key for the corresponding -syslog.listenAddr.tcp if the corresponding -syslog.tls is set. "+
		"The provided key file is automatically re-read every second, so it can be dynamically updated. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#security")
	tlsCipherSuites = flagutil.NewArrayString("syslog.tlsCipherSuites", "Optional list of TLS cipher suites for -syslog.listenAddr.tcp if -syslog.tls is set. "+
		"See the list of supported cipher suites at https://pkg.go.dev/crypto/tls#pkg-constants . "+
		"See also https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#security")
	tlsMinVersion = flag.String("syslog.tlsMinVersion", "TLS13", "The minimum TLS version to use for -syslog.listenAddr.tcp if -syslog.tls is set. "+
		"Supported values: TLS10, TLS11, TLS12, TLS13. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#security")

	compressMethodTCP = flagutil.NewArrayString("syslog.compressMethod.tcp", "Compression method for syslog messages received at the corresponding -syslog.listenAddr.tcp. "+
		"Supported values: none, gzip, deflate. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#compression")
	compressMethodUDP = flagutil.NewArrayString("syslog.compressMethod.udp", "Compression method for syslog messages received at the corresponding -syslog.listenAddr.udp. "+
		"Supported values: none, gzip, deflate. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#compression")

	useLocalTimestampTCP = flagutil.NewArrayBool("syslog.useLocalTimestamp.tcp", "Whether to use local timestamp instead of the original timestamp for the ingested syslog messages "+
		"at the corresponding -syslog.listenAddr.tcp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#log-timestamps")
	useLocalTimestampUDP = flagutil.NewArrayBool("syslog.useLocalTimestamp.udp", "Whether to use local timestamp instead of the original timestamp for the ingested syslog messages "+
		"at the corresponding -syslog.listenAddr.udp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#log-timestamps")
)

// MustInit initializes syslog parser at the given -syslog.listenAddr.tcp and -syslog.listenAddr.udp ports
//
// This function must be called after flag.Parse().
//
// MustStop() must be called in order to free up resources occupied by the initialized syslog parser.
func MustInit() {
	if workersStopCh != nil {
		logger.Panicf("BUG: MustInit() called twice without MustStop() call")
	}
	workersStopCh = make(chan struct{})

	for argIdx, addr := range *listenAddrTCP {
		workersWG.Add(1)
		go func(addr string, argIdx int) {
			runTCPListener(addr, argIdx)
			workersWG.Done()
		}(addr, argIdx)
	}

	for argIdx, addr := range *listenAddrUDP {
		workersWG.Add(1)
		go func(addr string, argIdx int) {
			runUDPListener(addr, argIdx)
			workersWG.Done()
		}(addr, argIdx)
	}

	currentYear := time.Now().Year()
	globalCurrentYear.Store(int64(currentYear))
	workersWG.Add(1)
	go func() {
		ticker := time.NewTicker(time.Minute)
		for {
			select {
			case <-workersStopCh:
				ticker.Stop()
				workersWG.Done()
				return
			case <-ticker.C:
				currentYear := time.Now().Year()
				globalCurrentYear.Store(int64(currentYear))
			}
		}
	}()

	if *syslogTimezone != "" {
		tz, err := time.LoadLocation(*syslogTimezone)
		if err != nil {
			logger.Fatalf("cannot parse -syslog.timezone=%q: %s", *syslogTimezone, err)
		}
		globalTimezone = tz
	} else {
		globalTimezone = time.Local
	}
}

var (
	globalCurrentYear atomic.Int64
	globalTimezone    *time.Location
)

var (
	workersWG     sync.WaitGroup
	workersStopCh chan struct{}
)

// MustStop stops syslog parser initialized via MustInit()
func MustStop() {
	close(workersStopCh)
	workersWG.Wait()
	workersStopCh = nil
}

func runUDPListener(addr string, argIdx int) {
	ln, err := net.ListenPacket(netutil.GetUDPNetwork(), addr)
	if err != nil {
		logger.Fatalf("cannot start UDP syslog server at %q: %s", addr, err)
	}

	tenantIDStr := tenantIDUDP.GetOptionalArg(argIdx)
	tenantID, err := logstorage.ParseTenantID(tenantIDStr)
	if err != nil {
		logger.Fatalf("cannot parse -syslog.tenantID.udp=%q for -syslog.listenAddr.udp=%q: %s", tenantIDStr, addr, err)
	}

	compressMethod := compressMethodUDP.GetOptionalArg(argIdx)
	checkCompressMethod(compressMethod, addr, "udp")

	useLocalTimestamp := useLocalTimestampUDP.GetOptionalArg(argIdx)

	streamFieldsStr := streamFieldsUDP.GetOptionalArg(argIdx)
	streamFields, err := parseFieldsList(streamFieldsStr)
	if err != nil {
		logger.Fatalf("cannot parse -syslog.streamFields.udp=%q for -syslog.listenAddr.udp=%q: %s", streamFieldsStr, addr, err)
	}

	ignoreFieldsStr := ignoreFieldsUDP.GetOptionalArg(argIdx)
	ignoreFields, err := parseFieldsList(ignoreFieldsStr)
	if err != nil {
		logger.Fatalf("cannot parse -syslog.ignoreFields.udp=%q for -syslog.listenAddr.udp=%q: %s", ignoreFieldsStr, addr, err)
	}

	decolorizeFieldsStr := decolorizeFieldsUDP.GetOptionalArg(argIdx)
	decolorizeFields, err := parseFieldsList(decolorizeFieldsStr)
	if err != nil {
		logger.Fatalf("cannot parse -syslog.decolorizeFields.udp=%q for -syslog.listenAddr.udp=%q: %s", decolorizeFieldsStr, addr, err)
	}

	extraFieldsStr := extraFieldsUDP.GetOptionalArg(argIdx)
	extraFields, err := parseExtraFields(extraFieldsStr)
	if err != nil {
		logger.Fatalf("cannot parse -syslog.extraFields.udp=%q for -syslog.listenAddr.udp=%q: %s", extraFieldsStr, addr, err)
	}

	doneCh := make(chan struct{})
	go func() {
		serveUDP(ln, tenantID, compressMethod, useLocalTimestamp, streamFields, ignoreFields, decolorizeFields, extraFields)
		close(doneCh)
	}()

	logger.Infof("started accepting syslog messages at -syslog.listenAddr.udp=%q", addr)
	<-workersStopCh
	if err := ln.Close(); err != nil {
		logger.Fatalf("syslog: cannot close UDP listener at %s: %s", addr, err)
	}
	<-doneCh
	logger.Infof("finished accepting syslog messages at -syslog.listenAddr.udp=%q", addr)
}

func runTCPListener(addr string, argIdx int) {
	var tlsConfig *tls.Config
	if tlsEnable.GetOptionalArg(argIdx) {
		certFile := tlsCertFile.GetOptionalArg(argIdx)
		keyFile := tlsKeyFile.GetOptionalArg(argIdx)
		tc, err := netutil.GetServerTLSConfig(certFile, keyFile, *tlsMinVersion, *tlsCipherSuites)
		if err != nil {
			logger.Fatalf("cannot load TLS cert from -syslog.tlsCertFile=%q, -syslog.tlsKeyFile=%q, -syslog.tlsMinVersion=%q, -syslog.tlsCipherSuites=%q: %s",
				certFile, keyFile, *tlsMinVersion, *tlsCipherSuites, err)
		}
		tlsConfig = tc
	}
	ln, err := netutil.NewTCPListener("syslog", addr, false, tlsConfig)
	if err != nil {
		logger.Fatalf("syslog: cannot start TCP listener at %s: %s", addr, err)
	}

	tenantIDStr := tenantIDTCP.GetOptionalArg(argIdx)
	tenantID, err := logstorage.ParseTenantID(tenantIDStr)
	if err != nil {
		logger.Fatalf("cannot parse -syslog.tenantID.tcp=%q for -syslog.listenAddr.tcp=%q: %s", tenantIDStr, addr, err)
	}

	compressMethod := compressMethodTCP.GetOptionalArg(argIdx)
	checkCompressMethod(compressMethod, addr, "tcp")

	useLocalTimestamp := useLocalTimestampTCP.GetOptionalArg(argIdx)

	streamFieldsStr := streamFieldsTCP.GetOptionalArg(argIdx)
	streamFields, err := parseFieldsList(streamFieldsStr)
	if err != nil {
		logger.Fatalf("cannot parse -syslog.streamFields.tcp=%q for -syslog.listenAddr.tcp=%q: %s", streamFieldsStr, addr, err)
	}

	ignoreFieldsStr := ignoreFieldsTCP.GetOptionalArg(argIdx)
	ignoreFields, err := parseFieldsList(ignoreFieldsStr)
	if err != nil {
		logger.Fatalf("cannot parse -syslog.ignoreFields.tcp=%q for -syslog.listenAddr.tcp=%q: %s", ignoreFieldsStr, addr, err)
	}

	decolorizeFieldsStr := decolorizeFieldsTCP.GetOptionalArg(argIdx)
	decolorizeFields, err := parseFieldsList(decolorizeFieldsStr)
	if err != nil {
		logger.Fatalf("cannot parse -syslog.decolorizeFields.tcp=%q for -syslog.listenAddr.tcp=%q: %s", decolorizeFieldsStr, addr, err)
	}

	extraFieldsStr := extraFieldsTCP.GetOptionalArg(argIdx)
	extraFields, err := parseExtraFields(extraFieldsStr)
	if err != nil {
		logger.Fatalf("cannot parse -syslog.extraFields.tcp=%q for -syslog.listenAddr.tcp=%q: %s", extraFieldsStr, addr, err)
	}

	doneCh := make(chan struct{})
	go func() {
		serveTCP(ln, tenantID, compressMethod, useLocalTimestamp, streamFields, ignoreFields, decolorizeFields, extraFields)
		close(doneCh)
	}()

	logger.Infof("started accepting syslog messages at -syslog.listenAddr.tcp=%q", addr)
	<-workersStopCh
	if err := ln.Close(); err != nil {
		logger.Fatalf("syslog: cannot close TCP listener at %s: %s", addr, err)
	}
	<-doneCh
	logger.Infof("finished accepting syslog messages at -syslog.listenAddr.tcp=%q", addr)
}

func checkCompressMethod(compressMethod, addr, protocol string) {
	switch compressMethod {
	case "", "none", "zstd", "gzip", "deflate":
		return
	default:
		logger.Fatalf("unsupported -syslog.compressMethod.%s=%q for -syslog.listenAddr.%s=%q; supported values: 'none', 'zstd', 'gzip', 'deflate'", protocol, compressMethod, protocol, addr)
	}
}

func serveUDP(ln net.PacketConn, tenantID logstorage.TenantID, encoding string, useLocalTimestamp bool, streamFields, ignoreFields, decolorizeFields []string, extraFields []logstorage.Field) {
	gomaxprocs := cgroup.AvailableCPUs()
	var wg sync.WaitGroup
	localAddr := ln.LocalAddr()
	for i := 0; i < gomaxprocs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cp := insertutil.GetCommonParamsForSyslog(tenantID, streamFields, ignoreFields, decolorizeFields, extraFields)
			var bb bytesutil.ByteBuffer
			bb.B = bytesutil.ResizeNoCopyNoOverallocate(bb.B, 64*1024)
			for {
				bb.Reset()
				bb.B = bb.B[:cap(bb.B)]
				n, remoteAddr, err := ln.ReadFrom(bb.B)
				if err != nil {
					udpErrorsTotal.Inc()
					var ne net.Error
					if errors.As(err, &ne) {
						if ne.Temporary() {
							logger.Errorf("syslog: temporary error when listening for UDP at %q: %s", localAddr, err)
							time.Sleep(time.Second)
							continue
						}
						if strings.Contains(err.Error(), "use of closed network connection") {
							break
						}
					}
					logger.Errorf("syslog: cannot read UDP data from %s at %s: %s", remoteAddr, localAddr, err)
					continue
				}
				bb.B = bb.B[:n]
				udpRequestsTotal.Inc()
				if err := processStream("udp", bb.NewReader(), encoding, useLocalTimestamp, cp); err != nil {
					logger.Errorf("syslog: cannot process UDP data from %s at %s: %s", remoteAddr, localAddr, err)
				}
			}
		}()
	}
	wg.Wait()
}

func serveTCP(ln net.Listener, tenantID logstorage.TenantID, encoding string, useLocalTimestamp bool, streamFields, ignoreFields, decolorizeFields []string, extraFields []logstorage.Field) {
	var cm ingestserver.ConnsMap
	cm.Init("syslog")

	var wg sync.WaitGroup
	addr := ln.Addr()
	for {
		c, err := ln.Accept()
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) {
				if ne.Temporary() {
					logger.Errorf("syslog: temporary error when listening for TCP addr %q: %s", addr, err)
					time.Sleep(time.Second)
					continue
				}
				if strings.Contains(err.Error(), "use of closed network connection") {
					break
				}
				logger.Fatalf("syslog: unrecoverable error when accepting TCP connections at %q: %s", addr, err)
			}
			logger.Fatalf("syslog: unexpected error when accepting TCP connections at %q: %s", addr, err)
		}
		if !cm.Add(c) {
			_ = c.Close()
			break
		}

		wg.Add(1)
		go func() {
			cp := insertutil.GetCommonParamsForSyslog(tenantID, streamFields, ignoreFields, decolorizeFields, extraFields)
			if err := processStream("tcp", c, encoding, useLocalTimestamp, cp); err != nil {
				logger.Errorf("syslog: cannot process TCP data at %q: %s", addr, err)
			}

			cm.Delete(c)
			_ = c.Close()
			wg.Done()
		}()
	}

	cm.CloseAll(0)
	wg.Wait()
}

// processStream parses a stream of syslog messages from r and ingests them into vlstorage.
func processStream(protocol string, r io.Reader, encoding string, useLocalTimestamp bool, cp *insertutil.CommonParams) error {
	if err := insertutil.CanWriteData(); err != nil {
		return err
	}

	lmp := cp.NewLogMessageProcessor("syslog_"+protocol, true)
	err := processStreamInternal(r, encoding, useLocalTimestamp, lmp)
	lmp.MustClose()

	return err
}

func processStreamInternal(r io.Reader, encoding string, useLocalTimestamp bool, lmp insertutil.LogMessageProcessor) error {
	reader, err := protoparserutil.GetUncompressedReader(r, encoding)
	if err != nil {
		return fmt.Errorf("cannot decode syslog data: %w", err)
	}
	defer protoparserutil.PutUncompressedReader(reader)

	return processUncompressedStream(reader, useLocalTimestamp, lmp)
}

func processUncompressedStream(r io.Reader, useLocalTimestamp bool, lmp insertutil.LogMessageProcessor) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	slr := getSyslogLineReader(wcr)
	defer putSyslogLineReader(slr)

	n := 0
	for {
		ok := slr.nextLine()
		wcr.DecConcurrency()
		if !ok {
			break
		}

		currentYear := int(globalCurrentYear.Load())
		err := processLine(slr.line, currentYear, globalTimezone, useLocalTimestamp, lmp)
		if err != nil {
			errorsTotal.Inc()
			return fmt.Errorf("cannot read line #%d: %s", n, err)
		}
		n++
	}
	return slr.Error()
}

type syslogLineReader struct {
	line []byte

	br  *bufio.Reader
	err error
}

func (slr *syslogLineReader) reset(r io.Reader) {
	slr.line = slr.line[:0]
	slr.br.Reset(r)
	slr.err = nil
}

// Error returns the last error occurred in slr.
func (slr *syslogLineReader) Error() error {
	if slr.err == nil || slr.err == io.EOF {
		return nil
	}
	return slr.err
}

// nextLine reads the next syslog line from slr and stores it at slr.line.
//
// false is returned if the next line cannot be read. Error() must be called in this case
// in order to verify whether there is an error or just slr stream has been finished.
func (slr *syslogLineReader) nextLine() bool {
	if slr.err != nil {
		return false
	}

again:
	prefix, err := slr.br.ReadSlice(' ')
	if err != nil {
		if err != io.EOF {
			slr.err = fmt.Errorf("cannot read message frame prefix: %w", err)
			return false
		}
		if len(prefix) == 0 {
			slr.err = err
			return false
		}
	}
	// skip empty lines
	for len(prefix) > 0 && prefix[0] == '\n' {
		prefix = prefix[1:]
	}
	if len(prefix) == 0 {
		// An empty prefix or a prefix with empty lines - try reading yet another prefix.
		goto again
	}

	if prefix[0] >= '0' && prefix[0] <= '9' {
		// This is octet-counting method. See https://www.ietf.org/archive/id/draft-gerhards-syslog-plain-tcp-07.html#msgxfer
		msgLenStr := bytesutil.ToUnsafeString(prefix[:len(prefix)-1])
		msgLen, err := strconv.ParseUint(msgLenStr, 10, 64)
		if err != nil {
			slr.err = fmt.Errorf("cannot parse message length from %q: %w", msgLenStr, err)
			return false
		}
		if maxMsgLen := insertutil.MaxLineSizeBytes.IntN(); msgLen > uint64(maxMsgLen) {
			slr.err = fmt.Errorf("cannot read message longer than %d bytes; msgLen=%d", maxMsgLen, msgLen)
			return false
		}
		slr.line = slicesutil.SetLength(slr.line, int(msgLen))
		if _, err := io.ReadFull(slr.br, slr.line); err != nil {
			slr.err = fmt.Errorf("cannot read message with size %d bytes: %w", msgLen, err)
			return false
		}
		return true
	}

	// This is octet-stuffing method. See https://www.ietf.org/archive/id/draft-gerhards-syslog-plain-tcp-07.html#octet-stuffing-legacy
	slr.line = append(slr.line[:0], prefix...)
	for {
		line, err := slr.br.ReadSlice('\n')
		if err == nil {
			slr.line = append(slr.line, line[:len(line)-1]...)
			return true
		}
		if err == io.EOF {
			slr.line = append(slr.line, line...)
			return true
		}
		if err == bufio.ErrBufferFull {
			slr.line = append(slr.line, line...)
			continue
		}
		slr.err = fmt.Errorf("cannot read message in octet-stuffing method: %w", err)
		return false
	}
}

func getSyslogLineReader(r io.Reader) *syslogLineReader {
	v := syslogLineReaderPool.Get()
	if v == nil {
		br := bufio.NewReaderSize(r, 64*1024)
		return &syslogLineReader{
			br: br,
		}
	}
	slr := v.(*syslogLineReader)
	slr.reset(r)
	return slr
}

func putSyslogLineReader(slr *syslogLineReader) {
	syslogLineReaderPool.Put(slr)
}

var syslogLineReaderPool sync.Pool

func processLine(line []byte, currentYear int, timezone *time.Location, useLocalTimestamp bool, lmp insertutil.LogMessageProcessor) error {
	p := logstorage.GetSyslogParser(currentYear, timezone)
	lineStr := bytesutil.ToUnsafeString(line)
	p.Parse(lineStr)

	var ts int64
	if useLocalTimestamp {
		ts = time.Now().UnixNano()
	} else {
		nsecs, err := insertutil.ExtractTimestampFromFields(timeFields, p.Fields)
		if err != nil {
			return fmt.Errorf("cannot get timestamp from syslog line %q: %w", line, err)
		}
		ts = nsecs
	}
	logstorage.RenameField(p.Fields, msgFields, "_msg")
	lmp.AddRow(ts, p.Fields, nil)
	logstorage.PutSyslogParser(p)

	return nil
}

var timeFields = []string{"timestamp"}
var msgFields = []string{"message"}

var (
	errorsTotal = metrics.NewCounter(`vl_errors_total{type="syslog"}`)

	udpRequestsTotal = metrics.NewCounter(`vl_udp_reqests_total{type="syslog"}`)
	udpErrorsTotal   = metrics.NewCounter(`vl_udp_errors_total{type="syslog"}`)
)

func parseFieldsList(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}

	var a []string
	err := json.Unmarshal([]byte(s), &a)
	return a, err
}

func parseExtraFields(s string) ([]logstorage.Field, error) {
	if s == "" {
		return nil, nil
	}

	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	fields := make([]logstorage.Field, 0, len(m))
	for k, v := range m {
		fields = append(fields, logstorage.Field{
			Name:  k,
			Value: v,
		})
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})
	return fields, nil
}
