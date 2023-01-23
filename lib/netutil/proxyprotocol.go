package netutil

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

var bufioReaderPool = sync.Pool{}

func getBufioReader(r io.Reader) *bufio.Reader {
	v := bufioReaderPool.Get()
	if v == nil {
		return bufio.NewReader(r)
	}
	br := v.(*bufio.Reader)
	br.Reset(r)
	return br
}

func putBufioReader(r *bufio.Reader) {
	bufioReaderPool.Put(r)
}

type proxyProtocolConn struct {
	net.Conn
	br         *bufio.Reader
	remoteAddr net.Addr
}

func newProxyProtocolConn(srcConn net.Conn) (net.Conn, error) {
	br := getBufioReader(srcConn)
	maybeSrcAddr, err := readProxyProto(br)
	if err != nil {
		return nil, fmt.Errorf("cannot read proxy protocol from connection: %w", err)
	}
	if maybeSrcAddr == nil {
		maybeSrcAddr = srcConn.RemoteAddr()
	}
	return &proxyProtocolConn{
		Conn:       srcConn,
		br:         br,
		remoteAddr: maybeSrcAddr,
	}, nil
}

func (ppc *proxyProtocolConn) Read(b []byte) (n int, err error) {
	return ppc.br.Read(b)
}

func (ppc *proxyProtocolConn) RemoteAddr() net.Addr {
	return ppc.remoteAddr
}

func (ppc *proxyProtocolConn) Close() error {
	putBufioReader(ppc.br)
	return ppc.Conn.Close()
}

var (
	V2Identifier = []byte("\r\n\r\n\x00\r\nQUIT\n")
)

// https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
func readProxyProto(r *bufio.Reader) (net.Addr, error) {
	// first 12 bytes - signature
	// 13 byte - protocol and command
	maybeProto, err := r.Peek(13)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read data from buffer: %w", err)
	}
	if !bytes.HasPrefix(maybeProto, V2Identifier) {
		// it's not a proxy protocol
		return nil, nil
	}
	command := maybeProto[12]
	// advance reader
	if _, err := r.Discard(13); err != nil {
		return nil, fmt.Errorf("cannot adavance prefix reader, possible bug: %w", err)
	}
	// Ensure the version is 2
	if (command & 0xF0) != 0x20 {
		return nil, fmt.Errorf("unsupported proxy protocol version, only v2 protocol version is supported, got: %d", command&0xF0)
	}
	return readSrcProxyAddr(r, command)
}

func readSrcProxyAddr(r *bufio.Reader, command byte) (net.Addr, error) {
	var srcAddr net.Addr

	// Read protocol header
	// 1 byte contain the transport proto and family
	// 2-3 byte the length of the header
	h, err := r.Peek(3)
	if err != nil {
		return nil, fmt.Errorf("cannot read proxy protocol header prefix: %w", err)
	}
	ipFamily := h[0]
	// The length of the remainder of the header including any TLVs in network byte order
	// 0, 1, 2
	length := int(binary.BigEndian.Uint16(h[1:3]))
	// in general RFC doesn't limit header length, but for sane check lets limit it to 2kb
	// in theory TLVs may occupy some space
	if length > 2048 {
		return nil, fmt.Errorf("too big proxy protocol header length: %d", length)
	}
	if _, err := r.Discard(3); err != nil {
		return nil, fmt.Errorf("cannot advance header prefix reader, possible bug: %w", err)
	}
	// Read the remainder of the header
	ppHeader, err := r.Peek(length)
	if err != nil {
		return nil, fmt.Errorf("cannot read proxy protocol header: %w", err)
	}
	switch command & 0x0F {
	case 0x00:
		// proxy LOCAL command
		// no-op
	case 0x01:
		// Translate the addresses according to the family
		switch ipFamily {
		// ipv4
		case 0x11, 0x12:
			if len(ppHeader) < 12 {
				return nil, fmt.Errorf("expected %d bytes for IPV4 address", 12)
			}
			srcAddr = &net.TCPAddr{
				IP:   net.IPv4(ppHeader[0], ppHeader[1], ppHeader[2], ppHeader[3]),
				Port: int(binary.BigEndian.Uint16(ppHeader[8:10])),
			}
			if (ipFamily & 0x0F) == 0x02 {
				srcAddr = &net.UDPAddr{
					IP:   net.IPv4(ppHeader[0], ppHeader[1], ppHeader[2], ppHeader[3]),
					Port: int(binary.BigEndian.Uint16(ppHeader[8:10])),
				}
			}
			// ipv6
		case 0x21, 0x22:
			if len(ppHeader) < 36 {
				return nil, fmt.Errorf("expected %d bytes for IPV6 address", 36)
			}

			srcAddr = &net.TCPAddr{
				IP:   ppHeader[0:16],
				Port: int(binary.BigEndian.Uint16(ppHeader[32:34])),
			}

			if (ipFamily & 0x0F) == 0x02 { // UDP
				srcAddr = &net.UDPAddr{
					IP:   ppHeader[0:16],
					Port: int(binary.BigEndian.Uint16(ppHeader[32:34])),
				}
			}
			// not supported proto family
		default:
			return nil, fmt.Errorf("unsupported protocol family: %d", ipFamily)
		}
		// not supported proto families
	default:
		return nil, fmt.Errorf("unsupported proxy protocol command: %d", command)
	}
	// we skip any TLVs, it's not interested to us
	// advance reader
	if _, err := r.Discard(length); err != nil {
		return nil, fmt.Errorf("cannot advance header reader, possible bug: %w", err)
	}
	return srcAddr, nil
}
