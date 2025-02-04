package remote_read_integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/util/annotations"
)

const (
	maxBytesInFrame = 1024 * 1024
)

type RemoteReadServer struct {
	server  *httptest.Server
	series  []*prompb.TimeSeries
	storage *MockStorage
}

// NewRemoteReadServer creates a remote read server. It exposes a single endpoint and responds with the
// passed series based on the request to the read endpoint. It returns a server which should be closed after
// being used.
func NewRemoteReadServer(t *testing.T) *RemoteReadServer {
	rrs := &RemoteReadServer{
		series: make([]*prompb.TimeSeries, 0),
	}
	rrs.server = httptest.NewServer(rrs.getReadHandler(t))
	return rrs
}

// Close closes the server.
func (rrs *RemoteReadServer) Close() {
	rrs.server.Close()
}

func (rrs *RemoteReadServer) URL() string {
	return rrs.server.URL
}

func (rrs *RemoteReadServer) SetRemoteReadSeries(series []*prompb.TimeSeries) {
	rrs.series = append(rrs.series, series...)
}

func (rrs *RemoteReadServer) getReadHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !validateReadHeaders(t, r) {
			t.Fatalf("invalid read headers")
		}

		compressed, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("error read body: %s", err)
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			t.Fatalf("error decode compressed data:%s", err)
		}

		var req prompb.ReadRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			t.Fatalf("error unmarshal read request: %s", err)
		}

		resp := &prompb.ReadResponse{
			Results: make([]*prompb.QueryResult, len(req.Queries)),
		}

		for i, r := range req.Queries {
			startTs := r.StartTimestampMs
			endTs := r.EndTimestampMs
			ts := make([]*prompb.TimeSeries, len(rrs.series))
			for i, s := range rrs.series {
				var samples []prompb.Sample
				for _, sample := range s.Samples {
					if sample.Timestamp >= startTs && sample.Timestamp < endTs {
						samples = append(samples, sample)
					}
				}
				var series prompb.TimeSeries
				if len(samples) > 0 {
					series.Labels = s.Labels
					series.Samples = samples
				}
				ts[i] = &series
			}

			resp.Results[i] = &prompb.QueryResult{Timeseries: ts}
			data, err := proto.Marshal(resp)
			if err != nil {
				t.Fatalf("error marshal response: %s", err)
			}

			compressed = snappy.Encode(nil, data)

			w.Header().Set("Content-Type", "application/x-protobuf")
			w.Header().Set("Content-Encoding", "snappy")
			w.WriteHeader(http.StatusOK)

			if _, err := w.Write(compressed); err != nil {
				t.Fatalf("snappy encode error: %s", err)
			}
		}
	})
}

func NewRemoteReadStreamServer(t *testing.T) *RemoteReadServer {
	rrs := &RemoteReadServer{
		series: make([]*prompb.TimeSeries, 0),
	}
	rrs.server = httptest.NewServer(rrs.getStreamReadHandler(t))
	return rrs
}

func (rrs *RemoteReadServer) InitMockStorage(series []*prompb.TimeSeries) {
	rrs.storage = NewMockStorage(series)
}

