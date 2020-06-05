package fs

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func BenchmarkReaderAtMustReadAt(b *testing.B) {
	b.Run("mmap_on", func(b *testing.B) {
		benchmarkReaderAtMustReadAt(b, true)
	})
	b.Run("mmap_off", func(b *testing.B) {
		benchmarkReaderAtMustReadAt(b, false)
	})
}

func benchmarkReaderAtMustReadAt(b *testing.B, isMmap bool) {
	prevDisableMmap := *disableMmap
	*disableMmap = !isMmap
	defer func() {
		*disableMmap = prevDisableMmap
	}()

	path := "BenchmarkReaderAtMustReadAt"
	const fileSize = 8 * 1024 * 1024
	data := make([]byte, fileSize)
	if err := ioutil.WriteFile(path, data, 0600); err != nil {
		b.Fatalf("cannot create %q: %s", path, err)
	}
	defer MustRemoveAll(path)
	r, err := OpenReaderAt(path)
	if err != nil {
		b.Fatalf("error in OpenReaderAt(%q): %s", path, err)
	}
	defer r.MustClose()

	b.ResetTimer()
	for _, bufSize := range []int{1, 1e1, 1e2, 1e3, 1e4, 1e5} {
		b.Run(fmt.Sprintf("%d", bufSize), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(bufSize))
			b.RunParallel(func(pb *testing.PB) {
				buf := make([]byte, bufSize)
				var offset int64
				for pb.Next() {
					if len(buf)+int(offset) > fileSize {
						offset = 0
					}
					r.MustReadAt(buf, offset)
					offset += int64(len(buf))
				}
			})
		})
	}
}
