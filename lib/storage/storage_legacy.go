package storage

import (
	"fmt"
	"math"
	"sort"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
)

func (s *Storage) registerMetricNamesLegacy(qt *querytracer.Tracer, mrs []MetricRow) {
	qt = qt.NewChild("registering %d series", len(mrs))
	defer qt.Done()
	var metricNameBuf []byte
	var genTSID generationTSID
	mn := GetMetricName()
	defer PutMetricName(mn)

	var newSeriesCount uint64
	var seriesRepopulated uint64

	idb := s.idb()
	generation := idb.generation
	is := idb.getIndexSearch(noDeadline)
	defer idb.putIndexSearch(is)
	var firstWarn error
	for i := range mrs {
		mr := &mrs[i]
		date := uint64(mr.Timestamp) / msecPerDay
		if s.getTSIDFromCache(&genTSID, mr.MetricNameRaw) {
			// Fast path - mr.MetricNameRaw has been already registered in the current idb.
			if !s.registerSeriesCardinality(genTSID.TSID.MetricID, mr.MetricNameRaw) {
				// Skip row, since it exceeds cardinality limit
				continue
			}
			if genTSID.generation < generation {
				// The found TSID is from the previous indexdb. Create it in the current indexdb.

				if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
					// Do not stop adding rows on error - just skip invalid row.
					// This guarantees that invalid rows don't prevent
					// from adding valid rows into the storage.
					if firstWarn == nil {
						firstWarn = fmt.Errorf("cannot umarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
					}
					s.invalidRawMetricNames.Add(1)
					continue
				}
				mn.sortTags()

				createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
				genTSID.generation = generation
				s.putSeriesToCacheLegacy(mr.MetricNameRaw, &genTSID, date)
				seriesRepopulated++
			} else if !s.dateMetricIDCache.Has(generation, date, genTSID.TSID.MetricID) {
				if !is.hasDateMetricIDNoExtDB(date, genTSID.TSID.MetricID) {
					if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
						if firstWarn == nil {
							firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
						}
						continue
					}
					mn.sortTags()
					is.createPerDayIndexes(date, &genTSID.TSID, mn)
				}
				s.dateMetricIDCache.Set(generation, date, genTSID.TSID.MetricID)
			}
			continue
		}

		// Slow path - search TSID for the given metricName in indexdb.

		// Construct canonical metric name - it is used below.
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			// Do not stop adding rows on error - just skip invalid row.
			// This guarantees that invalid rows don't prevent
			// from adding valid rows into the storage.
			if firstWarn == nil {
				firstWarn = fmt.Errorf("cannot umarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
			}
			s.invalidRawMetricNames.Add(1)
			continue
		}
		mn.sortTags()
		metricNameBuf = mn.Marshal(metricNameBuf[:0])

		if is.getTSIDByMetricNameLegacy(&genTSID, metricNameBuf, date) {
			// Slower path - the TSID has been found in indexdb.

			if !s.registerSeriesCardinality(genTSID.TSID.MetricID, mr.MetricNameRaw) {
				// Skip the row, since it exceeds the configured cardinality limit.
				continue
			}

			if genTSID.generation < generation {
				// The found TSID is from the previous indexdb. Create it in the current indexdb.
				createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
				genTSID.generation = generation
				seriesRepopulated++
			}
			s.putSeriesToCacheLegacy(mr.MetricNameRaw, &genTSID, date)
			continue
		}

		// Slowest path - there is no TSID in indexdb for the given mr.MetricNameRaw. Create it.
		generateTSID(&genTSID.TSID, mn)

		if !s.registerSeriesCardinality(genTSID.TSID.MetricID, mr.MetricNameRaw) {
			// Skip the row, since it exceeds the configured cardinality limit.
			continue
		}

		// Schedule creating TSID indexes instead of creating them synchronously.
		// This should keep stable the ingestion rate when new time series are ingested.
		createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
		genTSID.generation = generation
		s.putSeriesToCacheLegacy(mr.MetricNameRaw, &genTSID, date)
		newSeriesCount++
	}

	s.newTimeseriesCreated.Add(newSeriesCount)
	s.timeseriesRepopulated.Add(seriesRepopulated)

	// There is no need in pre-filling idbNext here, since RegisterMetricNames() is rarely called.
	// So it is OK to register metric names in blocking manner after indexdb rotation.

	if firstWarn != nil {
		logger.Warnf("cannot create some metrics: %s", firstWarn)
	}
}

