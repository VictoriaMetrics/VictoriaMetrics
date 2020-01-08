// +build gofuzz

package fastjson

func Fuzz(data []byte) int {
	err := ValidateBytes(data)
	if err != nil {
		return 0
	}

	v := MustParseBytes(data)

	dst := make([]byte, 0)
	dst = v.MarshalTo(dst)

	err = ValidateBytes(dst)
	if err != nil {
		panic(err)
	}

	return 1
}
