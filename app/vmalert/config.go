package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
)

// Parse parses rule configs from given file patterns
func Parse(pathPatterns []string, validateAnnotations bool) ([]Group, error) {
	var fp []string
	for _, pattern := range pathPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("error reading file patther %s:%v", pattern, err)
		}
		fp = append(fp, matches...)
	}
	var groups []Group
	for _, file := range fp {
		groupsNames := map[string]struct{}{}
		gr, err := parseFile(file)
		if err != nil {
			return nil, fmt.Errorf("file %s: %w", file, err)
		}
		for _, g := range gr {
			if _, ok := groupsNames[g.Name]; ok {
				return nil, fmt.Errorf("one file can not contain groups with the same name %s, filepath:%s", g.Name, file)
			}
			g.File = file
			g.doneCh = make(chan struct{})
			g.finishedCh = make(chan struct{})
			g.updateCh = make(chan Group)

			groupsNames[g.Name] = struct{}{}
			for _, rule := range g.Rules {
				if err = rule.Validate(); err != nil {
					return nil, fmt.Errorf("invalid rule filepath: %s, group %s: %w", file, g.Name, err)
				}
				if validateAnnotations {
					if err = notifier.ValidateTemplates(rule.Annotations); err != nil {
						return nil, fmt.Errorf("invalid annotations filepath: %s, group %s: %w", file, g.Name, err)
					}
					if err = notifier.ValidateTemplates(rule.Labels); err != nil {
						return nil, fmt.Errorf("invalid labels filepath: %s, group %s: %w", file, g.Name, err)
					}
				}
				rule.group = g
				rule.alerts = make(map[uint64]*notifier.Alert)
			}
			groups = append(groups, g)
		}
	}
	if len(groups) < 1 {
		return nil, fmt.Errorf("no groups found in %s", strings.Join(pathPatterns, ";"))
	}
	return groups, nil
}

func parseFile(path string) ([]Group, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading alert rule file: %w", err)
	}
	g := struct {
		Groups []Group `yaml:"groups"`
	}{}
	err = yaml.Unmarshal(data, &g)
	for i := 0 ; i < len(g.Groups); i++ {
		if g.Groups[i].Interval == 0 {
			g.Groups[i].Interval = *evaluationInterval
		}
	}
	return g.Groups, err
}
