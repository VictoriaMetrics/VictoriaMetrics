package kubernetes

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/easyproto"
)

func mustReadTestFile(tb testing.TB, name string) []byte {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		tb.Fatalf("cannot read test file %q: %s", name, err)
	}
	return data
}

func TestIsProtobufContentType(t *testing.T) {
	f := func(contentType string, resultExpected bool) {
		t.Helper()
		if result := isProtobufContentType(contentType); result != resultExpected {
			t.Fatalf("unexpected result for Content-Type %q; got %v; want %v", contentType, result, resultExpected)
		}
	}
	f("application/vnd.kubernetes.protobuf", true)
	f("application/vnd.kubernetes.protobuf;charset=utf-8", true)
	f("application/vnd.kubernetes.protobuf; charset=utf-8", true)
	f("application/json", false)
	f("application/json, application/vnd.kubernetes.protobuf", false)
	f("", false)
}

// TestParsePodListProtobuf verifies that parsePodListProtobuf returns objects
// equivalent to the objects returned by parsePodList for the same list.
func TestParsePodListProtobuf(t *testing.T) {
	pbData := mustReadTestFile(t, "pods_list.pb")
	jsonData := mustReadTestFile(t, "pods_list.json")

	objectsExpected, lmExpected, err := parsePodList(bytes.NewReader(jsonData))
	if err != nil {
		t.Fatalf("cannot parse JSON list: %s", err)
	}
	objectsGot, lmGot, err := parsePodListProtobuf(bytes.NewReader(pbData))
	if err != nil {
		t.Fatalf("cannot parse protobuf list: %s", err)
	}
	if lmGot.ResourceVersion != lmExpected.ResourceVersion {
		t.Fatalf("unexpected list resourceVersion; got %q; want %q", lmGot.ResourceVersion, lmExpected.ResourceVersion)
	}
	if len(objectsGot) != 3 {
		t.Fatalf("unexpected number of objects; got %d; want 3", len(objectsGot))
	}
	if !reflect.DeepEqual(objectsGot, objectsExpected) {
		t.Fatalf("unexpected objects;\ngot\n%v\nwant\n%v", objectsGot, objectsExpected)
	}

	// The parsed objects mustn't refer to the input buffer, since it may be re-used.
	clear(pbData)
	if !reflect.DeepEqual(objectsGot, objectsExpected) {
		t.Fatalf("the parsed objects refer to the input buffer")
	}
}

func TestParsePodListProtobufEmpty(t *testing.T) {
	pbData := mustReadTestFile(t, "pods_list_empty.pb")
	jsonData := mustReadTestFile(t, "pods_list_empty.json")

	objectsExpected, lmExpected, err := parsePodList(bytes.NewReader(jsonData))
	if err != nil {
		t.Fatalf("cannot parse JSON list: %s", err)
	}
	objectsGot, lmGot, err := parsePodListProtobuf(bytes.NewReader(pbData))
	if err != nil {
		t.Fatalf("cannot parse protobuf list: %s", err)
	}
	if len(objectsGot) != 0 {
		t.Fatalf("unexpected number of objects; got %d; want 0", len(objectsGot))
	}
	if lmGot.ResourceVersion != lmExpected.ResourceVersion {
		t.Fatalf("unexpected list resourceVersion; got %q; want %q", lmGot.ResourceVersion, lmExpected.ResourceVersion)
	}
	if !reflect.DeepEqual(objectsGot, objectsExpected) {
		t.Fatalf("unexpected objects;\ngot\n%v\nwant\n%v", objectsGot, objectsExpected)
	}
}

// TestParsePodListProtobufTruncated verifies that truncated protobuf list is never parsed into a partial result.
//
// This is important, since a partial list would delete the missing objects from the discovery cache.
func TestParsePodListProtobufTruncated(t *testing.T) {
	data := mustReadTestFile(t, "pods_list.pb")
	objectsFull, _, err := parsePodListProtobuf(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("cannot parse the full list: %s", err)
	}
	successCount := 0
	for i := 0; i < len(data); i++ {
		objects, _, err := parsePodListProtobuf(bytes.NewReader(data[:i]))
		if err != nil {
			continue
		}
		successCount++
		if !reflect.DeepEqual(objects, objectsFull) {
			t.Fatalf("partial list is returned for the prefix of %d bytes", i)
		}
	}
	// Only the last few bytes of the list encoding may be optional, so at most a couple of prefixes can succeed.
	if successCount > 4 {
		t.Fatalf("too many successfully parsed truncated lists: %d", successCount)
	}
}

