package streamaggr

import (
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	// "time"
	// "github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

func TestPushSampleRace(t *testing.T) {
	as := maxAggrState{}
	for i := 0; i < 2; i++ {
		go func() {
			inputLabels := []prompbmarshal.Label{
				{
					Name:  "id",
					Value: "1",
					// Value:  fmt.Sprintf("%d", i),
				},
			}
			outputLabels := []prompbmarshal.Label{

				{
					Name:  "label",
					Value: "label",
				},
				{
					Name:  "label1",
					Value: "label1",
				},
				{
					Name:  "label2",
					Value: "label2",
				},
				{
					Name:  "label3",
					Value: "label3",
				},
				{
					Name:  "label4",
					Value: "label4",
				},
				{
					Name:  "label5",
					Value: "label5",
				},
				{
					Name:  "label6",
					Value: "label6",
				},
				{
					Name:  "label7",
					Value: "label7",
				},
				{
					Name:  "label8",
					Value: "label8",
				},
				{
					Name:  "label9",
					Value: "label9",
				},
				{
					Name:  "label11",
					Value: "label11",
				},
				{
					Name:  "label12",
					Value: "label12",
				},
			}
			buf := compressLabels([]byte{}, inputLabels, outputLabels)
			key := bytesutil.ToUnsafeString(buf)
			sample := []pushSample{
				{
					key:       key,
					value:     1,
					timestamp: 123,
				},
			}
			as.pushSamples(sample)
		}()
	}
	time.Sleep(1 * time.Second)
}

func TestPushSamples(t *testing.T) {
    for i := 0; i < 5; i++ {
        t.Run(fmt.Sprintf("Run%d", i+1), func(t *testing.T) {
            TestPushSampleRace(t)
        })
    }
}
