package vmselectapi

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
)

func setupTestCtx(t *testing.T) (*vmselectRequestCtx, net.Conn) {
	t.Helper()

	client, server := net.Pipe()

	// Use channel to synchronize handshake results
	errCh := make(chan error, 1)

	go func() {
		bc, err := handshake.VMSelectServer(server, 0)
		if err != nil {
			errCh <- err
			return
		}
		// Keep bc open to ensure the connection remains usable
		errCh <- nil
		_ = bc
	}()

	// Client-side handshake
	bc, err := handshake.VMSelectClient(client, 0)
	if err != nil {
		t.Fatalf("client handshake failed: %v", err)
	}

	// Wait for the server-side handshake to complete
	if err := <-errCh; err != nil {
		t.Fatalf("server handshake failed: %v", err)
	}

	return &vmselectRequestCtx{
		bc:      bc,
		sizeBuf: make([]byte, 8),
	}, server
}

func TestVmselectRequestCtx_WriteSingleMetaFrame(t *testing.T) {
	tests := []struct {
		name       string
		version    uint8
		fieldIndex uint16
		payload    []byte
	}{
		{
			name:       "with_payload",
			version:    1,
			fieldIndex: 123,
			payload:    []byte("test-meta"),
		},
		{
			name:       "empty_payload",
			version:    0,
			fieldIndex: 5,
			payload:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, server := setupTestCtx(t)
			defer server.Close()
			defer ctx.bc.Close()

			dataSize := uint64(len(tt.payload))

			// Writing data in a separate goroutine
			go func() {
				if err := ctx.writeSingleMetaFrame(tt.version, tt.fieldIndex, tt.payload); err != nil {
					t.Errorf("writeSingleMetaFrame failed: %v", err)
				}
				ctx.bc.Flush()
			}()

			// === 1. Read and verify header ===
			headerBuf := make([]byte, 8)
			if _, err := io.ReadFull(server, headerBuf); err != nil {
				t.Fatalf("failed to read header: %v", err)
			}
			header := binary.BigEndian.Uint64(headerBuf)

			// Verify FlagMetadata bit
			if (header & FlagMetadata) == 0 {
				t.Error("FlagMetadata bit should be set")
			}

			// Verify Version
			gotVersion := uint8((header >> 61) & MaskVersion)
			if gotVersion != tt.version {
				t.Errorf("expected version %d, got %d", tt.version, gotVersion)
			}

			// Verify FieldIndex
			gotFieldIndex := uint16((header >> 48) & MaskFieldIndex)
			if gotFieldIndex != tt.fieldIndex {
				t.Errorf("expected fieldIndex %d, got %d", tt.fieldIndex, gotFieldIndex)
			}

			// Verify DataSize
			gotSize := header & MaskDataSize
			if gotSize != dataSize {
				t.Errorf("expected dataSize %d, got %d", dataSize, gotSize)
			}

			// === 2. Verify payload (only if non-empty) ===
			if dataSize > 0 {
				payloadBuf := make([]byte, dataSize)
				if _, err := io.ReadFull(server, payloadBuf); err != nil {
					t.Fatalf("failed to read payload: %v", err)
				}
				if !bytes.Equal(payloadBuf, tt.payload) {
					t.Errorf("expected payload %q, got %q", tt.payload, payloadBuf)
				}
			} else {
				// Empty payload: confirm no extra data exists (non-blocking check)
				server.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
				buf := make([]byte, 1)
				n, err := server.Read(buf)

				if err == nil || n > 0 {
					t.Errorf("expected no payload, but got %d bytes", n)
				}

				// Timeout is the expected behavior here; do not treat it as an error
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					// expected result
				} else if err != nil && err != io.EOF {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
