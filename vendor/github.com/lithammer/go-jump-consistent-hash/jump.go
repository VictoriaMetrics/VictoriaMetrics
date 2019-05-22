package jump

// Hash takes a 64 bit key and the number of buckets. It outputs a bucket
// number in the range [0, buckets].
func Hash(key uint64, buckets int32) int32 {
	var b, j int64

	if buckets <= 0 {
		buckets = 1
	}

	for j < int64(buckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = int64(float64(b+1) * (float64(int64(1)<<31) / float64((key>>33)+1)))
	}

	return int32(b)
}
