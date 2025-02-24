package logstorage

import (
	"fmt"
	"testing"
)

func BenchmarkJSONParserParseLogMessage(b *testing.B) {
	msg := []byte(`{
  "actor": {
    "avatar_url": "https://avatars.githubusercontent.com/u/51232360?",
    "display_login": "ioiofda",
    "id": "578980",
    "login": "jooiojya",
    "url": "https://api.github.com/users/jjoiojoa"
  },
  "payload": {
    "action": "started"
  },
  "public": "true",
  "repo": {
    "id": "5898l329",
    "name": "piijojojva/sjwjjkjllj",
    "url": "https://api.github.com/repos/jjsdhfkjhdsfk/jlkowiejro"
  },
  "type": "WatchEvent"
}`)

	b.ReportAllocs()
	b.SetBytes(int64(len(msg)))
	b.RunParallel(func(pb *testing.PB) {
		p := GetJSONParser()
		for pb.Next() {
			if err := p.ParseLogMessage(msg); err != nil {
				panic(fmt.Errorf("unexpected error: %s", err))
			}
		}
		PutJSONParser(p)
	})
}
