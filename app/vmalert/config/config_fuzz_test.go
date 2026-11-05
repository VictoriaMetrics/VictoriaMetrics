package config

import (
	"regexp"
	"testing"

	"gopkg.in/yaml.v2"
	"sigs.k8s.io/randfill"
)

var (
	// Skip XXX since fuzzer cannot fill in `any` fields
	fuzzSkipXXX = regexp.MustCompile("^XXX$")
	// Predefined fuzz types and expressions to avoid generating invalid configs
	fuzzTypes = []Type{NewPrometheusType(), NewGraphiteType(), NewVLogsType()}
	fuzzExprs = []string{"up == 0", "foo.bar", "_time: 1m | stats count(*) as requests"}
)

func FuzzParseConfig(f *testing.F) {
	f.Add([]byte("groups:\n- name: test\n  rules:\n  - alert: up\n    expr: up == 0\n"))
	f.Add([]byte("groups:\n- name: test\n  labels:\n    home: \"%{HOME}\"\n  rules:\n  - alert: up\n    expr: up == 0\n"))
	f.Add([]byte("groups: ["))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_, _ = parseConfig(data)
	})
}

func FuzzValidConfig(f *testing.F) {
	f.Add([]byte("production"), uint8(0))
	f.Add([]byte("%{HOME}"), uint8(1))
	f.Add([]byte(""), uint8(2))

	f.Fuzz(func(t *testing.T, data []byte, typeIndex uint8) {
		if len(data) > 4096 {
			t.Skip()
		}

		i := int(typeIndex) % len(fuzzTypes)
		cfg := struct {
			Groups []Group `yaml:"groups"`
		}{Groups: []Group{{
			Name: "test",
			Type: fuzzTypes[i],
			Rules: []Rule{
				{Alert: "up", Expr: fuzzExprs[i], Labels: map[string]string{"source": string(data)}},
				{Record: "cpu", Expr: fuzzExprs[i]},
			},
		}}}
		b, err := yaml.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}

		groups, err := parseConfig(b)
		if err != nil {
			t.Fatal(err)
		}
		if len(groups) != 1 || len(groups[0].Rules) != 2 {
			t.Fatalf("unexpected parsed groups: %+v", groups)
		}
		if groups[0].Checksum == "" {
			t.Fatal("group checksum not set")
		}
		for _, r := range groups[0].Rules {
			if r.ID != HashRule(r) {
				t.Fatal("rule ID does not match rule hash")
			}
		}
		if err := groups[0].Validate(nil, true); err != nil {
			t.Fatal(err)
		}
	})
}

func FuzzValidateGroup(f *testing.F) {
	f.Add([]byte("test up == 0"), uint8(0))
	f.Add([]byte("test foo.bar"), uint8(1))
	f.Add([]byte("test _time: 1m | stats count(*) as requests"), uint8(2))
	f.Add([]byte(""), uint8(0))

	f.Fuzz(func(t *testing.T, data []byte, typeIndex uint8) {
		if len(data) > 4096 {
			t.Skip()
		}

		var g Group
		randfill.NewFromGoFuzz(data).
			NilChance(0.2).
			MaxDepth(5).
			NumElements(0, 3).
			SkipFieldsWithPattern(fuzzSkipXXX).
			Fill(&g)
		g.Type = fuzzTypes[int(typeIndex)%len(fuzzTypes)]
		_ = g.Validate(nil, true)
	})
}

func FuzzValidateRule(f *testing.F) {
	f.Add([]byte("up == 0"))
	f.Add([]byte(""))
	f.Add([]byte("cpu up"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		var r Rule
		randfill.NewFromGoFuzz(data).
			NilChance(0.2).
			MaxDepth(5).
			NumElements(0, 3).
			SkipFieldsWithPattern(fuzzSkipXXX).
			Fill(&r)
		_ = r.Validate()
	})
}
