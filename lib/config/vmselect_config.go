package config

import (
	"context"
	"errors"

	//"errors"
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"gopkg.in/yaml.v3"
	"strings"
	"sync/atomic"
)

var (
	VMSelectConfigVar           = atomic.Pointer[VMSelectConfig]{}
	VMSelectTreatDotsAsIsLabels = atomic.Pointer[Labels]{}

	EqualBlockedTotal    = metrics.NewCounter(`vmselect_equal_query_blocked_total`)
	ContainsBlockedTotal = metrics.NewCounter(`vmselect_contains_query_blocked_total`)
)
var ErrBlockedQuery = errors.New("query is blocked! contact administator for more information")

type Labels map[string]struct{}

func (l Labels) Contains(label string) bool {
	_, ok := l[label]
	return ok
}

type VMSelectConfig struct {
	BlockedQueries      *BlockedQueries `yaml:"blockedQueries,omitempty"`
	TreatDotsAsIsLabels []string        `yaml:"treatDotsAsIsLabels,omitempty"`
}

type BlockedQueries struct {
	BlockIfEquals   []string `yaml:"blockIfEquals,omitempty"`
	BlockIfContains []string `yaml:"blockIfContains,omitempty"`
	LogIfEquals     []string `yaml:"logIfEquals,omitempty"`
	LogIfContains   []string `yaml:"logIfContains,omitempty"`
}

func InitVMSelectConfig() (context.CancelFunc, error) {
	return LoadConfig(func(data []byte) error {
		var c VMSelectConfig
		if err := yaml.Unmarshal(data, &c); err != nil {
			return fmt.Errorf("cannot unmarshal vmselect config: %w", err)
		}

		// 设置 VMSelectTreatDotsAsIsLabels
		if len(c.TreatDotsAsIsLabels) > 0 {
			treatDotsAsIsLabelsMaps := make(Labels, len(c.TreatDotsAsIsLabels))
			for _, labelName := range c.TreatDotsAsIsLabels {
				treatDotsAsIsLabelsMaps[labelName] = struct{}{}
			}
			VMSelectTreatDotsAsIsLabels.Store(&treatDotsAsIsLabelsMaps)
		}

		// 设置 VMSelectConfigVar
		VMSelectConfigVar.Store(&c)
		return nil
	})
}

func IsQueryBlocked(query string) bool {
	c := VMSelectConfigVar.Load()
	if c == nil {
		return false
	}
	if c.BlockedQueries != nil {
		blockQueryIfEquals := c.BlockedQueries.BlockIfEquals
		if len(blockQueryIfEquals) > 0 {
			for _, blockedQuery := range blockQueryIfEquals {
				if query == blockedQuery {
					EqualBlockedTotal.Inc()
					logger.Infof("blockedQueries: block query [equals]: %s", query)
					return true
				}
			}
		}
		blockQueryIfContains := c.BlockedQueries.BlockIfContains
		if len(blockQueryIfContains) > 0 {
			for _, blockedQuery := range blockQueryIfContains {
				if strings.Contains(query, blockedQuery) {
					ContainsBlockedTotal.Inc()
					logger.Infof("blockedQueries: block query [contains]: %s", query)
					return true
				}
			}
		}
		logIfEquals := c.BlockedQueries.LogIfEquals
		if len(logIfEquals) > 0 {
			for _, logQuery := range logIfEquals {
				if query == logQuery {
					logger.Infof("blockedQueries: log query [equals]: %s", query)
				}
			}
		}
		logIfContains := c.BlockedQueries.LogIfContains
		if len(logIfContains) > 0 {
			for _, logQuery := range logIfContains {
				if strings.Contains(query, logQuery) {
					logger.Infof("blockedQueries: log query [contains]: %s", query)
				}
			}
		}
	}
	return false
}
