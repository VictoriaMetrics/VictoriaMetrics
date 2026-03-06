package zookeeper

import (
	"fmt"
	"time"

	"github.com/go-zookeeper/zk"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

type apiConfig struct {
	conn    *zk.Conn
	servers []string
	timeout time.Duration
}

var configMap = discoveryutil.NewConfigMap()

func getAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	timeout := 10 * time.Second
	if sdc.Timeout != nil {
		timeout = *sdc.Timeout
	}
	conn, _, err := zk.Connect(sdc.Servers, timeout, zk.WithLogger(&zkLogger{}))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to ZooKeeper servers %v: %w", sdc.Servers, err)
	}
	cfg := &apiConfig{
		conn:    conn,
		servers: sdc.Servers,
		timeout: timeout,
	}
	return cfg, nil
}

func (cfg *apiConfig) mustStop() {
	cfg.conn.Close()
}

// zkLogger adapts ZooKeeper client logging to VictoriaMetrics logger.
type zkLogger struct{}

func (l *zkLogger) Printf(format string, args ...any) {
	logger.Infof("zookeeper: "+format, args...)
}
