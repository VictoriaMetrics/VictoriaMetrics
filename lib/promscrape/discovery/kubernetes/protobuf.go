package kubernetes

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"mime"
	"strings"

	"github.com/VictoriaMetrics/easyproto"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

var useProtobuf = flag.Bool("promscrape.kubernetes.useProtobuf", false, "Whether to request Kubernetes objects from Kubernetes API server in protobuf format instead of JSON. "+
	"This may reduce CPU usage, memory usage and network traffic when discovering a large number of Kubernetes objects. "+
	"Currently protobuf is requested only for pods, including pods watched for role: endpoints and role: endpointslice, while other objects are requested in JSON. "+
	"JSON responses are still accepted if Kubernetes API server cannot return protobuf for the requested objects")

const (
	protobufContentType = "application/vnd.kubernetes.protobuf"

	// protobufAcceptHeader allows Kubernetes API server to fall back to JSON for objects without protobuf support.
	protobufAcceptHeader = protobufContentType + ", application/json"

	// maxProtobufMessageSize is the maximum size of a single protobuf-encoded Kubernetes object or watch event.
	maxProtobufMessageSize = 64 * 1024 * 1024
)

// protobufMagic is the prefix for protobuf-encoded Kubernetes objects.
//
// See https://kubernetes.io/docs/reference/using-api/api-concepts/#protobuf-encoding
var protobufMagic = []byte("k8s\x00")

// isProtobufContentType returns true if the given Content-Type header value corresponds to Kubernetes protobuf encoding.
func isProtobufContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == protobufContentType
}

// protobufUnmarshaler is implemented by types, which can be unmarshaled from protobuf.
type protobufUnmarshaler interface {
	unmarshalProtobuf(src []byte) error
}

// unwrapProtobufObject returns kind and protobuf-encoded object from data, which must start with protobufMagic followed by runtime.Unknown message.
//
// The returned kind and raw are valid while data isn't changed.
func unwrapProtobufObject(data []byte) (string, []byte, error) {
	if !bytes.HasPrefix(data, protobufMagic) {
		return "", nil, fmt.Errorf("missing protobuf magic prefix %q", protobufMagic)
	}
	src := data[len(protobufMagic):]
	var kind string
	var raw []byte
	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return "", nil, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			typeMeta, ok := fc.MessageData()
			if !ok {
				return "", nil, fmt.Errorf("cannot read typeMeta")
			}
			kind, err = getProtobufTypeMetaKind(typeMeta)
			if err != nil {
				return "", nil, err
			}
		case 2:
			b, ok := fc.Bytes()
			if !ok {
				return "", nil, fmt.Errorf("cannot read raw")
			}
			raw = b
		case 3:
			contentEncoding, ok := fc.String()
			if !ok {
				return "", nil, fmt.Errorf("cannot read contentEncoding")
			}
			if contentEncoding != "" {
				return "", nil, fmt.Errorf("unsupported contentEncoding=%q", contentEncoding)
			}
		}
	}
	return kind, raw, nil
}

func getProtobufTypeMetaKind(typeMeta []byte) (string, error) {
	kind, _, err := easyproto.GetString(typeMeta, 2)
	if err != nil {
		return "", fmt.Errorf("cannot read kind from typeMeta: %w", err)
	}
	return kind, nil
}

func checkProtobufKind(kind, kindExpected string) error {
	if kind != "" && kind != kindExpected {
		return fmt.Errorf("unexpected kind=%q; want %q", kind, kindExpected)
	}
	return nil
}

// parseBookmarkResourceVersionProtobuf returns resourceVersion from protobuf-encoded bookmark object.
//
// Bookmark object contains only metadata.resourceVersion. Metadata is the first field for all the Kubernetes objects.
func parseBookmarkResourceVersionProtobuf(data []byte) (string, error) {
	_, raw, err := unwrapProtobufObject(data)
	if err != nil {
		return "", err
	}
	metadata, _, err := easyproto.GetMessageData(raw, 1)
	if err != nil {
		return "", fmt.Errorf("cannot read metadata: %w", err)
	}
	rv, _, err := easyproto.GetString(metadata, 6)
	if err != nil {
		return "", fmt.Errorf("cannot read resourceVersion: %w", err)
	}
	return strings.Clone(rv), nil
}

