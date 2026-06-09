package actions

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

// partitionFromPath extracts the partition name (in the YYYY_MM form) from the
// given canonical backup part path.
//
// It returns the partition name, whether the path points to per-partition
// indexdb data and whether the path belongs to a partition at all (ok).
//
// ok is false when the path doesn't belong to any partition, for example
// metadata files or the legacy global indexdb stored before per-partition
// indexdb was introduced. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7599 .
//
// The expected per-partition layout inside a backup is:
//
//	data/small/<YYYY_MM>/...
//	data/big/<YYYY_MM>/...
//	data/indexdb/<YYYY_MM>/...
func partitionFromPath(path string) (name string, isIndexDB bool, ok bool) {
	segs := strings.Split(path, "/")
	if len(segs) < 3 || segs[0] != "data" {
		return "", false, false
	}
	switch segs[1] {
	case "small", "big":
		if isPartitionName(segs[2]) {
			return segs[2], false, true
		}
	case "indexdb":
		if isPartitionName(segs[2]) {
			return segs[2], true, true
		}
	}
	return "", false, false
}

// isPartitionName returns true if s is a valid partition name in the YYYY_MM form.
func isPartitionName(s string) bool {
	_, err := time.Parse("2006_01", s)
	return err == nil
}

// filterPartsByPartitions returns the subset of parts that must be restored when
// -restorePartitions=re is set.
//
// All non-partition parts (metadata, legacy global indexdb, etc.) are always kept,
// since they are small and required for a consistent storage. Parts belonging to
// partitions are kept only if the partition name matches re.
//
// An error is returned when:
//   - no partition in the backup matches re;
//   - at least one matched partition doesn't contain per-partition indexdb data.
//     This indicates an old-style backup made before per-partition indexdb was
//     introduced (https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7599),
//     which cannot be safely restored partition by partition.
func filterPartsByPartitions(parts []common.Part, re *regexp.Regexp) ([]common.Part, error) {
	dataPartitions := make(map[string]bool)
	indexDBPartitions := make(map[string]bool)
	for _, p := range parts {
		name, isIndexDB, ok := partitionFromPath(p.Path)
		if !ok {
			continue
		}
		if isIndexDB {
			indexDBPartitions[name] = true
		} else {
			dataPartitions[name] = true
		}
	}

	allPartitions := make(map[string]bool, len(dataPartitions)+len(indexDBPartitions))
	for name := range dataPartitions {
		allPartitions[name] = true
	}
	for name := range indexDBPartitions {
		allPartitions[name] = true
	}

	var matched []string
	for name := range allPartitions {
		if re.MatchString(name) {
			matched = append(matched, name)
		}
	}
	if len(matched) == 0 {
		return nil, fmt.Errorf("no partitions in the backup match -restorePartitions=%q; available partitions: %s",
			re.String(), strings.Join(sortedKeys(allPartitions), ", "))
	}

	var missingIndexDB []string
	for _, name := range matched {
		if !indexDBPartitions[name] {
			missingIndexDB = append(missingIndexDB, name)
		}
	}
	if len(missingIndexDB) > 0 {
		sort.Strings(missingIndexDB)
		return nil, fmt.Errorf("cannot restore partitions [%s] selected by -restorePartitions=%q, since they do not contain per-partition indexdb data; "+
			"this usually means the backup was created by an old VictoriaMetrics version, which stores indexdb globally instead of per-partition; "+
			"such backups can only be restored in full, i.e. without -restorePartitions",
			strings.Join(missingIndexDB, ", "), re.String())
	}

	filtered := make([]common.Part, 0, len(parts))
	for _, p := range parts {
		name, _, ok := partitionFromPath(p.Path)
		if !ok {
			// Always keep non-partition parts (metadata, legacy global indexdb, etc.).
			filtered = append(filtered, p)
			continue
		}
		if re.MatchString(name) {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
