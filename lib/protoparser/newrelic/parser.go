package newrelic

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/valyala/fastjson"
)

// MetricPost entity item for the HTTP post to be sent to the ingest service.
type MetricPost struct {
	ExternalKeys []string          `json:"ExternalKeys,omitempty"`
	EntityID     uint64            `json:"EntityID,omitempty"`
	IsAgent      bool              `json:"IsAgent"`
	Events       []json.RawMessage `json:"Events"`
	// Entity ID of the reporting agent, which will = EntityID when IsAgent == true.
	// The field is required in the backend for host metadata matching of the remote entities
	ReportingAgentID uint64 `json:"ReportingAgentID,omitempty"`
}

type Request struct {
	Metrics []MetricPost
	Series  []Series
}

type Series struct {
	Timestamp uint64
	Labels    map[string]string
	Name      string
	Value     float64
}

// BaseEvent type specifying properties for all sample events
// All fields on SampleEvent must be set before it is sent.
type BaseEvent struct {
	EventType string `json:"eventType"`
	Timestmp  int64  `json:"timestamp"`
	EntityKey string `json:"entityKey"`
}

func (r *Request) Unmarshal(b []byte) error {
	// req.reset()
	if err := json.Unmarshal(b, &r.Metrics); err != nil {
		return fmt.Errorf("cannot unmarshal %q: %w", b, err)
	}

	return nil
}

func Parse(r io.Reader, contentEncoding string, callback func(series []Series) error) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)
	r = wcr

	switch contentEncoding {
	case "gzip":
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped DataDog data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	case "deflate":
		zlr, err := common.GetZlibReader(r)
		if err != nil {
			return fmt.Errorf("cannot read deflated DataDog data: %w", err)
		}
		defer common.PutZlibReader(zlr)
		r = zlr
	}
	ctx := getPushCtx(r)
	defer putPushCtx(ctx)
	if err := ctx.Read(); err != nil {
		return err
	}
	req := getRequest()
	defer putRequest(req)
	if err := req.Unmarshal(ctx.reqBuf.B); err != nil {
		// unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal NewRelic POST request with size %d bytes: %s", len(ctx.reqBuf.B), err)
	}

	for _, metric := range req.Metrics {
		var p fastjson.Parser
		for _, event := range metric.Events {
			value, err := p.ParseBytes(event)
			if err != nil {
				return err
			}
			log.Printf("VALUE => %s", value.String())

			obj, err := value.Object()
			if err != nil {
				return err
			}
			obj.Visit(func(key []byte, v *fastjson.Value) {
				log.Printf("KEY => %s; value => %s", key, v.Type().String())
			})
		}
	}

	if err := callback(req.Series); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}

type pushCtx struct {
	br     *bufio.Reader
	reqBuf bytesutil.ByteBuffer
}

func (ctx *pushCtx) reset() {
	ctx.br.Reset(nil)
	ctx.reqBuf.Reset()
}

func getPushCtx(r io.Reader) *pushCtx {
	select {
	case ctx := <-pushCtxPoolCh:
		ctx.br.Reset(r)
		return ctx
	default:
		if v := pushCtxPool.Get(); v != nil {
			ctx := v.(*pushCtx)
			ctx.br.Reset(r)
			return ctx
		}
		return &pushCtx{
			br: bufio.NewReaderSize(r, 64*1024),
		}
	}
}

func putPushCtx(ctx *pushCtx) {
	ctx.reset()
	select {
	case pushCtxPoolCh <- ctx:
	default:
		pushCtxPool.Put(ctx)
	}
}

var pushCtxPool sync.Pool
var pushCtxPoolCh = make(chan *pushCtx, cgroup.AvailableCPUs())

func getRequest() *Request {
	v := requestPool.Get()
	if v == nil {
		return &Request{}
	}
	return v.(*Request)
}

func putRequest(req *Request) {
	requestPool.Put(req)
}

var requestPool sync.Pool

func (ctx *pushCtx) Read() error {
	// readCalls.Inc()
	lr := io.LimitReader(ctx.br, int64(64*1024*1024)+1)
	startTime := fasttime.UnixTimestamp()
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	if err != nil {
		// readErrors.Inc()
		return fmt.Errorf("cannot read compressed request in %d seconds: %w", fasttime.UnixTimestamp()-startTime, err)
	}
	if reqLen > int64(64*1024*1024) {
		// readErrors.Inc()
		return fmt.Errorf("too big packed request; mustn't exceed `-maxInsertRequestSize=%d` bytes", 64*1024*1024)
	}
	return nil
}
