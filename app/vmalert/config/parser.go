package config

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/common"
	"gopkg.in/yaml.v2"
)

// Parse parses rule configs from given file patterns
func Parse(pathPatterns []string, validateAnnotations bool) ([]common.Group, error) {
	var fp []string
	for _, pattern := range pathPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("error reading file patther %s:%v", pattern, err)
		}
		fp = append(fp, matches...)
	}
	var groups []common.Group
	for _, file := range fp {
		groupsNames := map[string]struct{}{}
		gr, err := parseFile(file)
		if err != nil {
			return nil, fmt.Errorf("file %s: %w", file, err)
		}
		for _, group := range gr {
			if _, ok := groupsNames[group.Name]; ok {
				return nil, fmt.Errorf("one file can not contain groups with the same name %s, filepath:%s", file, group.Name)
			}
			groupsNames[group.Name] = struct{}{}
			for _, rule := range group.Rules {
				if err = rule.Validate(); err != nil {
					return nil, fmt.Errorf("invalid rule filepath:%s, group %s:%w", file, group.Name, err)
				}
				if validateAnnotations {
					if err = common.ValidateAnnotations(rule.Annotations); err != nil {
						return nil, fmt.Errorf("invalida annotations filepath:%s, group %s:%w", file, group.Name, err)
					}
				}
			}
		}
		groups = append(groups, gr...)
	}
	if len(groups) < 1 {
		return nil, fmt.Errorf("no groups found in %s", strings.Join(pathPatterns, ";"))
	}
	return groups, nil
}

func parseFile(path string) ([]common.Group, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading alert rule file: %w", err)
	}
	g := struct {
		Groups []common.Group `yaml:"groups"`
	}{}
	err = yaml.Unmarshal(data, &g)
	return g.Groups, err
}