func (rrs *RemoteReadServer) getStreamReadHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !validateStreamReadHeaders(t, r) {
			t.Fatalf("invalid read headers")
		}

		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("internal http.ResponseWriter does not implement http.Flusher interface")
		}

		stream := remote.NewChunkedWriter(w, f)

		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("error read body: %s", err)
		}

		decodedData, err := snappy.Decode(nil, data)
		if err != nil {
			t.Fatalf("error decode compressed data:%s", err)
		}

		var req prompb.ReadRequest
		if err := proto.Unmarshal(decodedData, &req); err != nil {
			t.Fatalf("error unmarshal read request: %s", err)
		}

		var chks []prompb.Chunk
		ctx := context.Background()
		for idx, r := range req.Queries {
			startTs := r.StartTimestampMs
			endTs := r.EndTimestampMs

			var matchers []*labels.Matcher
			cb := func() (int64, error) { return 0, nil }

			c := remote.NewSampleAndChunkQueryableClient(rrs.storage, nil, matchers, true, cb)

			q, err := c.ChunkQuerier(startTs, endTs)
			if err != nil {
				t.Fatalf("error init chunk querier: %s", err)
			}

			ss := q.Select(ctx, false, nil, matchers...)
			var iter chunks.Iterator
			for ss.Next() {
				series := ss.At()
				iter = series.Iterator(iter)
				labels := remote.MergeLabels(labelsToLabelsProto(series.Labels()), nil)

				frameBytesLeft := maxBytesInFrame
				for _, lb := range labels {
					frameBytesLeft -= lb.Size()
				}

				isNext := iter.Next()

				for isNext {
					chunk := iter.At()

					if chunk.Chunk == nil {
						t.Fatalf("error found not populated chunk returned by SeriesSet at ref: %v", chunk.Ref)
					}

					chks = append(chks, prompb.Chunk{
						MinTimeMs: chunk.MinTime,
						MaxTimeMs: chunk.MaxTime,
						Type:      prompb.Chunk_Encoding(chunk.Chunk.Encoding()),
						Data:      chunk.Chunk.Bytes(),
					})

					frameBytesLeft -= chks[len(chks)-1].Size()

					// We are fine with minor inaccuracy of max bytes per frame. The inaccuracy will be max of full chunk size.
					isNext = iter.Next()
					if frameBytesLeft > 0 && isNext {
						continue
					}

					resp := &prompb.ChunkedReadResponse{
						ChunkedSeries: []*prompb.ChunkedSeries{
							{Labels: labels, Chunks: chks},
						},
						QueryIndex: int64(idx),
					}

					b, err := proto.Marshal(resp)
					if err != nil {
						t.Fatalf("error marshal response: %s", err)
					}

					if _, err := stream.Write(b); err != nil {
						t.Fatalf("error write to stream: %s", err)
					}
					chks = chks[:0]
					rrs.storage.Reset()
				}
				if err := iter.Err(); err != nil {
					t.Fatalf("error iterate over chunk series: %s", err)
				}
			}
		}
	})
}

func validateReadHeaders(t *testing.T, r *http.Request) bool {
	if r.Method != http.MethodPost {
		t.Fatalf("got %q method, expected %q", r.Method, http.MethodPost)
	}
	if r.Header.Get("Content-Encoding") != "snappy" {
		t.Fatalf("got %q content encoding header, expected %q", r.Header.Get("Content-Encoding"), "snappy")
	}
	if r.Header.Get("Content-Type") != "application/x-protobuf" {
		t.Fatalf("got %q content type header, expected %q", r.Header.Get("Content-Type"), "application/x-protobuf")
	}

	remoteReadVersion := r.Header.Get("X-Prometheus-Remote-Read-Version")
	if remoteReadVersion == "" {
		t.Fatalf("got empty prometheus remote read header")
	}
	if !strings.HasPrefix(remoteReadVersion, "0.1.") {
		t.Fatalf("wrong remote version defined")
	}

	return true
}

func validateStreamReadHeaders(t *testing.T, r *http.Request) bool {
	if r.Method != http.MethodPost {
		t.Fatalf("got %q method, expected %q", r.Method, http.MethodPost)
	}
	if r.Header.Get("Content-Encoding") != "snappy" {
		t.Fatalf("got %q content encoding header, expected %q", r.Header.Get("Content-Encoding"), "snappy")
	}
	if r.Header.Get("Content-Type") != "application/x-streamed-protobuf; proto=prometheus.ChunkedReadResponse" {
		t.Fatalf("got %q content type header, expected %q", r.Header.Get("Content-Type"), "application/x-streamed-protobuf; proto=prometheus.ChunkedReadResponse")
	}

	remoteReadVersion := r.Header.Get("X-Prometheus-Remote-Read-Version")
	if remoteReadVersion == "" {
		t.Fatalf("got empty prometheus remote read header")
	}
	if !strings.HasPrefix(remoteReadVersion, "0.1.") {
		t.Fatalf("wrong remote version defined")
	}
	return true
}

func GenerateRemoteReadSeries(start, end, numOfSeries, numOfSamples int64) []*prompb.TimeSeries {
	var ts []*prompb.TimeSeries
	j := 0
	for i := 0; i < int(numOfSeries); i++ {
		if i%3 == 0 {
			j++
		}

		timeSeries := prompb.TimeSeries{
			Labels: []prompb.Label{
				{Name: labels.MetricName, Value: fmt.Sprintf("vm_metric_%d", j)},
				{Name: "job", Value: strconv.Itoa(i)},
			},
		}

		ts = append(ts, &timeSeries)
	}

	for i := range ts {
		ts[i].Samples = generateRemoteReadSamples(i, start, end, numOfSamples)
	}

	return ts
}

