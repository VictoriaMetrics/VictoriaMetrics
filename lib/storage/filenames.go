package storage

const (
	metaindexFilename  = "metaindex.bin"
	indexFilename      = "index.bin"
	valuesFilename     = "values.bin"
	timestampsFilename = "timestamps.bin"
	partsFilename      = "parts.json"
	metadataFilename   = "metadata.json"

	appliedRetentionFilename    = "appliedRetention.txt"
	resetCacheOnStartupFilename = "reset_cache_on_startup"

	tsidCacheFilename         = "metricName_tsid"
	metricIDCacheFilename     = "metricID_tsid"
	metricNameCacheFilename   = "metricID_metricName"
	prevHourMetricIDsFilename = "prev_hour_metric_ids"
	currHourMetricIDsFilename = "curr_hour_metric_ids"
	nextDayMetricIDsFilename  = "next_day_metric_ids_v2"
	metricNameTrackerFilename = "metric_usage_tracker"
)

const (
	smallDirname = "small"
	bigDirname   = "big"

	indexdbDirname   = "indexdb"
	dataDirname      = "data"
	metadataDirname  = "metadata"
	snapshotsDirname = "snapshots"
	cacheDirname     = "cache"
)