func (s *Storage) addLegacy(rows []rawRow, dstMrs []*MetricRow, mrs []MetricRow, precisionBits uint8) int {
	idb := s.idb()
	generation := idb.generation
	is := idb.getIndexSearch(noDeadline)
	defer idb.putIndexSearch(is)

	mn := GetMetricName()
	defer PutMetricName(mn)

	var (
		// These vars are used for speeding up bulk imports of multiple adjacent rows for the same metricName.
		prevTSID          TSID
		prevMetricNameRaw []byte
	)
	var metricNameBuf []byte

	var slowInsertsCount uint64
	var newSeriesCount uint64
	var seriesRepopulated uint64

	minTimestamp, maxTimestamp := s.tb.getMinMaxTimestamps()

	var genTSID generationTSID

	// Log only the first error, since it has no sense in logging all errors.
	var firstWarn error

	j := 0
	for i := range mrs {
		mr := &mrs[i]
		if math.IsNaN(mr.Value) {
			if !decimal.IsStaleNaN(mr.Value) {
				// Skip NaNs other than Prometheus staleness marker, since the underlying encoding
				// doesn't know how to work with them.
				continue
			}
		}
		if mr.Timestamp < minTimestamp {
			// Skip rows with too small timestamps outside the retention.
			if firstWarn == nil {
				metricName := getUserReadableMetricName(mr.MetricNameRaw)
				firstWarn = fmt.Errorf("cannot insert row with too small timestamp %d outside the retention; minimum allowed timestamp is %d; "+
					"probably you need updating -retentionPeriod command-line flag; metricName: %s",
					mr.Timestamp, minTimestamp, metricName)
			}
			s.tooSmallTimestampRows.Add(1)
			continue
		}
		if mr.Timestamp > maxTimestamp {
			// Skip rows with too big timestamps significantly exceeding the current time.
			if firstWarn == nil {
				metricName := getUserReadableMetricName(mr.MetricNameRaw)
				firstWarn = fmt.Errorf("cannot insert row with too big timestamp %d exceeding the current time; maximum allowed timestamp is %d; metricName: %s",
					mr.Timestamp, maxTimestamp, metricName)
			}
			s.tooBigTimestampRows.Add(1)
			continue
		}
		dstMrs[j] = mr
		r := &rows[j]
		j++
		r.Timestamp = mr.Timestamp
		r.Value = mr.Value
		r.PrecisionBits = precisionBits

		// Search for TSID for the given mr.MetricNameRaw and store it at r.TSID.
		if string(mr.MetricNameRaw) == string(prevMetricNameRaw) {
			// Fast path - the current mr contains the same metric name as the previous mr, so it contains the same TSID.
			// This path should trigger on bulk imports when many rows contain the same MetricNameRaw.
			r.TSID = prevTSID
			continue
		}
		if s.getTSIDFromCache(&genTSID, mr.MetricNameRaw) {
			// Fast path - the TSID for the given mr.MetricNameRaw has been found in cache and isn't deleted.
			// There is no need in checking whether r.TSID.MetricID is deleted, since tsidCache doesn't
			// contain MetricName->TSID entries for deleted time series.
			// See Storage.DeleteSeries code for details.

			if !s.registerSeriesCardinality(r.TSID.MetricID, mr.MetricNameRaw) {
				// Skip row, since it exceeds cardinality limit
				j--
				continue
			}
			r.TSID = genTSID.TSID
			prevTSID = r.TSID
			prevMetricNameRaw = mr.MetricNameRaw

			if genTSID.generation < generation {
				// The found TSID is from the previous indexdb. Create it in the current indexdb.
				date := uint64(r.Timestamp) / msecPerDay

				if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
					if firstWarn == nil {
						firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
					}
					j--
					s.invalidRawMetricNames.Add(1)
					continue
				}
				mn.sortTags()

				createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
				genTSID.generation = generation
				s.putSeriesToCacheLegacy(mr.MetricNameRaw, &genTSID, date)
				seriesRepopulated++
				slowInsertsCount++
			}
			continue
		}

		// Slow path - the TSID for the given mr.MetricNameRaw is missing in the cache.
		slowInsertsCount++

		date := uint64(r.Timestamp) / msecPerDay

		// Construct canonical metric name - it is used below.
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			if firstWarn == nil {
				firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
			}
			j--
			s.invalidRawMetricNames.Add(1)
			continue
		}
		mn.sortTags()
		metricNameBuf = mn.Marshal(metricNameBuf[:0])

		// Search for TSID for the given mr.MetricNameRaw in the indexdb.
		if is.getTSIDByMetricNameLegacy(&genTSID, metricNameBuf, date) {
			// Slower path - the TSID has been found in indexdb.

			if !s.registerSeriesCardinality(genTSID.TSID.MetricID, mr.MetricNameRaw) {
				// Skip the row, since it exceeds the configured cardinality limit.
				j--
				continue
			}

			if genTSID.generation < generation {
				// The found TSID is from the previous indexdb. Create it in the current indexdb.
				createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
				genTSID.generation = generation
				seriesRepopulated++
			}
			s.putSeriesToCacheLegacy(mr.MetricNameRaw, &genTSID, date)

			r.TSID = genTSID.TSID
			prevTSID = genTSID.TSID
			prevMetricNameRaw = mr.MetricNameRaw
			continue
		}

		// Slowest path - the TSID for the given mr.MetricNameRaw isn't found in indexdb. Create it.
		generateTSID(&genTSID.TSID, mn)

		if !s.registerSeriesCardinality(genTSID.TSID.MetricID, mr.MetricNameRaw) {
			// Skip the row, since it exceeds the configured cardinality limit.
			j--
			continue
		}

		createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
		genTSID.generation = generation
		s.putSeriesToCacheLegacy(mr.MetricNameRaw, &genTSID, date)
		newSeriesCount++

		r.TSID = genTSID.TSID
		prevTSID = r.TSID
		prevMetricNameRaw = mr.MetricNameRaw

		if logNewSeries {
			logger.Infof("new series created: %s", mn.String())
		}
	}

	s.slowRowInserts.Add(slowInsertsCount)
	s.newTimeseriesCreated.Add(newSeriesCount)
	s.timeseriesRepopulated.Add(seriesRepopulated)

	dstMrs = dstMrs[:j]
	rows = rows[:j]

	if err := s.prefillNextIndexDBLegacy(rows, dstMrs); err != nil {
		if firstWarn == nil {
			firstWarn = fmt.Errorf("cannot prefill next indexdb: %w", err)
		}
	}
	if err := s.updatePerDateDataLegacy(rows, dstMrs); err != nil {
		if firstWarn == nil {
			firstWarn = fmt.Errorf("cannot not update per-day index: %w", err)
		}
	}

	if firstWarn != nil {
		storageAddRowsLogger.Warnf("warn occurred during rows addition: %s", firstWarn)
	}

	s.tb.MustAddRows(rows)

	return len(rows)
}

