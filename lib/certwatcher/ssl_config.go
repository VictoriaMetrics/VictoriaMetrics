package certwatcher

import (
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

const contentCheckInterval = 10 * time.Second

// Run watches for changes at given SSLConfig
// it triggers provided onUpdate function, if content of SSLConfig changes
func Run(sc *SSLConfig, stop chan struct{}, onUpdate func() error) {

	if !sc.isSet() {
		return
	}
	logger.Infof("start watching for ssl certificate changes for ca_file_path=%q, cert_file_path=%q, key_file_path=%q", sc.CAFilePath, sc.CertFilePath, sc.KeyFilePath)
	d := timeutil.AddJitterToDuration(contentCheckInterval)
	t := time.NewTicker(d)

	currentHash := sc.getConfigHash()

	for {
		select {
		case <-stop:
			return
		case <-t.C:
			newHash := sc.getConfigHash()
			if newHash == currentHash {
				continue
			}
			if err := onUpdate(); err != nil {
				logger.Errorf("error at certwatcher callback: %s", err)
				continue
			}
			currentHash = newHash
		}
	}
}

// SSLConfig defines ssl certificate paths
type SSLConfig struct {
	CAFilePath   string
	CertFilePath string
	KeyFilePath  string

	currentConfigHash atomic.Uint64
	checkDeadline     atomic.Int64
}

func (sc *SSLConfig) isSet() bool {
	return len(sc.CAFilePath) > 0 || len(sc.CertFilePath) > 0 || len(sc.KeyFilePath) > 0
}

func (sc *SSLConfig) getConfigHash() uint64 {
	currentConfigHash := sc.currentConfigHash.Load()
	ct := time.Now().UnixMilli()
	deadline := sc.checkDeadline.Load()

	if ct < deadline {
		return currentConfigHash
	}
	nextDeadline := ct + 5_000
	if !sc.checkDeadline.CompareAndSwap(deadline, nextDeadline) {
		return currentConfigHash
	}

	digest := xxhash.New()
	files := []struct {
		label string
		path  string
	}{
		{"ca", sc.CAFilePath},
		{"cert", sc.CertFilePath},
		{"key", sc.KeyFilePath},
	}
	for _, f := range files {
		if len(f.path) == 0 {
			continue
		}
		file, err := os.Open(f.path)
		if err != nil {
			logger.Errorf("cannot open ssl %s file at %q: %s", f.label, f.path, err)
			return currentConfigHash
		}
		_, err = io.Copy(digest, file)
		file.Close()
		if err != nil {
			logger.Errorf("cannot hash ssl %s file at %q: %s", f.label, f.path, err)
			return currentConfigHash
		}
	}
	newConfigHash := digest.Sum64()
	sc.currentConfigHash.Store(newConfigHash)
	return newConfigHash
}
