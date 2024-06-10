package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	logsPath     = flag.String("logsPath", "", "Path to logs directory")
	syslogAddr   = flag.String("syslog.addr", "logstash:12345", "Addr to send logs to")
	syslogAddr2  = flag.String("syslog.addr2", "logstash:12345", "Addr to send logs to")
	randomSuffix = flag.Bool("logs.randomSuffix", false, "Whether to add a random suffix to a log line")

	outputRateLimitItems  = flag.Int("outputRateLimitItems", 100, "Number of items to send per second")
	outputRateLimitPeriod = flag.Duration("outputRateLimitPeriod", time.Second, "Period of time to send items")
)

func main() {
	flag.Parse()
	startedAt := time.Now().Unix()

	logFiles, err := os.ReadDir(*logsPath)
	if err != nil {
		panic(fmt.Errorf("error reading directory %s:%w", *logsPath, err))
	}

	sourceFiles := make([]string, 0)

	for _, logFile := range logFiles {
		if strings.HasSuffix(logFile.Name(), ".log") {
			sourceFiles = append(sourceFiles, logFile.Name())
		}
	}
	log.Printf("sourceFiles: %v", sourceFiles)
	log.Printf("running with rate limit: %d items per %s", *outputRateLimitItems, *outputRateLimitPeriod)

	limitTicker := time.NewTicker(*outputRateLimitPeriod)
	limitItems := *outputRateLimitItems
	limitter := make(chan struct{}, limitItems)
	go func() {
		for {
			<-limitTicker.C
			for i := 0; i < limitItems; i++ {
				limitter <- struct{}{}
			}
		}
	}()

	for _, sourceFile := range sourceFiles {
		log.Printf("sourceFile: %s", sourceFile)
		f, err := os.Open(*logsPath + "/" + sourceFile)
		if err != nil {
			panic(err)
		}

		syslogTag := "logs-benchmark-" + sourceFile + "-" + strconv.FormatInt(startedAt, 10)

		// Loki uses RFC5424 syslog format, which has a 48 character limit on the tag.
		tagLen := len(syslogTag)
		if tagLen > 48 {
			truncate := tagLen - 48
			syslogTag = syslogTag[truncate:]
		}
		logger, err := syslog.Dial("tcp", *syslogAddr, syslog.LOG_INFO, syslogTag)
		if err != nil {
			panic(fmt.Errorf("error dialing syslog: %w", err))
		}
		logger2, err := syslog.Dial("tcp", *syslogAddr2, syslog.LOG_INFO, syslogTag)
		if err != nil {
			panic(fmt.Errorf("error dialing syslog: %w", err))
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			<-limitter
			line := scanner.Text()
			if *randomSuffix {
				line = line + " " + randomString()
			}
			_ = logger.Info(line)
			_ = logger2.Info(line)
		}

		logger.Close()
		logger2.Close()
	}

}

func randomString() string {
	buf := make([]byte, 4)
	ip := rand.Uint32()

	binary.LittleEndian.PutUint32(buf, ip)
	return net.IP(buf).String()
}