// parseErrorProtobuf parses protobuf-encoded Status object.
func parseErrorProtobuf(data []byte) (*Error, error) {
	kind, raw, err := unwrapProtobufObject(data)
	if err != nil {
		return nil, err
	}
	if err := checkProtobufKind(kind, "Status"); err != nil {
		return nil, err
	}
	message, _, err := easyproto.GetString(raw, 3)
	if err != nil {
		return nil, fmt.Errorf("cannot read message: %w", err)
	}
	code, _, err := easyproto.GetInt32(raw, 6)
	if err != nil {
		return nil, fmt.Errorf("cannot read code: %w", err)
	}
	return &Error{
		Code:    int(code),
		Message: strings.Clone(message),
	}, nil
}

// watchEventReader reads Kubernetes watch events from the watch response body.
type watchEventReader interface {
	// readEvent returns the type and the encoded object of the next watch event.
	//
	// The returned object is valid until the next readEvent call.
	readEvent() (string, []byte, error)
}

// jsonWatchEventReader reads JSON-encoded watch events.
type jsonWatchEventReader struct {
	d  *json.Decoder
	we WatchEvent
}

func newJSONWatchEventReader(r io.Reader) *jsonWatchEventReader {
	return &jsonWatchEventReader{
		d: json.NewDecoder(r),
	}
}

func (wr *jsonWatchEventReader) readEvent() (string, []byte, error) {
	// Reset the previous event, since Decode leaves fields missing in the next event unchanged.
	wr.we = WatchEvent{}
	if err := wr.d.Decode(&wr.we); err != nil {
		return "", nil, fmt.Errorf("cannot parse WatchEvent json response: %w", err)
	}
	return wr.we.Type, wr.we.Object, nil
}

// protobufWatchEventReader reads protobuf-encoded watch events.
//
// Every event is encoded as 4-byte big-endian length followed by WatchEvent message.
// See https://kubernetes.io/docs/reference/using-api/api-concepts/#protobuf-encoding
type protobufWatchEventReader struct {
	r   io.Reader
	hdr [4]byte
	buf []byte
}

func newProtobufWatchEventReader(r io.Reader) *protobufWatchEventReader {
	return &protobufWatchEventReader{
		r: r,
	}
}

func (wr *protobufWatchEventReader) readEvent() (string, []byte, error) {
	if _, err := io.ReadFull(wr.r, wr.hdr[:]); err != nil {
		// io.EOF is returned only if the stream is closed between events.
		return "", nil, fmt.Errorf("cannot read protobuf WatchEvent length: %w", err)
	}
	n := binary.BigEndian.Uint32(wr.hdr[:])
	if n > maxProtobufMessageSize {
		return "", nil, fmt.Errorf("too big protobuf WatchEvent length: %d bytes; mustn't exceed %d bytes", n, maxProtobufMessageSize)
	}
	wr.buf = slicesutil.SetLength(wr.buf, int(n))
	if _, err := io.ReadFull(wr.r, wr.buf); err != nil {
		return "", nil, fmt.Errorf("cannot read protobuf WatchEvent: %w", noEOF(err))
	}
	eventType, obj, err := unmarshalProtobufWatchEvent(wr.buf)
	if err != nil {
		return "", nil, fmt.Errorf("cannot unmarshal protobuf WatchEvent: %w", err)
	}
	return eventType, obj, nil
}

