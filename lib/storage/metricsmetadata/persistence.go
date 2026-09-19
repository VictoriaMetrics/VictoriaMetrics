package metricsmetadata

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

// This format is private to the cache. Row's RPC encoding must not change.
const cacheMagic = "VMMETA01"
const cacheHeaderSize = len(cacheMagic) + 8 + 4

func (s *Storage) load() error {
	if s.cachePath == "" || s.maxSizeBytes <= 0 {
		return nil
	}
	f, err := os.Open(s.cachePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot open cache: %w", err)
	}
	defer fs.MustClose(f)

	// The encoded rows are smaller than their in-memory size. Limit reads even
	// when the file is corrupt or was saved with a larger memory budget.
	limit := int64(s.maxSizeBytes) + int64(cacheHeaderSize)
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return fmt.Errorf("cannot read cache: %w", err)
	}
	if int64(len(data)) > limit {
		return fmt.Errorf("cache exceeds the current size limit")
	}
	return s.unmarshalCache(data, fasttime.UnixTimestamp())
}

func (s *Storage) unmarshalCache(data []byte, now uint64) error {
	if len(data) < cacheHeaderSize || string(data[:len(cacheMagic)]) != cacheMagic {
		return fmt.Errorf("unsupported or truncated cache header")
	}
	size := binary.BigEndian.Uint64(data[len(cacheMagic):])
	checksum := binary.BigEndian.Uint32(data[len(cacheMagic)+8:])
	data = data[cacheHeaderSize:]
	if size != uint64(len(data)) || checksum != crc32.ChecksumIEEE(data) {
		return fmt.Errorf("invalid cache size or checksum")
	}
	// Validate every record before adding any rows. Neither malformed lengths
	// nor truncated records should leave partially restored state.
	for tail := data; len(tail) > 0; {
		_, next, err := unmarshalCacheRow(tail)
		if err != nil {
			return err
		}
		tail = next
	}
	for len(data) > 0 {
		r, tail, _ := unmarshalCacheRow(data)
		data = tail
		// A wall-clock rollback doesn't expire a row. Keep its original time
		// and subtract only when the age is nonnegative.
		if r.lastWriteTime <= now && now-r.lastWriteTime >= uint64(metadataExpireDuration/time.Second) {
			continue
		}
		b := s.buckets[xxhash.Sum64(r.MetricFamilyName)%bucketsCount]
		b.add(&r, r.lastWriteTime)
	}
	return nil
}

func unmarshalCacheRow(data []byte) (Row, []byte, error) {
	// timestamp, accountID, projectID, type, and three string lengths.
	if len(data) < 32 {
		return Row{}, nil, fmt.Errorf("truncated cache record")
	}
	r := Row{
		lastWriteTime: binary.BigEndian.Uint64(data),
		AccountID:     binary.BigEndian.Uint32(data[8:]),
		ProjectID:     binary.BigEndian.Uint32(data[12:]),
		Type:          prompb.MetricType(binary.BigEndian.Uint32(data[16:])),
	}
	n := uint64(binary.BigEndian.Uint32(data[20:]))
	h := uint64(binary.BigEndian.Uint32(data[24:]))
	u := uint64(binary.BigEndian.Uint32(data[28:]))
	data = data[32:]
	if n+h+u > uint64(len(data)) {
		return Row{}, nil, fmt.Errorf("invalid cache record lengths")
	}
	r.MetricFamilyName = data[:n]
	r.Help = data[n : n+h]
	r.Unit = data[n+h : n+h+u]
	return r, data[n+h+u:], nil
}

func (s *Storage) mustSave() {
	if s.cachePath == "" {
		return
	}
	data := s.marshalCache(fasttime.UnixTimestamp())
	fs.MustMkdirIfNotExist(filepath.Dir(s.cachePath))
	fs.MustWriteAtomic(s.cachePath, data, true)
}

func (s *Storage) marshalCache(now uint64) []byte {
	data := make([]byte, cacheHeaderSize)
	copy(data, cacheMagic)
	for _, b := range s.buckets {
		b.mu.Lock()
		// Copy and sort while holding the lock: ingestion can update timestamps
		// even on otherwise immutable rows. Never sort the eviction heap itself.
		rows := append([]*Row(nil), b.lwh...)
		sortRows(rows)
		for _, r := range rows {
			if r.lastWriteTime <= now && now-r.lastWriteTime >= uint64(metadataExpireDuration/time.Second) {
				continue
			}
			data = binary.BigEndian.AppendUint64(data, r.lastWriteTime)
			data = binary.BigEndian.AppendUint32(data, r.AccountID)
			data = binary.BigEndian.AppendUint32(data, r.ProjectID)
			data = binary.BigEndian.AppendUint32(data, uint32(r.Type))
			data = binary.BigEndian.AppendUint32(data, uint32(len(r.MetricFamilyName)))
			data = binary.BigEndian.AppendUint32(data, uint32(len(r.Help)))
			data = binary.BigEndian.AppendUint32(data, uint32(len(r.Unit)))
			data = append(data, r.MetricFamilyName...)
			data = append(data, r.Help...)
			data = append(data, r.Unit...)
		}
		b.mu.Unlock()
	}
	binary.BigEndian.PutUint64(data[len(cacheMagic):], uint64(len(data)-cacheHeaderSize))
	binary.BigEndian.PutUint32(data[len(cacheMagic)+8:], crc32.ChecksumIEEE(data[cacheHeaderSize:]))
	return data
}

func (s *Storage) loadOrReset() {
	if err := s.load(); err != nil {
		logger.Errorf("cannot load metrics metadata cache at %q: %s; starting with empty cache", s.cachePath, err)
	}
}
