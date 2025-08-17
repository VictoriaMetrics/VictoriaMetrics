package consistenthash

import (
	"math"
	"math/rand"
	"testing"
)

func TestConsistentHash(t *testing.T) {
	r := rand.New(rand.NewSource(1))

	nodes := []string{
		"node1",
		"node2",
		"node3",
		"node4",
	}
	rh := NewConsistentHash(nodes, 0)

	keys := make([]uint64, 100000)
	for i := 0; i < len(keys); i++ {
		keys[i] = r.Uint64()
	}
	perIdxCounts := make([]int, len(nodes))
	keyIndexes := make([]int, len(keys))
	for i, k := range keys {
		idx := rh.GetNodeIdx(k, nil)
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
		idx := rh.GetNodeIdx(k, idxsExclude)
		perIdxCounts[idx]++
		if keyIndexes[i] != idx {
			indexMismatches++
		}
	}
	maxIndexMismatches := float64(len(keys)) / float64(len(nodes))
	if float64(indexMismatches) > maxIndexMismatches {
		t.Fatalf("too many index mismatches after excluding a node; got %d; want no more than %f", indexMismatches, maxIndexMismatches)
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