func TestParsePodProtobufFailure(t *testing.T) {
	f := func(name string, data []byte) {
		t.Helper()
		if _, err := parsePodProtobuf(data); err == nil {
			t.Fatalf("expecting non-nil error for %s", name)
		}
	}
	f("nil", nil)
	f("garbage", []byte("garbage"))
	f("truncated unknown", []byte("k8s\x00\x0a\x05"))
	// The list has kind=PodList, which doesn't match the expected kind=Pod.
	f("wrong kind", mustReadTestFile(t, "pods_list.pb"))
}

// testWatchEvent is a decoded watch event for tests.
type testWatchEvent struct {
	eventType string
	obj       []byte
}

// readAllTestWatchEvents reads all the watch events from wr until io.EOF.
//
// It fails the test on any other error.
func readAllTestWatchEvents(t *testing.T, wr watchEventReader) []testWatchEvent {
	t.Helper()
	events, err := readTestWatchEvents(wr)
	if err != nil {
		t.Fatalf("unexpected error when reading watch events: %s", err)
	}
	return events
}

// readTestWatchEvents reads watch events from wr until an error occurs.
//
// The returned events are copies, so they remain valid after the reader re-uses its internal buffer.
func readTestWatchEvents(wr watchEventReader) ([]testWatchEvent, error) {
	var events []testWatchEvent
	for {
		eventType, obj, err := wr.readEvent()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return events, nil
			}
			return events, err
		}
		events = append(events, testWatchEvent{
			eventType: eventType,
			obj:       append([]byte(nil), obj...),
		})
	}
}

// TestWatchEventReaders verifies that protobuf and JSON watch readers return equivalent events.
func TestWatchEventReaders(t *testing.T) {
	pbData := mustReadTestFile(t, "pods_watch.pb")
	jsonData := mustReadTestFile(t, "pods_watch.json")

	pbEvents := readAllTestWatchEvents(t, newProtobufWatchEventReader(bytes.NewReader(pbData)))
	jsonEvents := readAllTestWatchEvents(t, newJSONWatchEventReader(bytes.NewReader(jsonData)))

	eventTypesExpected := []string{"ADDED", "MODIFIED", "DELETED", "BOOKMARK", "ERROR"}
	if len(pbEvents) != len(eventTypesExpected) {
		t.Fatalf("unexpected number of protobuf events; got %d; want %d", len(pbEvents), len(eventTypesExpected))
	}
	if len(jsonEvents) != len(eventTypesExpected) {
		t.Fatalf("unexpected number of JSON events; got %d; want %d", len(jsonEvents), len(eventTypesExpected))
	}

	for i := range pbEvents {
		if pbEvents[i].eventType != eventTypesExpected[i] {
			t.Fatalf("unexpected protobuf event type at position %d; got %q; want %q", i, pbEvents[i].eventType, eventTypesExpected[i])
		}
		if jsonEvents[i].eventType != eventTypesExpected[i] {
			t.Fatalf("unexpected JSON event type at position %d; got %q; want %q", i, jsonEvents[i].eventType, eventTypesExpected[i])
		}
		switch pbEvents[i].eventType {
		case "ADDED", "MODIFIED", "DELETED":
			oPB, err := parsePodProtobuf(pbEvents[i].obj)
			if err != nil {
				t.Fatalf("cannot parse protobuf pod at position %d: %s", i, err)
			}
			oJSON, err := parsePod(jsonEvents[i].obj)
			if err != nil {
				t.Fatalf("cannot parse JSON pod at position %d: %s", i, err)
			}
			if !reflect.DeepEqual(oPB, oJSON) {
				t.Fatalf("unexpected pod at position %d;\ngot\n%v\nwant\n%v", i, oPB, oJSON)
			}
		case "BOOKMARK":
			rv, err := parseBookmarkResourceVersionProtobuf(pbEvents[i].obj)
			if err != nil {
				t.Fatalf("cannot parse protobuf bookmark: %s", err)
			}
			if rv != "1005" {
				t.Fatalf("unexpected bookmark resourceVersion; got %q; want %q", rv, "1005")
			}
		case "ERROR":
			emPB, err := parseErrorProtobuf(pbEvents[i].obj)
			if err != nil {
				t.Fatalf("cannot parse protobuf error: %s", err)
			}
			emJSON, err := parseError(jsonEvents[i].obj)
			if err != nil {
				t.Fatalf("cannot parse JSON error: %s", err)
			}
			emExpected := &Error{
				Code:    410,
				Message: "too old resource version: 1 (1001)",
			}
			if !reflect.DeepEqual(emPB, emExpected) {
				t.Fatalf("unexpected protobuf error;\ngot\n%v\nwant\n%v", emPB, emExpected)
			}
			if !reflect.DeepEqual(emJSON, emExpected) {
				t.Fatalf("unexpected JSON error;\ngot\n%v\nwant\n%v", emJSON, emExpected)
			}
		}
	}
}

