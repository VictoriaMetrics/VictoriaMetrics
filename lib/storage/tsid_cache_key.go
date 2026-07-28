package storage

import (
	"encoding/binary"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/cespare/xxhash/v2"
)

// Key modes for the storage/tsid cache: the metric name itself, or a fixed-size
// fingerprint of it plus a verifier stored in the cached value.
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

// Domain-separation suffixes for the second and the third xxHash64 output.
// The byte values are arbitrary; they only need to be distinct and stable.
var (
	tsidFPLane1Sep = [1]byte{0xF3}
	tsidFPLane2Sep = [1]byte{0xF4}
)

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

// marshalTSIDCacheKey returns a 128-bit fingerprint and a 64-bit verifier for
// metricNameRaw using domain-separated xxHash64 outputs.
//
// The fingerprint is written into the caller-provided kb to avoid allocations.
func marshalTSIDCacheKey(kb *[16]byte, metricNameRaw []byte) (key []byte, verifier uint64) {
	var d xxhash.Digest
	d.Reset()
	_, _ = d.Write(metricNameRaw)
	h0 := d.Sum64()
	_, _ = d.Write(tsidFPLane1Sep[:])
	h1 := d.Sum64()
	_, _ = d.Write(tsidFPLane2Sep[:])
	h2 := d.Sum64()
	binary.LittleEndian.PutUint64(kb[0:8], h0)
	binary.LittleEndian.PutUint64(kb[8:16], h1)
	return kb[:], h2
}