// unmarshalProtobufWatchEvent returns the type and the object of the protobuf-encoded WatchEvent from src.
//
// The returned object is valid while src isn't changed.
func unmarshalProtobufWatchEvent(src []byte) (string, []byte, error) {
	var eventType string
	var obj []byte
	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return "", nil, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			s, ok := fc.String()
			if !ok {
				return "", nil, fmt.Errorf("cannot read type")
			}
			eventType = s
		case 2:
			rawExtension, ok := fc.MessageData()
			if !ok {
				return "", nil, fmt.Errorf("cannot read object")
			}
			raw, _, err := easyproto.GetBytes(rawExtension, 1)
			if err != nil {
				return "", nil, fmt.Errorf("cannot read object raw data: %w", err)
			}
			obj = raw
		}
	}
	return getWatchEventType(eventType), obj, nil
}

// getWatchEventType returns a copy of eventType, which may refer to a temporary buffer.
func getWatchEventType(eventType string) string {
	switch eventType {
	case "ADDED":
		return "ADDED"
	case "MODIFIED":
		return "MODIFIED"
	case "DELETED":
		return "DELETED"
	case "BOOKMARK":
		return "BOOKMARK"
	case "ERROR":
		return "ERROR"
	default:
		return strings.Clone(eventType)
	}
}

const (
	protobufWireTypeVarint = 0
	protobufWireTypeI64    = 1
	protobufWireTypeLen    = 2
	protobufWireTypeI32    = 5
)

// protobufStreamReader reads protobuf fields from the underlying reader without buffering the whole message in memory.
type protobufStreamReader struct {
	br *bufio.Reader

	// n is the number of bytes read from br.
	n uint64

	buf []byte
}

// ReadByte implements io.ByteReader interface.
func (sr *protobufStreamReader) ReadByte() (byte, error) {
	b, err := sr.br.ReadByte()
	if err != nil {
		return 0, err
	}
	sr.n++
	return b, nil
}

// readTag reads the tag of the next field.
//
// io.EOF is returned if there are no more fields.
func (sr *protobufStreamReader) readTag() (uint32, byte, error) {
	tag, err := binary.ReadUvarint(sr)
	if err != nil {
		return 0, 0, err
	}
	fieldNum := tag >> 3
	if fieldNum == 0 || fieldNum > math.MaxUint32 {
		return 0, 0, fmt.Errorf("invalid field number: %d", fieldNum)
	}
	return uint32(fieldNum), byte(tag & 0x07), nil
}

// readLen reads the length of length-delimited field.
func (sr *protobufStreamReader) readLen() (uint64, error) {
	n, err := binary.ReadUvarint(sr)
	if err != nil {
		return 0, noEOF(err)
	}
	return n, nil
}

// readBytes reads n bytes.
//
// The returned bytes are valid until the next readBytes call.
func (sr *protobufStreamReader) readBytes(n uint64) ([]byte, error) {
	if n > maxProtobufMessageSize {
		return nil, fmt.Errorf("too big protobuf field length: %d bytes; mustn't exceed %d bytes", n, maxProtobufMessageSize)
	}
	sr.buf = slicesutil.SetLength(sr.buf, int(n))
	k, err := io.ReadFull(sr.br, sr.buf)
	sr.n += uint64(k)
	if err != nil {
		return nil, noEOF(err)
	}
	return sr.buf, nil
}

// discard skips n bytes.
func (sr *protobufStreamReader) discard(n uint64) error {
	for n > 0 {
		m := min(n, 1<<30)
		k, err := sr.br.Discard(int(m))
		sr.n += uint64(k)
		if err != nil {
			return noEOF(err)
		}
		n -= m
	}
	return nil
}

// skipField skips the value of the field with the given wireType.
func (sr *protobufStreamReader) skipField(wireType byte) error {
	switch wireType {
	case protobufWireTypeVarint:
		_, err := binary.ReadUvarint(sr)
		return noEOF(err)
	case protobufWireTypeI64:
		return sr.discard(8)
	case protobufWireTypeLen:
		n, err := sr.readLen()
		if err != nil {
			return err
		}
		return sr.discard(n)
	case protobufWireTypeI32:
		return sr.discard(4)
	default:
		return fmt.Errorf("unsupported wireType=%d", wireType)
	}
}

// noEOF converts io.EOF to io.ErrUnexpectedEOF.
//
// It must be used when the data is truncated in the middle.
func noEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