func (s *Storage) putSeriesToCacheLegacy(metricNameRaw []byte, genTSID *generationTSID, date uint64) {
	// Store the TSID for the current indexdb into cache,
	// so future rows for that TSID are ingested via fast path.
	s.putTSIDToCache(genTSID, metricNameRaw)

	// Register the (generation, date, metricID) entry in the cache,
	// so next time the entry is found there instead of searching for it in the indexdb.
	s.dateMetricIDCache.Set(genTSID.generation, date, genTSID.TSID.MetricID)
}

func (s *Storage) prefillNextIndexDBLegacy(rows []rawRow, mrs []*MetricRow) error {
	d := s.nextRetentionSeconds()
	if d >= 3600 {
		// Fast path: nothing to pre-fill because it is too early.
		// The pre-fill is started during the last hour before the indexdb rotation.
		return nil
	}

	// Slower path: less than hour left for the next indexdb rotation.
	// Pre-populate idbNext with the increasing probability until the rotation.
	// The probability increases from 0% to 100% proportioinally to d=[3600 .. 0].
	pMin := float64(d) / 3600

	idbNext := s.idbNext.Load()
	generation := idbNext.generation
	isNext := idbNext.getIndexSearch(noDeadline)
	defer idbNext.putIndexSearch(isNext)

	var firstError error
	var genTSID generationTSID
	mn := GetMetricName()
	defer PutMetricName(mn)

	timeseriesPreCreated := uint64(0)
	for i := range rows {
		r := &rows[i]
		p := float64(uint32(fastHashUint64(r.TSID.MetricID))) / (1 << 32)
		if p < pMin {
			// Fast path: it is too early to pre-fill indexes for the given MetricID.
			continue
		}

		// Check whether the given MetricID is already present in dateMetricIDCache.
		date := uint64(r.Timestamp) / msecPerDay
		metricID := r.TSID.MetricID
		if s.dateMetricIDCache.Has(generation, date, metricID) {
			// Indexes are already pre-filled.
			continue
		}

		// Check whether the given (date, metricID) is already present in idbNext.
		if isNext.hasDateMetricIDNoExtDB(date, metricID) {
			// Indexes are already pre-filled at idbNext.
			//
			// Register the (generation, date, metricID) entry in the cache,
			// so next time the entry is found there instead of searching for it in the indexdb.
			s.dateMetricIDCache.Set(generation, date, metricID)
			continue
		}

		// Slow path: pre-fill indexes in idbNext.
		metricNameRaw := mrs[i].MetricNameRaw
		if err := mn.UnmarshalRaw(metricNameRaw); err != nil {
			if firstError == nil {
				firstError = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", metricNameRaw, err)
			}
			s.invalidRawMetricNames.Add(1)
			continue
		}
		mn.sortTags()

		createAllIndexesForMetricName(isNext, mn, &r.TSID, date)
		genTSID.TSID = r.TSID
		genTSID.generation = generation
		s.putSeriesToCacheLegacy(metricNameRaw, &genTSID, date)
		timeseriesPreCreated++
	}
	s.timeseriesPreCreated.Add(timeseriesPreCreated)

	return firstError
}

