package newrelic

import (
	"testing"

	"github.com/valyala/fastjson"
)

func BenchmarkRequestUnmarshal(b *testing.B) {
	reqBody := `[
    {
      "EntityID":28257883748326179,
      "IsAgent":true,
      "Events":[
        {
          "eventType":"SystemSample",
          "timestamp":1690286061,
          "entityKey":"macbook-pro.local",
          "cpuPercent":25.056660790748904,
          "cpuUserPercent":8.687987912389374,
          "cpuSystemPercent":16.36867287835953,
          "cpuIOWaitPercent":0,
          "cpuIdlePercent":74.94333920925109,
          "cpuStealPercent":0,
          "loadAverageOneMinute":5.42333984375,
          "loadAverageFiveMinute":4.099609375,
          "loadAverageFifteenMinute":3.58203125,
          "memoryTotalBytes":17179869184,
          "memoryFreeBytes":3782705152,
          "memoryUsedBytes":13397164032,
          "memoryFreePercent":22.01824188232422,
          "memoryUsedPercent":77.98175811767578,
          "memoryCachedBytes":0,
          "memorySlabBytes":0,
          "memorySharedBytes":0,
          "memoryKernelFree":89587712,
          "swapTotalBytes":7516192768,
          "swapFreeBytes":1737293824,
          "swapUsedBytes":5778898944,
          "diskUsedBytes":0,
          "diskUsedPercent":0,
          "diskFreeBytes":0,
          "diskFreePercent":0,
          "diskTotalBytes":0,
          "diskUtilizationPercent":0,
          "diskReadUtilizationPercent":0,
          "diskWriteUtilizationPercent":0,
          "diskReadsPerSecond":0,
          "diskWritesPerSecond":0,
          "uptime":762376
        }
      ],
      "ReportingAgentID":28257883748326179
    }
  ]`
	b.SetBytes(int64(len(reqBody)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		value, err := fastjson.Parse(reqBody)
		if err != nil {
			b.Errorf("cannot parse json error: %s", err)
		}
		v, err := value.Array()
		if err != nil {
			b.Errorf("cannot get array from json")
		}
		for pb.Next() {
			e := &Events{Metrics: []Metric{}}
			if err := e.Unmarshal(v); err != nil {
				b.Errorf("Unmarshal() error = %v", err)
			}
			if len(e.Metrics) == 0 {
				b.Errorf("metrics should have at least one element")
			}
		}
	})
}
