package gcsremote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

func TestDownloadPartRange(t *testing.T) {
	testCases := []struct {
		name                 string
		offset, size         uint64
		empty, ignore, short bool
		wantErr              bool
	}{
		{"whole", 0, 16, false, false, false, false},
		{"middle", 4, 5, false, false, false, false},
		{"tail", 15, 1, false, false, false, false},
		{"empty", 0, 0, true, false, false, false},
		{"ignored-range", 4, 5, false, true, false, true},
		{"short-response", 4, 5, false, false, true, true},
		{"outside-object", 15, 2, false, false, false, true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := "0123456789abcdef"
			if tc.empty {
				data = ""
			}
			p := common.Part{Path: "data/file", FileSize: 1024, Offset: 128, Size: uint64(len(data))}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if !strings.HasSuffix(r.URL.Path, p.RemotePath("backup")) {
					t.Errorf("request changed the original object key: %s", r.URL.Path)
				}
				wantRange := ""
				if tc.offset != 0 || tc.size != p.Size {
					wantRange = fmt.Sprintf("bytes=%d-%d", tc.offset, tc.offset+tc.size-1)
				}
				if r.Header.Get("Range") != wantRange {
					t.Errorf("unexpected Range: %q; want %q", r.Header.Get("Range"), wantRange)
				}
				if tc.ignore {
					_, _ = io.WriteString(w, data)
				} else if tc.short {
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", tc.offset, tc.offset+tc.size-1, p.Size))
					w.WriteHeader(http.StatusPartialContent)
					_, _ = io.WriteString(w, data[tc.offset:tc.offset+tc.size-1])
				} else {
					http.ServeContent(w, r, "object", time.Time{}, bytes.NewReader([]byte(data)))
				}
			}))
			defer server.Close()
			ctx := context.Background()
			client, err := storage.NewClient(ctx, option.WithEndpoint(server.URL), option.WithoutAuthentication(), storage.WithJSONReads())
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			fs := &FS{Bucket: "bucket", Dir: "backup/", bkt: client.Bucket("bucket"), ctx: ctx}
			var dst bytes.Buffer
			err = fs.DownloadPartRange(p, tc.offset, tc.size, &dst)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected download error: %v", err)
			}
			if !tc.wantErr && dst.String() != data[tc.offset:tc.offset+tc.size] {
				t.Fatalf("unexpected downloaded bytes: %q", dst.String())
			}
			if tc.name == "outside-object" && calls.Load() != 0 {
				t.Fatal("invalid range issued an HTTP request")
			}
		})
	}
}
