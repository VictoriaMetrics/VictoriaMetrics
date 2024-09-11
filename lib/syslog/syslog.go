package syslog

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	
	"github.com/VictoriaMetrics/metrics"
)

/*
The syslog config supplied via args is stored in SyslogConfig, QueueConfig & Syslog structs 
*/
type SyslogConfig struct {
	Syslog      Syslog      `json:"syslog"`
	QueueConfig QueueConfig `json:"queue_config"`
}

type QueueConfig struct {
	Capacity int64 `json:"capacity,omitempty"`
	Retries  int64 `json:"retries,omitempty"`
}

type Syslog struct {
	Host        string    `json:"host"`
	Port        int64     `json:"port,omitempty"`
	Protocol    string    `json:"protocol,omitempty"`
	RFCNum      int64     `json:"rfcNum,omitempty"`
	Facility    int64     `json:"facility,omitempty"`
	DefSeverity string    `json:"defSeverity,omitempty"`
	BasicAuth   BasicAuth `json:"basic_auth,omitempty"`
	TLS         TLS       `json:"tls,omitempty"`
}

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type TLS struct {
	Ca                 string `json:"ca_file,omitempty"`
	CertFile           string `json:"cert_file,omitempty"`
	Keyfile            string `json:"key_file,omitempty"`
	ServerName         string `json:"server_name,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
}

/*
Log data written into the buffered channel to be sent to the syslog server
*/
type SyslogLogInfo struct {
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
	sysCfg       SyslogConfig
	syslogConfig = flag.String("syslogConfig", "", "Configuration file for syslog")
	logChan      chan SyslogLogInfo
)

/*
Initializes the buffered channel and the syslog writer
*/
func Init() {
	if *syslogConfig != "" {
		cfgData, err := os.ReadFile(*syslogConfig)
		if err != nil {
			panic(err)
		}

		err = yaml.Unmarshal(cfgData, &sysCfg)
		if err != nil {
			panic(err)
		}
		if sysCfg.QueueConfig.Capacity == 0 {
			sysCfg.QueueConfig.Capacity = DEF_QUEUE_SIZE_VALUE
		}

		if sysCfg.Syslog.Port == 0 {
			sysCfg.Syslog.Port = DEF_SYSLOG_SERVER_PORT
		}

		if sysCfg.Syslog.Protocol == "" {
			sysCfg.Syslog.Protocol = DEF_SYSLOG_PROTOCOL
		}
	} else {
		panic(fmt.Errorf("Syslog is configured but configuration file missing"))
	}

	// Initializes the buffered channel and the syslog writer 
	logChan = make(chan SyslogLogInfo, sysCfg.QueueConfig.Capacity)
	syslogW := &syslogWriter{}
	
	// watches the channel for log data written by the logger and forwards it to the syslog server
	go syslogW.LogSender();
}

/*
Writes the log data to buffered channel. If the channel is full the oldest log data is dropped 
Input: Data for each log message
Output: error handling(TO_BE_DONE)
*/
func WriteInfo(s SyslogLogInfo) (error) {
	channelSize := "vm_syslog_queue_size"
	metrics.GetOrCreateGauge(channelSize, func() float64 {
		return float64(len(logChan))
	})

	totalDroppedMsgs := "vm_syslog_dropped_logs_total"

	select {
	case logChan <- s:
		//message sent
	default:
		drpMsg := <-logChan
		fmt.Println("Dropped Message:", drpMsg)
		metrics.GetOrCreateCounter(totalDroppedMsgs).Inc()
		logChan <- s
	}

	return nil
}