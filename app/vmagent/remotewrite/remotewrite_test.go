package remotewrite

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestExtractShardingLabels(t *testing.T) {
	shardByURLLabelsMap = make(map[string]struct{})
	shardByURLLabelsMap["instance"] = struct{}{}
	shardByURLLabelsMap["job"] = struct{}{}
	defer func() {
		shardByURLLabelsMap = nil
	}()

	f := func(in, exp []prompbmarshal.Label, inverse bool) {
		t.Helper()
		var got []prompbmarshal.Label
		got = extractShardingLabels(got, in, inverse)
		if !reflect.DeepEqual(got, exp) {
			t.Fatalf("expected to get \n%#v; \ngot \n%#v instead", exp, got)
		}
	}

	f(nil, nil, true)
	f(nil, nil, false)

	f([]prompbmarshal.Label{{Name: "foo"}}, nil, false)
	f([]prompbmarshal.Label{{Name: "foo"}}, []prompbmarshal.Label{{Name: "foo"}}, true)

	f([]prompbmarshal.Label{{Name: "foo"}, {Name: "job"}}, []prompbmarshal.Label{{Name: "job"}}, false)
	f([]prompbmarshal.Label{{Name: "foo"}, {Name: "job"}}, []prompbmarshal.Label{{Name: "foo"}}, true)

	f([]prompbmarshal.Label{{Name: "foo"}, {Name: "instance"}, {Name: "job"}}, []prompbmarshal.Label{{Name: "instance"}, {Name: "job"}}, false)
	f([]prompbmarshal.Label{{Name: "foo"}, {Name: "instance"}, {Name: "job"}}, []prompbmarshal.Label{{Name: "foo"}}, true)
}