// TestJSONWatchEventReaderMissingObject verifies that the object from the previous event isn't returned
// for the event without object.
func TestJSONWatchEventReaderMissingObject(t *testing.T) {
	data := `{"type":"ADDED","object":{"metadata":{"name":"foo"}}}{"type":"DELETED"}`
	wr := newJSONWatchEventReader(bytes.NewReader([]byte(data)))
	events := readAllTestWatchEvents(t, wr)
	if len(events) != 2 {
		t.Fatalf("unexpected number of events; got %d; want 2", len(events))
	}
	if len(events[1].obj) != 0 {
		t.Fatalf("unexpected object for the event without object: %q", events[1].obj)
	}
}

// TestProtobufMessageSizeLimit verifies that too big protobuf messages are rejected before allocating the buffer for them.
func TestProtobufMessageSizeLimit(t *testing.T) {
	// Watch event with the length prefix exceeding maxProtobufMessageSize.
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], maxProtobufMessageSize+1)
	wr := newProtobufWatchEventReader(bytes.NewReader(hdr[:]))
	if _, _, err := wr.readEvent(); err == nil || !strings.Contains(err.Error(), "too big protobuf WatchEvent length") {
		t.Fatalf("expecting too big WatchEvent length error; got %v", err)
	}
	if cap(wr.buf) != 0 {
		t.Fatalf("unexpected buffer allocation for too big WatchEvent; cap=%d", cap(wr.buf))
	}

	// Field in object list exceeding maxProtobufMessageSize.
	sr := &protobufStreamReader{
		br: bufio.NewReader(bytes.NewReader(nil)),
	}
	if _, err := sr.readBytes(maxProtobufMessageSize + 1); err == nil || !strings.Contains(err.Error(), "too big protobuf field length") {
		t.Fatalf("expecting too big field length error; got %v", err)
	}
	if cap(sr.buf) != 0 {
		t.Fatalf("unexpected buffer allocation for too big field; cap=%d", cap(sr.buf))
	}
}

// TestGetProtobufInt verifies decoding of int32 fields, including negative values encoded as 64-bit varints.
func TestGetProtobufInt(t *testing.T) {
	f := func(src []byte, resultExpected int, okExpected bool) {
		t.Helper()
		var fc easyproto.FieldContext
		if _, err := fc.NextField(src); err != nil {
			t.Fatalf("cannot read field: %s", err)
		}
		result, err := getProtobufInt(&fc, "test")
		if (err == nil) != okExpected {
			t.Fatalf("unexpected error: %v; want ok=%v", err, okExpected)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %d; want %d", result, resultExpected)
		}
	}
	// field 1, varint wire type
	f([]byte{0x08, 0x00}, 0, true)
	f([]byte{0x08, 0x89, 0x01}, 137, true)
	f([]byte{0x08, 0xff, 0xff, 0xff, 0xff, 0x07}, math.MaxInt32, true)
	// -1 encoded as 10-byte varint
	f([]byte{0x08, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}, -1, true)
	// math.MinInt32
	f([]byte{0x08, 0x80, 0x80, 0x80, 0x80, 0xf8, 0xff, 0xff, 0xff, 0xff, 0x01}, math.MinInt32, true)
	// math.MaxInt32+1 doesn't fit int32
	f([]byte{0x08, 0x80, 0x80, 0x80, 0x80, 0x08}, 0, false)
	// length-delimited wire type
	f([]byte{0x0a, 0x00}, 0, false)
}

