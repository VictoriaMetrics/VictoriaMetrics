package promremotewrite

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/andybalholm/brotli"
	"github.com/golang/snappy"
)

const (
	algSnappy = "snappy"
	algBrotli = "br"

	brotliQualityMax = 11
	errUnknownAlg    = "unknown compression algorithm"
)

var (
	remoteWriteCompressionAlg = flag.String("remoteWrite.compression", algSnappy, "Set the compression algorithm to be used when sending data over to remoteWrite.url[s]. [snappy | br]")
	remoteWriteBrotliQuality  = flag.Int("remoteWrite.brotli.quality", brotliQualityMax, "Set the brotli compression quality, values range between [1..11], the higher the number the higher the cpu usage and better compression.")
)

// Encode encodes a given byte array using
// either of the [snappy | brotli] algorithms.
func Encode(dst, src []byte) ([]byte, error) {

	switch *remoteWriteCompressionAlg {
	case algSnappy:
		return snappy.Encode(dst, src), nil
	case algBrotli:
		b := new(bytes.Buffer)
		w := brotli.NewWriterOptions(b, brotli.WriterOptions{Quality: *remoteWriteBrotliQuality})

		_, err := w.Write(src)
		if err != nil {
			return nil, err
		}

		err = w.Close()
		if err != nil {
			return nil, err
		}

		return b.Bytes(), nil
	default:
		return nil, fmt.Errorf("%s: %s", errUnknownAlg, *remoteWriteCompressionAlg)
	}
}

// Decode decodes a given byte array using
// either of the [snappy | brotli] algorithms.
func Decode(dst, src []byte) ([]byte, error) {

	switch *remoteWriteCompressionAlg {
	case algSnappy:
		return snappy.Decode(dst, src)
	case algBrotli:
		b := bytes.NewReader(src)
		r := brotli.NewReader(b)

		return ioutil.ReadAll(r)
	default:
		return nil, fmt.Errorf("%s: %s", errUnknownAlg, *remoteWriteCompressionAlg)
	}
}

// Header returns the name of the selected encoding algorithm.
func Header() string {
	return *remoteWriteCompressionAlg
}