func (s *Storage) updatePerDateDataLegacy(rows []rawRow, mrs []*MetricRow) error {
	var date uint64
	var hour uint64
	var prevTimestamp int64
	var (
		// These vars are used for speeding up bulk imports when multiple adjacent rows
		// contain the same (metricID, date) pairs.
		prevDate     uint64
		prevMetricID uint64
	)

	idb := s.idb()
	generation := idb.generation

	hm := s.currHourMetricIDs.Load()
	hmPrev := s.prevHourMetricIDs.Load()
	hmPrevDate := hmPrev.hour / 24
	nextDayMetricIDs := &s.nextDayMetricIDs.Load().v
	ts := fasttime.UnixTimestamp()
	// Start pre-populating the next per-day inverted index during the last hour of the current day.
	// pMin linearly increases from 0 to 1 during the last hour of the day.
	pMin := (float64(ts%(3600*24)) / 3600) - 23
	type pendingDateMetricID struct {
		date uint64
		tsid *TSID
		mr   *MetricRow
	}
	var pendingDateMetricIDs []pendingDateMetricID
	var pendingNextDayMetricIDs []uint64
	var pendingHourEntries []uint64
	for i := range rows {
		r := &rows[i]
		if r.Timestamp != prevTimestamp {
			date = uint64(r.Timestamp) / msecPerDay
			hour = uint64(r.Timestamp) / msecPerHour
			prevTimestamp = r.Timestamp
		}
		metricID := r.TSID.MetricID
		if metricID == prevMetricID && date == prevDate {
			// Fast path for bulk import of multiple rows with the same (date, metricID) pairs.
			continue
		}
		prevDate = date
		prevMetricID = metricID
		if hour == hm.hour {
			// The row belongs to the current hour. Check for the current hour cache.
			if hm.m.Has(metricID) {
				// Fast path: the metricID is in the current hour cache.
				// This means the metricID has been already added to per-day inverted index.

				// Gradually pre-populate per-day inverted index for the next day during the last hour of the current day.
				// This should reduce CPU usage spike and slowdown at the beginning of the next day
				// when entries for all the active time series must be added to the index.
				// This should address https://github.com/VictoriaMetrics/VictoriaMetrics/issues/430 .
				if pMin > 0 {
					p := float64(uint32(fastHashUint64(metricID))) / (1 << 32)
					if p < pMin && !nextDayMetricIDs.Has(metricID) {
						pendingDateMetricIDs = append(pendingDateMetricIDs, pendingDateMetricID{
							date: date + 1,
							tsid: &r.TSID,
							mr:   mrs[i],
						})
						pendingNextDayMetricIDs = append(pendingNextDayMetricIDs, metricID)
					}
				}
				continue
			}
			pendingHourEntries = append(pendingHourEntries, metricID)
			if date == hmPrevDate && hmPrev.m.Has(metricID) {
				// The metricID is already registered for the current day on the previous hour.
				continue
			}
		}

		// Slower path: check global cache for (generation, date, metricID) entry.
		if s.dateMetricIDCache.Has(generation, date, metricID) {
			continue
		}
		// Slow path: store the (date, metricID) entry in the indexDB.
		pendingDateMetricIDs = append(pendingDateMetricIDs, pendingDateMetricID{
			date: date,
			tsid: &r.TSID,
			mr:   mrs[i],
		})
	}
	if len(pendingNextDayMetricIDs) > 0 {
		s.pendingNextDayMetricIDsLock.Lock()
		s.pendingNextDayMetricIDs.AddMulti(pendingNextDayMetricIDs)
		s.pendingNextDayMetricIDsLock.Unlock()
	}
	if len(pendingHourEntries) > 0 {
		s.pendingHourEntriesLock.Lock()
		s.pendingHourEntries.AddMulti(pendingHourEntries)
		s.pendingHourEntriesLock.Unlock()
	}
	if len(pendingDateMetricIDs) == 0 {
		// Fast path - there are no new (date, metricID) entries.
		return nil
	}

	// Slow path - add new (date, metricID) entries to indexDB.

	s.slowPerDayIndexInserts.Add(uint64(len(pendingDateMetricIDs)))
	// Sort pendingDateMetricIDs by (date, metricID) in order to speed up `is` search in the loop below.
	sort.Slice(pendingDateMetricIDs, func(i, j int) bool {
		a := pendingDateMetricIDs[i]
		b := pendingDateMetricIDs[j]
		if a.date != b.date {
			return a.date < b.date
		}
		return a.tsid.MetricID < b.tsid.MetricID
	})

	is := idb.getIndexSearch(noDeadline)
	defer idb.putIndexSearch(is)

	var firstError error
	dateMetricIDsForCache := make([]dateMetricID, 0, len(pendingDateMetricIDs))
	mn := GetMetricName()
	for _, dmid := range pendingDateMetricIDs {
		date := dmid.date
		metricID := dmid.tsid.MetricID
		if !is.hasDateMetricIDNoExtDB(date, metricID) {
			// The (date, metricID) entry is missing in the indexDB. Add it there together with per-day indexes.
			// It is OK if the (date, metricID) entry is added multiple times to indexdb
			// by concurrent goroutines.
			if err := mn.UnmarshalRaw(dmid.mr.MetricNameRaw); err != nil {
				if firstError == nil {
					firstError = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", dmid.mr.MetricNameRaw, err)
				}
				s.invalidRawMetricNames.Add(1)
				continue
			}
			mn.sortTags()
			is.createPerDayIndexes(date, dmid.tsid, mn)
		}
		dateMetricIDsForCache = append(dateMetricIDsForCache, dateMetricID{
			date:     date,
			metricID: metricID,
		})
	}
	PutMetricName(mn)
	// The (date, metricID) entries must be added to cache only after they have been successfully added to indexDB.
	s.dateMetricIDCache.Store(generation, dateMetricIDsForCache)
	return firstError
}
