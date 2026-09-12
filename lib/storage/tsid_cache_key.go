package storage

import (
	"encoding/binary"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/cespare/xxhash/v2"
)

// Key modes for the storage/tsid cache: the metric name itself, or a fixed-size
// 128-bit fingerprint of it.
const (
	tsidCacheKeyModeMetricName  = "metricName"
	tsidCacheKeyModeFingerprint = "fingerprint"
)

// tsidCacheFingerprintKey is read on the ingestion hot path.
//
// It is set once by SetTSIDCacheKeyMode before MustOpenStorage and is read-only
// afterwards, so no synchronization is required (same pattern as
// maxTSIDCacheSize).
var tsidCacheFingerprintKey = false

// magicSuffixForHash separates the two xxHash64 outputs of the fingerprint.
var magicSuffixForHash = []byte("magic!")

// SetTSIDCacheKeyMode selects the key mode for the storage/tsid cache.
//
// It must be called before MustOpenStorage. It panics on an unknown mode.
// Mirrors SetTSIDCacheSize.
func SetTSIDCacheKeyMode(mode string) {
	switch mode {
	case tsidCacheKeyModeMetricName:
		tsidCacheFingerprintKey = false
	case tsidCacheKeyModeFingerprint:
		tsidCacheFingerprintKey = true
	default:
		logger.Panicf("FATAL: unsupported -storage.tsidCacheKeyMode=%q; supported values: %q, %q",
			mode, tsidCacheKeyModeMetricName, tsidCacheKeyModeFingerprint)
	}
}

// tsidCacheFilenameForMode returns the on-disk cache filename for the active
// key mode. The two modes persist under different filenames, so a mode switch
// cannot load a cache file written with the other key format.
func tsidCacheFilenameForMode() string {
	if tsidCacheFingerprintKey {
		return tsidCacheFPFilename
	}
	return tsidCacheFilename
}

// marshalTSIDCacheKey writes a 128-bit fingerprint of metricNameRaw into kb and
// returns it. It mirrors VictoriaLogs lib/logstorage.hash128 and accepts the
// possibility of a theoretical hash collision.
//
// Changing the derivation or the cached value format requires bumping
// tsidCacheFPFilename.
func marshalTSIDCacheKey(kb *[16]byte, metricNameRaw []byte) []byte {
	h := getHasher()
	_, _ = h.Write(metricNameRaw)
	hi := h.Sum64()
	_, _ = h.Write(magicSuffixForHash)
	lo := h.Sum64()
	putHasher(h)

	binary.LittleEndian.PutUint64(kb[0:8], hi)
	binary.LittleEndian.PutUint64(kb[8:16], lo)
	return kb[:]
}

func getHasher() *xxhash.Digest {
	v := hasherPool.Get()
	if v == nil {
		return xxhash.New()
	}
	return v.(*xxhash.Digest)
}

func putHasher(h *xxhash.Digest) {
	h.Reset()
	hasherPool.Put(h)
}

var hasherPool sync.Pool
