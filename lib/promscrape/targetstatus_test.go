package promscrape

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bloomfilter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

const (
	targetDropReasonTest = targetDropReason("test")
)

func TestDroppedTargetsRegister(t *testing.T) {
	var tmpDroppedTargetsMap = &droppedTargets{
		m:      make(map[uint64]droppedTarget),
		filter: bloomfilter.NewFilter(4000),
	}

	randSeed := rand.New(rand.NewSource(time.Now().Unix()))
	for i := 0; i < 10000; i++ {
		tmpDroppedTargetsMap.Register(&promutils.Labels{
			Labels: []prompbmarshal.Label{
				{
					Name:  strconv.Itoa(randSeed.Int()),
					Value: strconv.Itoa(randSeed.Int()),
				},
			},
		}, nil, targetDropReasonTest, nil)
	}

	if len(tmpDroppedTargetsMap.m) != 1000 {
		t.Fatalf("expected 1000 targets, got %d", len(tmpDroppedTargetsMap.m))
	}

	// over 98% coverage rate
	if tmpDroppedTargetsMap.totalTargets < 9800 {
		t.Fatalf("expected total targets higher than 9800, get %d", tmpDroppedTargetsMap.totalTargets)
	}
}
