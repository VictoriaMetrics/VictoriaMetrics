package prompb

import "github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"

func ToPromMarshal(spans []BucketSpan) []prompbmarshal.BucketSpan {
	var res []prompbmarshal.BucketSpan
	for _, span := range spans {
		res = append(res, prompbmarshal.BucketSpan{
			Offset: span.Offset,
			Length: span.Length,
		})
	}
	return res
}
