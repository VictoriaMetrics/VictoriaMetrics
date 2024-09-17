package syslog

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	
	"github.com/VictoriaMetrics/metrics"
)


type config struct {
	syslogConfig syslogConfig `json:"syslog"`
	queueConfig queueConfig   `json:"queue_config"`
}

type queueConfig struct {
	capacity int64 `json:"capacity,omitempty"`
	retries  int64 `json:"retries,omitempty"`
}

type syslogConfig struct {
	remoteHost  string    `json:"host"`
	port        int64     `json:"port,omitempty"`
	hostname    string    `json:"hostname,omitempty"`
	protocol    string    `json:"protocol,omitempty"`
	rfcNum      int64     `json:"rfcNum,omitempty"`
	facility    int64     `json:"facility,omitempty"`
	basicAuth   basicAuth `json:"basic_auth,omitempty"`
	tls         tlsConfig `json:"tls,omitempty"`
}

type basicAuth struct {
	username string `json:"username"`
	password string `json:"password"`
}

type tlsConfig struct {
	ca                 string `json:"ca_file,omitempty"`
	certFile           string `json:"cert_file,omitempty"`
	keyfile            string `json:"key_file,omitempty"`
	serverName         string `json:"server_name,omitempty"`
	insecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
}


// Log data to be written into the buffered channel and sent to the syslog server
type SyslogLogContent struct {
	Msg string
	LogLevel string
}

const (
	DEF_QUEUE_SIZE_VALUE           = 1000
	DEF_INSECURE_SKIP_VERIFY_VALUE = false
	DEF_RFC_NUM_VALUE              = 3164
	DEF_SYSLOG_SERVER_PORT         = 514
	DEF_SYSLOG_FACILITY            = 16
	DEF_SYSLOG_PROTOCOL            = "udp"
)

var (
	cfgFile = flag.String("syslogConfig", "", "Configuration file for syslog")
	logChan      chan SyslogLogContent
)


//Initializes the buffered channel and the syslog writer

func Init() {
	sysCfg := &config{}

	if *cfgFile != "" {
		cfgData, err := os.ReadFile(*cfgFile)
		if err != nil {
			panic(err)
		}

		err = yaml.Unmarshal(cfgData, &sysCfg)
		if err != nil {
			panic(err)
		}
		if sysCfg.queueConfig.capacity == 0 {
			sysCfg.queueConfig.capacity = DEF_QUEUE_SIZE_VALUE
		}

		if sysCfg.syslogConfig.port == 0 {
			sysCfg.syslogConfig.port = DEF_SYSLOG_SERVER_PORT
		}

		if sysCfg.syslogConfig.protocol == "" {
			sysCfg.syslogConfig.protocol = DEF_SYSLOG_PROTOCOL
		}
		if sysCfg.syslogConfig.facility == 0 {
			sysCfg.syslogConfig.facility = DEF_SYSLOG_FACILITY
		}
	} else {
		panic(fmt.Errorf("Syslog is configured but configuration file missing"))
	}

	// Initializes the buffered channel and the syslog writer 
	logChan = make(chan SyslogLogContent, sysCfg.queueConfig.capacity)
	syslogW := &syslogWriter{framer: defaultFramer, formatter: defaultFormatter}
	syslogW.sysCfg = sysCfg
	
	// watches the channel for log data written by the logger and forwards it to the syslog server
	go syslogW.logSender();
}

//Writes the log data to buffered channel. If the channel is full the oldest log data is dropped 
func WriteInfo(s SyslogLogContent) (error) {
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

	return nil
}