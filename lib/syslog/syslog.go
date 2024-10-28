package syslog

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/VictoriaMetrics/metrics"
)

var (
	syslogSendWG     sync.WaitGroup
	syslogSendStopCh chan struct{}
)

type config struct {
	SyslogConfig syslogConfig `yaml:"syslog"`
	QueueConfig  queueConfig  `yaml:"queue_config"`
}

type queueConfig struct {
	Capacity      int64  `yaml:"capacity,omitempty"`
	Retries       int64  `yaml:"retries,omitempty"`
	RetryDuration string `yaml:"retryDuration,omitempty"`
}

type syslogConfig struct {
	RemoteHost string    `yaml:"host"`
	Port       int64     `yaml:"port,omitempty"`
	Hostname   string    `yaml:"hostname,omitempty"`
	Protocol   string    `yaml:"protocol,omitempty"`
	RfcNum     int64     `yaml:"rfcNum,omitempty"`
	Facility   int64     `yaml:"facility,omitempty"`
	BasicAuth  basicAuth `yaml:"basic_auth,omitempty"`
	TLS        tlsConfig `yaml:"tls,omitempty"`
}

type basicAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type tlsConfig struct {
	Ca                 string `yaml:"ca_file,omitempty"`
	CertFile           string `yaml:"cert_file,omitempty"`
	Keyfile            string `yaml:"key_file,omitempty"`
	ServerName         string `yaml:"server_name,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"`
}

// LogContent contains log data to be written into the buffered channel and sent to the syslog server and the log level
type LogContent struct {
	Msg      string
	LogLevel string
}

const (
	defaultQueueSizeValue   = 1000
	defaultSyslogServerPort = 514
	defaultSyslogFacility   = 16
	defaultSyslogProtocol   = "udp"
	defaultRetryDuration    = 2 * time.Second
)

var (
	cfgFile = flag.String("syslogConfig", "", "Configuration file for syslog")
	logChan chan LogContent
)

// Init initializes the buffered channel and the syslog writer
func Init() {
	sysCfg := &config{}

	if *cfgFile != "" {
		cfgData, err := os.ReadFile(*cfgFile)
		if err != nil {
			panic(err)
		}

		err = yaml.Unmarshal(cfgData, sysCfg)
		if err != nil {
			panic(err)
		}
		if sysCfg.QueueConfig.Capacity == 0 {
			sysCfg.QueueConfig.Capacity = defaultQueueSizeValue
		}

		if sysCfg.SyslogConfig.Port == 0 {
			sysCfg.SyslogConfig.Port = defaultSyslogServerPort
		}

		if sysCfg.SyslogConfig.Protocol == "" {
			sysCfg.SyslogConfig.Protocol = defaultSyslogProtocol
		}
		if sysCfg.SyslogConfig.Facility == 0 {
			sysCfg.SyslogConfig.Facility = defaultSyslogFacility
		}
	} else {
		panic(fmt.Errorf("Syslog is configured but configuration file missing"))
	}

	// Initializes the buffered channel and the syslog writer
	logChan = make(chan LogContent, sysCfg.QueueConfig.Capacity)
	syslogW := &syslogWriter{framer: defaultFramer, formatter: defaultFormatter}
	syslogW.sysCfg = sysCfg
	// watches the channel for log data written by the logger and forwards it to the syslog server
	syslogSendStopCh = make(chan struct{})
	syslogSendWG.Add(1)
	go func() {
		syslogW.logSender()
		syslogSendWG.Done()
	}()
}

// Stop flushes all the log messages present in the buffered channel before shutting down
func Stop() {
	close(syslogSendStopCh)
	syslogSendWG.Wait()
	syslogSendStopCh = nil
}

// WriteInfo writes the log data to buffered channel. If the channel is full the oldest log data is dropped
func WriteInfo(s LogContent) {
	channelSize := "vm_syslog_queue_size"
	metrics.GetOrCreateGauge(channelSize, func() float64 {
		return float64(len(logChan))
	})

	totalDroppedMsgs := "vm_syslog_dropped_logs_total"

	select {
	case logChan <- s:
	default:
		drpMsg := <-logChan
		fmt.Println("Dropped Message:", drpMsg)
		metrics.GetOrCreateCounter(totalDroppedMsgs).Inc()
		logChan <- s
	}
}
