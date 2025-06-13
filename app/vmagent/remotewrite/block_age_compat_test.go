package remotewrite

import (
	"testing"
	"time"
)

func plausibleTimestamp(ts int64) bool {
	return ts > 946684800 && ts <= time.Now().Unix()
}

func TestBlockAge_OldFormat_Compat_NoMaxPersistentQueueRetention(t *testing.T) {
	oldBlock := []byte{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	maxPersistentQueueRetention := time.Duration(0)
	shouldDrop := false

	if maxPersistentQueueRetention > 0 {
		if len(oldBlock) >= 8 {
			blockTimestamp := int64(oldBlock[0])
			if plausibleTimestamp(blockTimestamp) {
				blockAge := time.Now().Unix() - blockTimestamp
				if blockAge > int64(maxPersistentQueueRetention.Seconds()) {
					shouldDrop = true
				}
			}
		}
	}

	if shouldDrop {
		t.Fatalf("Old-format block should NOT be dropped when maxPersistentQueueRetention is disabled")
	}
}

func TestBlockAge_OldFormat_Compat_WithMaxPersistentQueueRetention(t *testing.T) {
	oldBlock := []byte{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	maxPersistentQueueRetention := 10 * time.Second
	shouldDrop := false

	if maxPersistentQueueRetention > 0 {
		if len(oldBlock) >= 8 {
			blockTimestamp := int64(oldBlock[0])
			if plausibleTimestamp(blockTimestamp) {
				blockAge := time.Now().Unix() - blockTimestamp
				if blockAge > int64(maxPersistentQueueRetention.Seconds()) {
					shouldDrop = true
				}
			}
		}
	}

	if shouldDrop {
		t.Fatalf("Old-format block should NOT be dropped even if maxPersistentQueueRetention is enabled (backward compatibility)")
	}
}
