package encoding

import (
	"encoding/hex"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

func TestIsConst(t *testing.T) {
	f := func(a []int64, okExpected bool) {
		t.Helper()
		ok := isConst(a)
		if ok != okExpected {
			t.Fatalf("unexpected isConst for a=%d; got %v; want %v", a, ok, okExpected)
		}
	}
	f([]int64{}, false)
	f([]int64{1}, true)
	f([]int64{1, 2}, false)
	f([]int64{1, 1}, true)
	f([]int64{1, 1, 1}, true)
	f([]int64{1, 1, 2}, false)
}

func TestIsDeltaConst(t *testing.T) {
	f := func(a []int64, okExpected bool) {
		t.Helper()
		ok := isDeltaConst(a)
		if ok != okExpected {
			t.Fatalf("unexpected isDeltaConst for a=%d; got %v; want %v", a, ok, okExpected)
		}
	}
	f([]int64{}, false)
	f([]int64{1}, false)
	f([]int64{1, 2}, true)
	f([]int64{1, 2, 3}, true)
	f([]int64{3, 2, 1}, true)
	f([]int64{3, 2, 1, 0, -1, -2}, true)
	f([]int64{3, 2, 1, 0, -1, -2, 2}, false)
	f([]int64{1, 1}, true)
	f([]int64{1, 2, 1}, false)
	f([]int64{1, 2, 4}, false)
}

func TestIsGauge(t *testing.T) {
	f := func(a []int64, okExpected bool) {
		t.Helper()
		ok := isGauge(a)
		if ok != okExpected {
			t.Fatalf("unexpected result for isGauge(%d); got %v; expecting %v", a, ok, okExpected)
		}
	}
	f([]int64{}, false)
	f([]int64{0}, false)
	f([]int64{1, 2}, false)
	f([]int64{0, 1, 2, 3, 4, 5}, false)
	f([]int64{0, -1, -2, -3, -4}, true)
	f([]int64{0, 0, 0, 0, 0, 0, 0}, false)
	f([]int64{1, 1, 1, 1, 1}, false)
	f([]int64{1, 1, 2, 2, 2, 2}, false)
	f([]int64{1, 17, 2, 3}, false) // a single counter reset
	f([]int64{1, 5, 2, 3}, true)
	f([]int64{1, 5, 2, 3, 2}, true)
	f([]int64{-1, -5, -2, -3}, true)
	f([]int64{-1, -5, -2, -3, -2}, true)
	f([]int64{5, 6, 4, 3, 2}, true)
	f([]int64{4, 5, 6, 5, 4, 3, 2}, true)
	f([]int64{1064, 1132, 1083, 1062, 856, 747}, true)
}

func TestEnsureNonDecreasingSequence(t *testing.T) {
	testEnsureNonDecreasingSequence(t, []int64{}, -1234, -34, []int64{})
	testEnsureNonDecreasingSequence(t, []int64{123}, -1234, -1234, []int64{-1234})
	testEnsureNonDecreasingSequence(t, []int64{123}, -1234, 345, []int64{345})
	testEnsureNonDecreasingSequence(t, []int64{-23, -14}, -23, -14, []int64{-23, -14})
	testEnsureNonDecreasingSequence(t, []int64{-23, -14}, -25, 0, []int64{-25, 0})
	testEnsureNonDecreasingSequence(t, []int64{0, -1, 10, 5, 6, 7}, 2, 8, []int64{2, 2, 8, 8, 8, 8})
	testEnsureNonDecreasingSequence(t, []int64{0, -1, 10, 5, 6, 7}, -2, 8, []int64{-2, -1, 8, 8, 8, 8})
	testEnsureNonDecreasingSequence(t, []int64{0, -1, 10, 5, 6, 7}, -2, 12, []int64{-2, -1, 10, 10, 10, 12})
	testEnsureNonDecreasingSequence(t, []int64{1, 2, 1, 3, 4, 5}, 1, 5, []int64{1, 2, 2, 3, 4, 5})
}

func testEnsureNonDecreasingSequence(t *testing.T, a []int64, vMin, vMax int64, aExpected []int64) {
	t.Helper()

	EnsureNonDecreasingSequence(a, vMin, vMax)
	if !reflect.DeepEqual(a, aExpected) {
		t.Fatalf("unexpected a; got\n%d; expecting\n%d", a, aExpected)
	}
}

func testMarshalUnmarshalInt64Array(t *testing.T, va []int64, precisionBits uint8, mtExpected MarshalType) {
	t.Helper()

	b, mt, firstValue := marshalInt64Array(nil, va, precisionBits)
	if mt != mtExpected {
		t.Fatalf("unexpected MarshalType for va=%d, precisionBits=%d: got %d; expecting %d", va, precisionBits, mt, mtExpected)
	}
	vaNew, err := unmarshalInt64Array(nil, b, mt, firstValue, len(va))
	if err != nil {
		t.Fatalf("unexpected error when unmarshaling va=%d, precisionBits=%d: %s", va, precisionBits, err)
	}
	if vaNew == nil && va != nil {
		vaNew = []int64{}
	}
	switch mt {
	case MarshalTypeZSTDNearestDelta, MarshalTypeZSTDNearestDelta2,
		MarshalTypeNearestDelta, MarshalTypeNearestDelta2:
		if err = checkPrecisionBits(va, vaNew, precisionBits); err != nil {
			t.Fatalf("too low precision for vaNew: %s", err)
		}
	default:
		if !reflect.DeepEqual(va, vaNew) {
			t.Fatalf("unexpected vaNew for va=%d, precisionBits=%d; got\n%d; expecting\n%d", va, precisionBits, vaNew, va)
		}
	}

	bPrefix := []byte{1, 2, 3}
	bNew, mtNew, firstValueNew := marshalInt64Array(bPrefix, va, precisionBits)
	if firstValueNew != firstValue {
		t.Fatalf("unexpected firstValue for prefixed va=%d, precisionBits=%d; got %d; want %d", va, precisionBits, firstValueNew, firstValue)
	}
	if string(bNew[:len(bPrefix)]) != string(bPrefix) {
		t.Fatalf("unexpected prefix for va=%d, precisionBits=%d; got\n%d; expecting\n%d", va, precisionBits, bNew[:len(bPrefix)], bPrefix)
	}
	if string(bNew[len(bPrefix):]) != string(b) {
		t.Fatalf("unexpected b for prefixed va=%d, precisionBits=%d; got\n%d; expecting\n%d", va, precisionBits, bNew[len(bPrefix):], b)
	}
	if mtNew != mt {
		t.Fatalf("unexpected mt for prefixed va=%d, precisionBits=%d; got %d; expecting %d", va, precisionBits, mtNew, mt)
	}

	vaPrefix := []int64{4, 5, 6, 8}
	vaNew, err = unmarshalInt64Array(vaPrefix, b, mt, firstValue, len(va))
	if err != nil {
		t.Fatalf("unexpected error when unmarshaling prefixed va=%d, precisionBits=%d: %s", va, precisionBits, err)
	}
	if !reflect.DeepEqual(vaNew[:len(vaPrefix)], vaPrefix) {
		t.Fatalf("unexpected prefix for va=%d, precisionBits=%d; got\n%d; expecting\n%d", va, precisionBits, vaNew[:len(vaPrefix)], vaPrefix)
	}
	if va == nil {
		va = []int64{}
	}
	switch mt {
	case MarshalTypeZSTDNearestDelta, MarshalTypeZSTDNearestDelta2,
		MarshalTypeNearestDelta, MarshalTypeNearestDelta2:
		if err = checkPrecisionBits(vaNew[len(vaPrefix):], va, precisionBits); err != nil {
			t.Fatalf("too low precision for prefixed vaNew: %s", err)
		}
	default:
		if !reflect.DeepEqual(vaNew[len(vaPrefix):], va) {
			t.Fatalf("unexpected prefixed vaNew for va=%d, precisionBits=%d; got\n%d; expecting\n%d", va, precisionBits, vaNew[len(vaPrefix):], va)
		}
	}
}

func TestMarshalUnmarshalTimestamps(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	const precisionBits = 3

	var timestamps []int64
	v := int64(0)
	for range 8 * 1024 {
		v += 30e3 * int64(r.NormFloat64()*5e2)
		timestamps = append(timestamps, v)
	}
	result, mt, firstTimestamp := MarshalTimestamps(nil, timestamps, precisionBits)
	timestamps2, err := UnmarshalTimestamps(nil, result, mt, firstTimestamp, len(timestamps))
	if err != nil {
		t.Fatalf("cannot unmarshal timestamps: %s", err)
	}
	if err := checkPrecisionBits(timestamps, timestamps2, precisionBits); err != nil {
		t.Fatalf("too low precision for timestamps: %s", err)
	}
}

func TestMarshalUnmarshalValues(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	const precisionBits = 3

	var values []int64
	v := int64(0)
	for range 8 * 1024 {
		v += int64(r.NormFloat64() * 1e2)
		values = append(values, v)
	}
	result, mt, firstValue := MarshalValues(nil, values, precisionBits)
	values2, err := UnmarshalValues(nil, result, mt, firstValue, len(values))
	if err != nil {
		t.Fatalf("cannot unmarshal values: %s", err)
	}
	if err := checkPrecisionBits(values, values2, precisionBits); err != nil {
		t.Fatalf("too low precision for values: %s", err)
	}
}

func TestMarshalUnmarshalInt64ArrayGeneric(t *testing.T) {
	testMarshalUnmarshalInt64Array(t, []int64{1, 20, 234}, 4, MarshalTypeNearestDelta2)
	testMarshalUnmarshalInt64Array(t, []int64{1, 20, -2345, 678934, 342}, 4, MarshalTypeNearestDelta)
	testMarshalUnmarshalInt64Array(t, []int64{1, 20, 2345, 6789, 12342}, 4, MarshalTypeNearestDelta2)

	// Constant encoding
	testMarshalUnmarshalInt64Array(t, []int64{1}, 4, MarshalTypeConst)
	testMarshalUnmarshalInt64Array(t, []int64{1, 2}, 4, MarshalTypeDeltaConst)
	testMarshalUnmarshalInt64Array(t, []int64{-1, 0, 1, 2, 3, 4, 5}, 4, MarshalTypeDeltaConst)
	testMarshalUnmarshalInt64Array(t, []int64{-10, -1, 8, 17, 26}, 4, MarshalTypeDeltaConst)
	testMarshalUnmarshalInt64Array(t, []int64{0, 0, 0, 0, 0, 0}, 4, MarshalTypeConst)
	testMarshalUnmarshalInt64Array(t, []int64{100, 100, 100, 100}, 4, MarshalTypeConst)
}

func testMarshalInt64ArraySize(t *testing.T, va []int64, precisionBits uint8, minSizeExpected, maxSizeExpected int) {
	t.Helper()

	b, _, _ := marshalInt64Array(nil, va, precisionBits)
	if len(b) > maxSizeExpected {
		t.Fatalf("too big size for marshaled %d items with precisionBits %d: got %d; expecting %d", len(va), precisionBits, len(b), maxSizeExpected)
	}
	if len(b) < minSizeExpected {
		t.Fatalf("too small size for marshaled %d items with precisionBits %d: got %d; expecting %d", len(va), precisionBits, len(b), minSizeExpected)
	}
}

func TestUnmarshalInt64ArrayZSTDLimitFail(t *testing.T) {
	// reset global sync pool to prevent possible non-empty buffers
	// which could skew test results
	bbPool = bytesutil.ByteBufferPool{}
	f := func(src string, mt MarshalType) {
		input, err := hex.DecodeString(src)
		if err != nil {
			t.Fatalf("BUG: unexpected hex input: %s", err)
		}
		_, err = unmarshalInt64Array(nil, input, mt, 0, 10)
		if err == nil {
			t.Fatalf("unexpected nil err")
		}
		if !strings.Contains(err.Error(), "limit") && !strings.Contains(err.Error(), "window size exceeded") {
			t.Fatalf(`expecting "exceeds limit" or "window size exceeded" errors, got: %q`, err.Error())
		}
	}
	// large input
	f("28b52ffdc0680000008000000000592e0041ce32383616690d9d593eff629adab9d68976436e9075ea409c3def3dacd9f22f1a501c72acc20f725a488fe8dd89edd3fc1c373896a2219a6e9eb239b91bae0a69fcc779372182e1733feac78b7cf8da5dcdbdab3c10552cc4e88cea5e5c4d0a922fcf0510fd7d237c1aa9b9fc2af12283806d7edc379e2d7613160af215e7676de0cccd8dce1a704dd16474890cf1e2492ddf68cd9013d45c15e84fcdbd9359d2075888080d6e00bf5db3b18998105484cdaa236793cd5b4fe8997247cd6a18999d09f0d731940ba8b62d29544965ae0d57676503b7e4f82684c22bb9bc84dc9b9a77bb54b3a2c9e2d26b9242950929bdc4ade8f77470e5b9e0fb337902f8ddaff53ca3d70ab17244ac05a5abf414fe6b0b16629d438858e0f4db40e118df53ada6ee5eb6afd83ea639921ae7df9d63ef25388d4c9c458a73b9fb0b487651756da626a54b193064e173aba94fcdbd912109ab1f5cf5beb34b27f34c0692657dc7ec7b30ecc0d11ecb52e80a3a368cc0d9af09d126c80d0b3e345bbb74e735a2937186b8f31dd3a23ecde0f500932228f010e85b008d47ca26dbca0fe8ead61baa2cdff3b7a74d2a11dc2c22f95c96013d23e07544bb8728da11d902bc166322e3151e9a91d758ec1c7864487d07018398e18bd749cae35d32d01f4b47e0f2ed1625da65d7240c2238991f21e50f7ae6d9417f39673374a1071036a2e1c29cb51b53ac0be30780023b35f66488e91b62d83c5553f66e522c30c8b4bee33479b3cb982c460cf7cdc5ff4781c8bd10f92f6ec9a4dd4ff2943684f03159a3ada01dfcecf1cdaa6c0ba45bc45707dff92f8471e0058ec83753f6dd8146297ba52a2c8845945815df51d527a612d7431ed39bbb7a1020bbb5a7a613728be8c2952f18469dac9ea6c8b8913be547f242f7feab238e5c4a7fe5a87cfcbc984cd0f4c6190eecd359b39e58107078910e35fbc8eb821583c36d3f6db27256050efd3a0867b9299f4096d9488a9c5786e473a24895ae061b45db3a0e7e4b3969e6a2e10d3a1d4ca90aa6f2a237997bbf2c546598df815abc86234ea51a615c5c8d155eecca144e88703b607b4e3edbd49e30bbcd02d208ff409bf448bc8bb778b9b02bbe4f7527d52bae9bee0963705cf2601d023679097c299da4109da2435323cabf9b0018cd28f1551c119fe131f0e03a024db44e54cbfa81d414f57cf88683bb939a17d53b187b8a5bfd488f16282018e2c7b229e1836b9dfe6efcf3cca891dc1ed5cb85663f81a4cfca3b44189b6a51368769e3a93d344a9a34ebdeba520483d96153fe4832bbfbf08fb2da2fe2763d76a61153a3910de11f7ef815b71c960a3b59e91ad98ce1e73765e3285295fc35193ce251c816aeb905655b70bd9efec8bdd3c3b010e3abc11b630707794bb48f976114b50f66cf2ac5014ec8c46ff415d6e81a99e5d0d1e014a3574d9d9d55a5692cde577116288cd76a08f3c3282f4afeb027ab51eb0b9c9b3b09d4b52399ce06b67a10e5c17138b2b7c1cbb26169bdae4220c04bcb114d28435673ba743662e6161fb663fd6285ce02b0278ab158c85eb73eaecc099b4b32d57602a32ae6b983660423e463b2cba3600fdf42b1b6c28426b94fcb3d60ee3107a15968da5f2c52bc632e5f8a1b62aaec15ccd3f939039ba9c97b41fe1a8f9ef21051021f5a12cd3073b0cd0b54a25bbcf9c434209ae31856a60825e57f85f0adf8c0688f90c26bdc6b7f257e14e32de9f5a15ebf92bb754688d06f8a4928a051948093c3b796a5d68475607b157e988a3f3a486ed7e12a90a7a918842ed025f61b687a5388e6f237f76875990dca15b81067eac422afae4d3b6d176ce8d5f8b4c55116d6bb08cb16fa325d71b51f1c8ba0003d5a2b7c79957bf365da2d9090a2ca115ee50856a78cc6ab0f9bc5d5231b28de38c8aaa0047fa114f966b65865d961824b182727ffb193a983aef644a47fded7d4a65774c624e4a6cca73b85f070c47ad406d0d34a9c9906b243fec517d0fbc5088bea4e389c7b83d536746fa88029d3f148bbb20067109893f296aef325b77fcd101a88d1ffa0820caa196e0621ca659b", MarshalTypeZSTDNearestDelta2)

	// input with stream window size bigger than actual payload
	f("28b52ffd8400005ed0b209000030ecaf4412", MarshalTypeZSTDNearestDelta2)

	// input with framecontent bigger than actual payload
	f("28b52ffd8400005ed0b209000030ecaf4412", MarshalTypeZSTDNearestDelta)
}
