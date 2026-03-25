package vmselectapi

import (
	"encoding/binary"
	"io"
	"net"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
)

// setupTestCtx creates a mock environment for testing vmselectRequestCtx.
// It performs a real handshake over a net.Pipe to ensure the BufferedConn is properly initialized.
func setupTestCtx(t *testing.T) (*vmselectRequestCtx, net.Conn) {
	t.Helper()

	// Create a synchronous in-memory pipe.
	client, server := net.Pipe()

	// Run the server-side handshake in a background goroutine to avoid blocking.
	go func() {
		// Use VMSelectServer to handle the handshake on the server end.
		// We use 0 for compressionLevel to simplify the test buffer inspection.
		bc, err := handshake.VMSelectServer(server, 0)
		if err != nil {
			// Using t.Errorf inside a goroutine is safe in modern Go.
			t.Errorf("failed to complete server-side handshake: %v", err)
			return
		}
		// The server-side bc is typically handled by the Server.processConn logic.
		// For the sake of this setup, we keep it open for the client to interact with.
		_ = bc
	}()

	// Perform the client-side handshake.
	bc, err := handshake.VMSelectClient(client, 0)
	if err != nil {
		t.Fatalf("failed to complete client-side handshake: %v", err)
	}

	// Return the initialized context and the server-side pipe for manual verification.
	return &vmselectRequestCtx{
		bc:      bc,
		sizeBuf: make([]byte, 8),
	}, server
}

func TestVmselectRequestCtx_MetadataAndEndResponse(t *testing.T) {
	// MaskMetadata uses the most significant bit (MSB) to indicate
	// whether the following payload is metadata instead of a data block.
	const MaskMetadata = uint64(1 << 63)

	t.Run("writeEndOfResponse_isPartial_true", func(t *testing.T) {
		client, server := net.Pipe()
		defer client.Close()
		defer server.Close()

		ctx, server := setupTestCtx(t)
		go func() {
			// writeEndOfResponse(true) should encode:
			// a metadata header with MSB=1 and zero payload,
			// indicating end of stream with partial=true
			err := ctx.writeEndOfResponse(true)
			if err != nil {
				t.Errorf("writeEndOfResponse failed: %v", err)
			}
			ctx.bc.Flush()
		}()

		headerBuf := make([]byte, 8)
		io.ReadFull(server, headerBuf)
		header := binary.BigEndian.Uint64(headerBuf)

		// MSB must be set → indicates metadata (partial flag)
		if header&MaskMetadata == 0 {
			t.Error("isPartial=true should set MSB to 1")
		}

		// No payload expected for end-of-response marker
		if header&^MaskMetadata != 0 {
			t.Errorf("End of response should have 0 size, got %d", header&^MaskMetadata)
		}
	})

	t.Run("writeEndOfResponse_isPartial_false", func(t *testing.T) {
		ctx, server := setupTestCtx(t)
		go func() {
			// writeEndOfResponse(false) should encode:
			// a zero header (uint64=0), which acts as EOF marker
			ctx.writeEndOfResponse(false)
			ctx.bc.Flush()
		}()

		headerBuf := make([]byte, 8)
		io.ReadFull(server, headerBuf)
		header := binary.BigEndian.Uint64(headerBuf)

		// header == 0 → EOF, full (non-partial) response
		if header != 0 {
			t.Errorf("Full response end should be 0, got %v", header)
		}
	})
}
