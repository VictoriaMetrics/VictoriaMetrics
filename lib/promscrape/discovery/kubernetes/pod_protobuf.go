package kubernetes

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/VictoriaMetrics/easyproto"
)

// Field numbers for protobuf messages below are taken from https://github.com/kubernetes/api/blob/master/core/v1/generated.proto

// parsePodProtobuf parses a single Pod from protobuf-encoded data.
//
// data must start with protobufMagic followed by runtime.Unknown message.
func parsePodProtobuf(data []byte) (object, error) {
	kind, raw, err := unwrapProtobufObject(data)
	if err != nil {
		return nil, err
	}
	if err := checkProtobufKind(kind, "Pod"); err != nil {
		return nil, err
	}
	var p Pod
	if err := p.unmarshalProtobuf(raw); err != nil {
		return nil, fmt.Errorf("cannot unmarshal protobuf Pod: %w", err)
	}
	return &p, nil
}

// parsePodListProtobuf parses a PodList from protobuf-encoded r item by item.
//
// The returned objects are equivalent to the objects returned by parsePodList for the same list.
func parsePodListProtobuf(r io.Reader) (map[string]object, ListMeta, error) {
	objectsByKey, lm, err := decodePodListProtobuf(r)
	if err != nil {
		return nil, lm, fmt.Errorf("cannot unmarshal protobuf PodList: %w", err)
	}
	return objectsByKey, lm, nil
}

func decodePodListProtobuf(r io.Reader) (map[string]object, ListMeta, error) {
	var lm ListMeta
	sr := &protobufStreamReader{
		br: bufio.NewReaderSize(r, 64*1024),
	}
	magic, err := sr.readBytes(uint64(len(protobufMagic)))
	if err != nil {
		return nil, lm, err
	}
	if !bytes.Equal(magic, protobufMagic) {
		return nil, lm, fmt.Errorf("missing protobuf magic prefix %q; got %q", protobufMagic, magic)
	}
	var objectsByKey map[string]object

	// Read runtime.Unknown message till the end of r.
	for {
		fieldNum, wireType, err := sr.readTag()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, lm, err
		}
		if wireType != protobufWireTypeLen {
			if err := sr.skipField(wireType); err != nil {
				return nil, lm, err
			}
			continue
		}
		n, err := sr.readLen()
		if err != nil {
			return nil, lm, err
		}
		switch fieldNum {
		case 1:
			// typeMeta
			data, err := sr.readBytes(n)
			if err != nil {
				return nil, lm, err
			}
			kind, err := getProtobufTypeMetaKind(data)
			if err != nil {
				return nil, lm, err
			}
			if err := checkProtobufKind(kind, "PodList"); err != nil {
				return nil, lm, err
			}
		case 2:
			// raw
			objectsByKey = make(map[string]object)
			lm, err = decodePodListItemsProtobuf(sr, n, objectsByKey)
			if err != nil {
				return nil, lm, err
			}
		case 3:
			// contentEncoding
			data, err := sr.readBytes(n)
			if err != nil {
				return nil, lm, err
			}
			if len(data) > 0 {
				return nil, lm, fmt.Errorf("unsupported contentEncoding=%q", data)
			}
		default:
			if err := sr.discard(n); err != nil {
				return nil, lm, err
			}
		}
	}
	if objectsByKey == nil {
		// Kubernetes API server always sends raw data with list metadata, even for empty lists.
		// The missing raw data means the response is truncated, so the list mustn't be treated as empty.
		return nil, lm, fmt.Errorf("missing raw data for PodList: %w", io.ErrUnexpectedEOF)
	}
	return objectsByKey, lm, nil
}