// TestProtobufWatchEventReaderTruncated verifies that a truncated watch stream never returns a corrupt event.
func TestProtobufWatchEventReaderTruncated(t *testing.T) {
	data := mustReadTestFile(t, "pods_watch.pb")
	eventsFull := readAllTestWatchEvents(t, newProtobufWatchEventReader(bytes.NewReader(data)))

	for i := 0; i <= len(data); i++ {
		events, _ := readTestWatchEvents(newProtobufWatchEventReader(bytes.NewReader(data[:i])))
		if len(events) > len(eventsFull) {
			t.Fatalf("unexpected number of events for the prefix of %d bytes; got %d; want at most %d", i, len(events), len(eventsFull))
		}
		for j := range events {
			if events[j].eventType != eventsFull[j].eventType || !bytes.Equal(events[j].obj, eventsFull[j].obj) {
				t.Fatalf("unexpected event %d for the prefix of %d bytes", j, i)
			}
		}
	}
}

func TestGetObjectProtobufParsersForRole(t *testing.T) {
	if po, pol := getObjectProtobufParsersForRole("pod"); po == nil || pol == nil {
		t.Fatalf("role=pod must support protobuf parsing")
	}
	for _, role := range []string{"node", "service", "endpoints", "endpointslice", "ingress", "namespace"} {
		if po, pol := getObjectProtobufParsersForRole(role); po != nil || pol != nil {
			t.Fatalf("role=%q mustn't support protobuf parsing", role)
		}
	}
}

// TestReloadObjectsProtobuf verifies content negotiation:
//   - protobuf is requested via the Accept header when protobuf parsing is enabled;
//   - protobuf responses are parsed with the protobuf parser;
//   - JSON responses are parsed with the JSON parser (fallback).
func TestReloadObjectsProtobuf(t *testing.T) {
	pbData := mustReadTestFile(t, "pods_list.pb")
	jsonData := mustReadTestFile(t, "pods_list.json")
	objectsExpected, _, err := parsePodList(bytes.NewReader(jsonData))
	if err != nil {
		t.Fatalf("cannot parse JSON list: %s", err)
	}

	f := func(name, responseContentType string, responseBody []byte) {
		t.Helper()
		var gotAccept string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAccept = r.Header.Get("Accept")
			w.Header().Set("Content-Type", responseContentType)
			_, _ = w.Write(responseBody)
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		gw := &groupWatcher{
			client:     server.Client(),
			ctx:        ctx,
			cancel:     cancel,
			setHeaders: func(_ *http.Request) error { return nil },
			m:          make(map[string]*urlWatcher),
		}
		uw := newURLWatcher("pod", server.URL+"/api/v1/pods", gw)
		// Enable protobuf parsing without touching the global flag, so the test doesn't race with background watchers.
		uw.parseObjectProtobuf, uw.parseObjectListProtobuf = getObjectProtobufParsersForRole("pod")
		gw.m["pod"] = uw

		rv := uw.reloadObjects()
		if rv != "1001" {
			t.Fatalf("[%s] unexpected resourceVersion; got %q; want %q", name, rv, "1001")
		}
		if gotAccept != protobufAcceptHeader {
			t.Fatalf("[%s] unexpected Accept header; got %q; want %q", name, gotAccept, protobufAcceptHeader)
		}
		if !reflect.DeepEqual(uw.objectsByKey, objectsExpected) {
			t.Fatalf("[%s] unexpected objects;\ngot\n%v\nwant\n%v", name, uw.objectsByKey, objectsExpected)
		}
	}
	f("protobuf", protobufContentType, pbData)
	f("json-fallback", "application/json", jsonData)
}
