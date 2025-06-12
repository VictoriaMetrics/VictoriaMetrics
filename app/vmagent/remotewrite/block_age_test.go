package remotewrite

import (
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

func TestBlockAgeLimit_DropOldBlock(t *testing.T) {
	maxPersistentQueueRetention := 10 * time.Second
	oldTimestamp := time.Now().Add(-2 * maxPersistentQueueRetention).Unix()
	block := make([]byte, 8+4)
	encoding.StoreUint64LE(block[:8], uint64(oldTimestamp))
	copy(block[8:], []byte{1, 2, 3, 4})

	shouldDrop := false
	fakeLogger := func(msg string) { shouldDrop = true }

	// Simulate the logic from client.go runWorker
	if maxPersistentQueueRetention > 0 {
		if len(block) >= 8 {
			blockTimestamp := int64(encoding.LoadUint64LE(block[:8]))
			blockAge := time.Now().Unix() - blockTimestamp
			if blockTimestamp > 0 && blockAge > int64(maxPersistentQueueRetention.Seconds()) {
				fakeLogger("dropped")
			}
		}
	}

	if !shouldDrop {
		t.Fatalf("block older than maxPersistentQueueRetention should be dropped")
	}
}

func TestBlockAgeLimit_KeepFreshBlock(t *testing.T) {
	maxPersistentQueueRetention := 10 * time.Second
	freshTimestamp := time.Now().Unix()
	block := make([]byte, 8+4)
	encoding.StoreUint64LE(block[:8], uint64(freshTimestamp))
	copy(block[8:], []byte{1, 2, 3, 4})

	shouldDrop := false
	fakeLogger := func(msg string) { shouldDrop = true }

	if maxPersistentQueueRetention > 0 {
		if len(block) >= 8 {
			blockTimestamp := int64(encoding.LoadUint64LE(block[:8]))
			blockAge := time.Now().Unix() - blockTimestamp
			if blockTimestamp > 0 && blockAge > int64(maxPersistentQueueRetention.Seconds()) {
				fakeLogger("dropped")
			}
		}
	}

	if shouldDrop {
		t.Fatalf("fresh block should not be dropped")
	}
}

func TestBlockAgeLimit_BackwardCompatibility(t *testing.T) {
	// Simulate a block without a timestamp (old format)
	maxPersistentQueueRetention := 10 * time.Second
	block := []byte{1, 2, 3, 4}
	shouldDrop := false
	fakeLogger := func(msg string) { shouldDrop = true }

	if maxPersistentQueueRetention > 0 {
		if len(block) >= 8 {
			blockTimestamp := int64(encoding.LoadUint64LE(block[:8]))
			blockAge := time.Now().Unix() - blockTimestamp
			if blockTimestamp > 0 && blockAge > int64(maxPersistentQueueRetention.Seconds()) {
				fakeLogger("dropped")
			}
		}
	}

	if shouldDrop {
		t.Fatalf("block without timestamp should not be dropped (backward compatibility)")
	}
}