// unmarshalProtobufMessage unmarshals protobuf message for the field fc into dst.
func unmarshalProtobufMessage(fc *easyproto.FieldContext, name string, dst protobufUnmarshaler) error {
	data, ok := fc.MessageData()
	if !ok {
		return fmt.Errorf("cannot read %s", name)
	}
	if err := dst.unmarshalProtobuf(data); err != nil {
		return fmt.Errorf("cannot unmarshal %s: %w", name, err)
	}
	return nil
}

// getProtobufString returns a copy of string value for the field fc.
func getProtobufString(fc *easyproto.FieldContext, name string) (string, error) {
	s, ok := fc.String()
	if !ok {
		return "", fmt.Errorf("cannot read %s", name)
	}
	return strings.Clone(s), nil
}

// getProtobufInt returns int32 value for the field fc.
func getProtobufInt(fc *easyproto.FieldContext, name string) (int, error) {
	// Negative int32 values are encoded as 64-bit varints, so they cannot be read via fc.Int32().
	n, ok := fc.Int64()
	if !ok || n < math.MinInt32 || n > math.MaxInt32 {
		return 0, fmt.Errorf("cannot read %s", name)
	}
	return int(n), nil
}

// unmarshalProtobuf unmarshals ObjectMeta from protobuf-encoded src.
//
// See https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/v1/generated.proto
func (om *ObjectMeta) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			om.Name, err = getProtobufString(&fc, "name")
		case 3:
			om.Namespace, err = getProtobufString(&fc, "namespace")
		case 5:
			om.UID, err = getProtobufString(&fc, "uid")
		case 11:
			om.Labels, err = appendProtobufMapEntry(om.Labels, &fc, "labels")
		case 12:
			om.Annotations, err = appendProtobufMapEntry(om.Annotations, &fc, "annotations")
		case 13:
			om.OwnerReferences = append(om.OwnerReferences, OwnerReference{})
			err = unmarshalProtobufMessage(&fc, "ownerReferences", &om.OwnerReferences[len(om.OwnerReferences)-1])
		}
		if err != nil {
			return err
		}
	}
	sortProtobufLabels(om.Labels)
	sortProtobufLabels(om.Annotations)
	return nil
}

// unmarshalProtobuf unmarshals OwnerReference from protobuf-encoded src.
func (or *OwnerReference) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			or.Kind, err = getProtobufString(&fc, "kind")
		case 3:
			or.Name, err = getProtobufString(&fc, "name")
		case 6:
			v, ok := fc.Bool()
			if !ok {
				return fmt.Errorf("cannot read controller")
			}
			or.Controller = v
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// appendProtobufMapEntry appends map<string, string> entry for the field fc to x.
func appendProtobufMapEntry(x *promutil.Labels, fc *easyproto.FieldContext, name string) (*promutil.Labels, error) {
	src, ok := fc.MessageData()
	if !ok {
		return x, fmt.Errorf("cannot read %s", name)
	}
	var key, value string
	var entry easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = entry.NextField(src)
		if err != nil {
			return x, fmt.Errorf("cannot read the next field for %s entry: %w", name, err)
		}
		switch entry.FieldNum {
		case 1:
			key, ok = entry.String()
		case 2:
			value, ok = entry.String()
		}
		if !ok {
			return x, fmt.Errorf("cannot read %s entry", name)
		}
	}
	if x == nil {
		x = &promutil.Labels{}
	}
	x.Add(strings.Clone(key), strings.Clone(value))
	return x, nil
}

// sortProtobufLabels sorts x by name and removes entries with duplicate names except of the last one.
//
// This makes x equivalent to the labels obtained from JSON-encoded map.
func sortProtobufLabels(x *promutil.Labels) {
	if x == nil {
		return
	}
	x.SortStable()
	labels := x.Labels
	dst := labels[:0]
	for i := range labels {
		if i+1 < len(labels) && labels[i+1].Name == labels[i].Name {
			continue
		}
		dst = append(dst, labels[i])
	}
	clear(labels[len(dst):])
	x.Labels = dst
}
