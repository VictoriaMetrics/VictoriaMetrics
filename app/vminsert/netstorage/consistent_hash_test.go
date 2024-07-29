package netstorage

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cespare/xxhash/v2"
)

func TestConsistentHash(t *testing.T) {
	r := rand.New(rand.NewSource(1))

	nodes := []uint64{
		xxhash.Sum64String("node1"),
		xxhash.Sum64String("node2"),
		xxhash.Sum64String("node3"),
		xxhash.Sum64String("node4"),
	}
	rh := newConsistentHash(nodes, 0)

	keys := make([]uint64, 100000)
	for i := 0; i < len(keys); i++ {
		keys[i] = r.Uint64()
	}
	perIdxCounts := make([]int, len(nodes))
	keyIndexes := make([]int, len(keys))
	for i, k := range keys {
		idx := rh.getNodeIdx(k, nil)
		perIdxCounts[idx]++
		keyIndexes[i] = idx
	}
	// verify that the number of selected node indexes per each node is roughly the same
	expectedPerIdxCount := float64(len(keys)) / float64(len(nodes))
	for _, perIdxCount := range perIdxCounts {
		if p := math.Abs(float64(perIdxCount)-expectedPerIdxCount) / expectedPerIdxCount; p > 0.005 {
			t.Fatalf("uneven number of per-index items %f: %d", p, perIdxCounts)
		}
	}
	// Ignore a single node and verify that the selection for the remaining nodes is even
	perIdxCounts = make([]int, len(nodes))
	idxsExclude := []int{1}
	indexMismatches := 0
	for i, k := range keys {
		idx := rh.getNodeIdx(k, idxsExclude)
		perIdxCounts[idx]++
		if keyIndexes[i] != idx {
			indexMismatches++
		}
	}
	maxIndexMismatches := float64(len(keys)) / float64(len(nodes))
	if float64(indexMismatches) > maxIndexMismatches {
		t.Fatalf("too many index mismtaches after excluding a node; got %d; want no more than %f", indexMismatches, maxIndexMismatches)
	}
	expectedPerIdxCount = float64(len(keys)) / float64(len(nodes)-1)
	for i, perIdxCount := range perIdxCounts {
		if i == idxsExclude[0] {
			if perIdxCount != 0 {
				t.Fatalf("unexpected non-zero items for excluded index %d: %d items", idxsExclude[0], perIdxCount)
			}
			continue
		}
		if p := math.Abs(float64(perIdxCount)-expectedPerIdxCount) / expectedPerIdxCount; p > 0.005 {
			t.Fatalf("uneven number of per-index items %f: %d", p, perIdxCounts)
		}
	}
}
