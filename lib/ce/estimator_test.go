package ce

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ShardIterator(t *testing.T) {

	n := 10
	ce := NewCardinalityEstimatorWithSettings(n, math.MaxUint64, 1)

	correct := []int{}
	for i := range n {
		correct = append(correct, i)
	}

	for range 10000 {

		seq := []int{}
		for i := range ce.RandomShardIterator() {
			seq = append(seq, i)
		}

		if !assert.ElementsMatch(t, seq, correct, fmt.Sprintf("sequences do not match: %v vs %v", seq, correct)) {
			return
		}
	}
}
