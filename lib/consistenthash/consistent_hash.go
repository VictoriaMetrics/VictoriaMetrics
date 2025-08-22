package consistenthash

import (
	"github.com/cespare/xxhash/v2"
)

// ConsistentHash See the following docs:
// - https://www.eecs.umich.edu/techreports/cse/96/CSE-TR-316-96.pdf
// - https://github.com/dgryski/go-rendezvous
// - https://dgryski.medium.com/consistent-hashing-algorithmic-tradeoffs-ef6b8e2fcae8
type ConsistentHash struct {
	hashSeed   uint64
	nodeHashes []uint64
}

// NewConsistentHash creates a consistent hash based on the nodes.
func NewConsistentHash(nodes []string, hashSeed uint64) *ConsistentHash {
	nodeHashes := make([]uint64, len(nodes))
	for i, node := range nodes {
		nodeHashes[i] = xxhash.Sum64([]byte(node))
	}
	return &ConsistentHash{
		hashSeed:   hashSeed,
		nodeHashes: nodeHashes,
	}
}

// GetNodeIdx returns the node index that the input hash value should belong to.
func (rh *ConsistentHash) GetNodeIdx(h uint64, excludeIdxs []int) int {
	var mMax uint64
	var idx int
	h ^= rh.hashSeed

	if len(excludeIdxs) == len(rh.nodeHashes) {
		// All the nodes are excluded. Treat this case as no nodes are excluded.
		// This is better from load-balancing PoV than selecting some static node.
		excludeIdxs = nil
	}

next:
	for i, nh := range rh.nodeHashes {
		for _, j := range excludeIdxs {
			if i == j {
				continue next
			}
		}
		if m := fastHashUint64(nh ^ h); m > mMax {
			mMax = m
			idx = i
		}
	}
	return idx
}

func fastHashUint64(x uint64) uint64 {
	x ^= x >> 12 // a
	x ^= x << 25 // b
	x ^= x >> 27 // c
	return x * 2685821657736338717
}
