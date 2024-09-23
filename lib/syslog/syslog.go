package syslog

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
	
	"github.com/VictoriaMetrics/metrics"
)


type config struct {
	SyslogConfig syslogConfig `yaml:"syslog"`
	QueueConfig queueConfig   `yaml:"queue_config"`
}

type queueConfig struct {
	Capacity int64 `yaml:"capacity,omitempty"`
	Retries  int64 `yaml:"retries,omitempty"`
}

type syslogConfig struct {
	RemoteHost  string    `yaml:"host"`
	Port        int64     `yaml:"port,omitempty"`
	Hostname    string    `yaml:"hostname,omitempty"`
	Protocol    string    `yaml:"protocol,omitempty"`
	RfcNum      int64     `yaml:"rfcNum,omitempty"`
	Facility    int64     `yaml:"facility,omitempty"`
	BasicAuth   basicAuth `yaml:"basic_auth,omitempty"`
	Tls         tlsConfig `yaml:"tls,omitempty"`
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

		err = yaml.Unmarshal(cfgData, sysCfg)
		if err != nil {
			panic(err)
		}
		if sysCfg.QueueConfig.Capacity == 0 {
			sysCfg.QueueConfig.Capacity = DEF_QUEUE_SIZE_VALUE
		}

		if sysCfg.SyslogConfig.Port == 0 {
			sysCfg.SyslogConfig.Port = DEF_SYSLOG_SERVER_PORT
		}

		if sysCfg.SyslogConfig.Protocol == "" {
			sysCfg.SyslogConfig.Protocol = DEF_SYSLOG_PROTOCOL
		}
		if sysCfg.SyslogConfig.Facility == 0 {
			sysCfg.SyslogConfig.Facility = DEF_SYSLOG_FACILITY
		}
	} else {
		panic(fmt.Errorf("Syslog is configured but configuration file missing"))
	}

	// Initializes the buffered channel and the syslog writer 
	logChan = make(chan SyslogLogContent, sysCfg.QueueConfig.Capacity)
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