// decodePodListItemsProtobuf decodes PodList message of the given size from sr and puts the decoded items to dst.
func decodePodListItemsProtobuf(sr *protobufStreamReader, size uint64, dst map[string]object) (ListMeta, error) {
	var lm ListMeta
	end := sr.n + size
	for sr.n < end {
		fieldNum, wireType, err := sr.readTag()
		if err != nil {
			return lm, noEOF(err)
		}
		if wireType != protobufWireTypeLen {
			if err := sr.skipField(wireType); err != nil {
				return lm, err
			}
			continue
		}
		n, err := sr.readLen()
		if err != nil {
			return lm, err
		}
		switch fieldNum {
		case 1:
			// metadata
			data, err := sr.readBytes(n)
			if err != nil {
				return lm, err
			}
			rv, _, err := easyproto.GetString(data, 2)
			if err != nil {
				return lm, fmt.Errorf("cannot read list metadata: %w", err)
			}
			lm.ResourceVersion = strings.Clone(rv)
		case 2:
			// items
			data, err := sr.readBytes(n)
			if err != nil {
				return lm, err
			}
			var p Pod
			if err := p.unmarshalProtobuf(data); err != nil {
				return lm, fmt.Errorf("cannot unmarshal list item: %w", err)
			}
			dst[p.key()] = &p
		default:
			if err := sr.discard(n); err != nil {
				return lm, err
			}
		}
	}
	if sr.n != end {
		return lm, fmt.Errorf("list fields exceed the list message size %d bytes", size)
	}
	return lm, nil
}

func (p *Pod) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			err = unmarshalProtobufMessage(&fc, "metadata", &p.Metadata)
		case 2:
			err = unmarshalProtobufMessage(&fc, "spec", &p.Spec)
		case 3:
			err = unmarshalProtobufMessage(&fc, "status", &p.Status)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (ps *PodSpec) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 2:
			ps.Containers = append(ps.Containers, Container{})
			err = unmarshalProtobufMessage(&fc, "containers", &ps.Containers[len(ps.Containers)-1])
		case 10:
			ps.NodeName, err = getProtobufString(&fc, "nodeName")
		case 20:
			ps.InitContainers = append(ps.InitContainers, Container{})
			err = unmarshalProtobufMessage(&fc, "initContainers", &ps.InitContainers[len(ps.InitContainers)-1])
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			c.Name, err = getProtobufString(&fc, "name")
		case 2:
			c.Image, err = getProtobufString(&fc, "image")
		case 6:
			c.Ports = append(c.Ports, ContainerPort{})
			err = unmarshalProtobufMessage(&fc, "ports", &c.Ports[len(c.Ports)-1])
		case 24:
			c.RestartPolicy, err = getProtobufString(&fc, "restartPolicy")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (cp *ContainerPort) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			cp.Name, err = getProtobufString(&fc, "name")
		case 3:
			cp.ContainerPort, err = getProtobufInt(&fc, "containerPort")
		case 4:
			cp.Protocol, err = getProtobufString(&fc, "protocol")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (ps *PodStatus) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			ps.Phase, err = getProtobufString(&fc, "phase")
		case 2:
			ps.Conditions = append(ps.Conditions, PodCondition{})
			err = unmarshalProtobufMessage(&fc, "conditions", &ps.Conditions[len(ps.Conditions)-1])
		case 5:
			ps.HostIP, err = getProtobufString(&fc, "hostIP")
		case 6:
			ps.PodIP, err = getProtobufString(&fc, "podIP")
		case 8:
			ps.ContainerStatuses = append(ps.ContainerStatuses, ContainerStatus{})
			err = unmarshalProtobufMessage(&fc, "containerStatuses", &ps.ContainerStatuses[len(ps.ContainerStatuses)-1])
		case 10:
			ps.InitContainerStatuses = append(ps.InitContainerStatuses, ContainerStatus{})
			err = unmarshalProtobufMessage(&fc, "initContainerStatuses", &ps.InitContainerStatuses[len(ps.InitContainerStatuses)-1])
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (pc *PodCondition) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			pc.Type, err = getProtobufString(&fc, "type")
		case 2:
			pc.Status, err = getProtobufString(&fc, "status")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (cs *ContainerStatus) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			cs.Name, err = getProtobufString(&fc, "name")
		case 2:
			err = unmarshalProtobufMessage(&fc, "state", &cs.State)
		case 8:
			cs.ContainerID, err = getProtobufString(&fc, "containerID")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (cs *ContainerState) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		if fc.FieldNum == 3 {
			if cs.Terminated == nil {
				cs.Terminated = &ContainerStateTerminated{}
			}
			if err := unmarshalProtobufMessage(&fc, "terminated", cs.Terminated); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cst *ContainerStateTerminated) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		if fc.FieldNum == 1 {
			cst.ExitCode, err = getProtobufInt(&fc, "exitCode")
			if err != nil {
				return err
			}
		}
	}
	return nil
}