func generateRemoteReadSamples(idx int, startTime, endTime, numOfSamples int64) []prompb.Sample {
	samples := make([]prompb.Sample, 0)
	delta := (endTime - startTime) / numOfSamples

	t := startTime
	for t != endTime {
		v := 100 * int64(idx)
		samples = append(samples, prompb.Sample{
			Timestamp: t * 1000,
			Value:     float64(v),
		})
		t = t + delta
	}

	return samples
}

type MockStorage struct {
	query *prompb.Query
	store []*prompb.TimeSeries
}

func NewMockStorage(series []*prompb.TimeSeries) *MockStorage {
	return &MockStorage{store: series}
}

func (ms *MockStorage) Read(_ context.Context, query *prompb.Query, sortSeries bool) (storage.SeriesSet, error) {
	if sortSeries {
		return nil, fmt.Errorf("unexpected sortSeries=true")
	}
	if ms.query != nil {
		return nil, fmt.Errorf("expected only one call to remote client got: %v", query)
	}
	ms.query = query

	tss := make([]*prompb.TimeSeries, 0, len(ms.store))
	for _, s := range ms.store {
		var samples []prompb.Sample
		for _, sample := range s.Samples {
			if sample.Timestamp >= query.StartTimestampMs && sample.Timestamp < query.EndTimestampMs {
				samples = append(samples, sample)
			}
		}
		var series prompb.TimeSeries
		if len(samples) > 0 {
			series.Labels = s.Labels
			series.Samples = samples
		}

		tss = append(tss, &series)
	}
	return &mockSeriesSet{
		tss: tss,
	}, nil
}

func (ms *MockStorage) Reset() {
	ms.query = nil
}

type mockSeriesSet struct {
	tss  []*prompb.TimeSeries
	next int
}

func (ss *mockSeriesSet) Next() bool {
	if ss.next >= len(ss.tss) {
		return false
	}
	ss.next++
	return true
}

func (ss *mockSeriesSet) At() storage.Series {
	return &mockSeries{
		s: ss.tss[ss.next-1],
	}
}

func (ss *mockSeriesSet) Err() error {
	return nil
}

func (ss *mockSeriesSet) Warnings() annotations.Annotations {
	return nil
}

type mockSeries struct {
	s *prompb.TimeSeries
}

func (s *mockSeries) Labels() labels.Labels {
	a := make(labels.Labels, len(s.s.Labels))
	for i, label := range s.s.Labels {
		a[i] = labels.Label{
			Name:  label.Name,
			Value: label.Value,
		}
	}
	return a
}

func (s *mockSeries) Iterator(chunkenc.Iterator) chunkenc.Iterator {
	return &mockSamplesIterator{
		samples: s.s.Samples,
	}
}

type mockSamplesIterator struct {
	samples []prompb.Sample
	next    int
}

func (si *mockSamplesIterator) Next() chunkenc.ValueType {
	if si.next >= len(si.samples) {
		return chunkenc.ValNone
	}
	si.next++
	return chunkenc.ValFloat
}

func (si *mockSamplesIterator) Seek(t int64) chunkenc.ValueType {
	for i := range si.samples {
		if si.samples[i].Timestamp >= t {
			si.next = i + 1
			return chunkenc.ValFloat
		}
	}
	return chunkenc.ValNone
}

func (si *mockSamplesIterator) At() (int64, float64) {
	s := si.samples[si.next-1]
	return s.Timestamp, s.Value
}

func (si *mockSamplesIterator) AtHistogram(*histogram.Histogram) (int64, *histogram.Histogram) {
	panic("BUG: musn't be called")
}

func (si *mockSamplesIterator) AtFloatHistogram(*histogram.FloatHistogram) (int64, *histogram.FloatHistogram) {
	panic("BUG: mustn't be called")
}

func (si *mockSamplesIterator) AtT() int64 {
	return si.samples[si.next-1].Timestamp
}

func (si *mockSamplesIterator) Err() error {
	return nil
}

func labelsToLabelsProto(labels labels.Labels) []prompb.Label {
	result := make([]prompb.Label, 0, len(labels))
	for _, l := range labels {
		result = append(result, prompb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	return result
}
