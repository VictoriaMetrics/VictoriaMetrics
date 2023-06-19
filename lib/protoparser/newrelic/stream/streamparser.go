package stream

import (
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/newrelic"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

func Parse(r io.Reader, contentEncoding string, callback func(series []newrelic.Metric) error) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)
	r = wcr

	if contentEncoding == "gzip" {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped Newrelic agent data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	ctx := getPushCtx(r)
	defer putPushCtx(ctx)
	if err := ctx.Read(); err != nil {
		return err
	}

	p := newrelic.GetJSONParser()
	defer newrelic.PutJSONParser(p)

	v, err := p.ParseBytes(ctx.reqBuf.B)
	if err != nil {
		return fmt.Errorf("cannot unmarshal NewRelic POST request with size %d bytes: %s", len(ctx.reqBuf.B), err)
	}

	metricsPost, err := v.Array()
	if err != nil {
		return fmt.Errorf("cannot get Newrelic post data: %s", err)
	}

	var events newrelic.Events

	if err := events.Unmarshal(metricsPost); err != nil {
		return fmt.Errorf("cannot unmarshal NewRelic POST request with size %d bytes: %s", len(ctx.reqBuf.B), err)
	}

	if err := callback(events.Metrics); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}
