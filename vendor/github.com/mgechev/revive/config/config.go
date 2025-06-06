// Package config implements revive's configuration data structures and related methods
package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/mgechev/revive/formatter"
	"github.com/mgechev/revive/lint"
	"github.com/mgechev/revive/rule"
)

var defaultRules = []lint.Rule{
	&rule.VarDeclarationsRule{},
	&rule.PackageCommentsRule{},
	&rule.DotImportsRule{},
	&rule.BlankImportsRule{},
	&rule.ExportedRule{},
	&rule.VarNamingRule{},
	&rule.IndentErrorFlowRule{},
	&rule.RangeRule{},
	&rule.ErrorfRule{},
	&rule.ErrorNamingRule{},
	&rule.ErrorStringsRule{},
	&rule.ReceiverNamingRule{},
	&rule.IncrementDecrementRule{},
	&rule.ErrorReturnRule{},
	&rule.UnexportedReturnRule{},
	&rule.TimeNamingRule{},
	&rule.ContextKeysType{},
	&rule.ContextAsArgumentRule{},
	&rule.EmptyBlockRule{},
	&rule.SuperfluousElseRule{},
	&rule.UnusedParamRule{},
	&rule.UnreachableCodeRule{},
	&rule.RedefinesBuiltinIDRule{},
}

var allRules = append([]lint.Rule{
	&rule.ArgumentsLimitRule{},
	&rule.CyclomaticRule{},
	&rule.FileHeaderRule{},
	&rule.ConfusingNamingRule{},
	&rule.GetReturnRule{},
	&rule.ModifiesParamRule{},
	&rule.ConfusingResultsRule{},
	&rule.DeepExitRule{},
	&rule.AddConstantRule{},
	&rule.FlagParamRule{},
	&rule.UnnecessaryStmtRule{},
	&rule.StructTagRule{},
	&rule.ModifiesValRecRule{},
	&rule.ConstantLogicalExprRule{},
	&rule.BoolLiteralRule{},
	&rule.ImportsBlocklistRule{},
	&rule.FunctionResultsLimitRule{},
	&rule.MaxPublicStructsRule{},
	&rule.RangeValInClosureRule{},
	&rule.RangeValAddress{},
	&rule.WaitGroupByValueRule{},
	&rule.AtomicRule{},
	&rule.EmptyLinesRule{},
	&rule.LineLengthLimitRule{},
	&rule.CallToGCRule{},
	&rule.DuplicatedImportsRule{},
	&rule.ImportShadowingRule{},
	&rule.BareReturnRule{},
	&rule.UnusedReceiverRule{},
	&rule.UnhandledErrorRule{},
	&rule.CognitiveComplexityRule{},
	&rule.StringOfIntRule{},
	&rule.StringFormatRule{},
	&rule.EarlyReturnRule{},
	&rule.UnconditionalRecursionRule{},
	&rule.IdenticalBranchesRule{},
	&rule.DeferRule{},
	&rule.UnexportedNamingRule{},
	&rule.FunctionLength{},
	&rule.NestedStructs{},
	&rule.UselessBreak{},
	&rule.UncheckedTypeAssertionRule{},
	&rule.TimeEqualRule{},
	&rule.BannedCharsRule{},
	&rule.OptimizeOperandsOrderRule{},
	&rule.UseAnyRule{},
	&rule.DataRaceRule{},
	&rule.CommentSpacingsRule{},
	&rule.IfReturnRule{},
	&rule.RedundantImportAlias{},
	&rule.ImportAliasNamingRule{},
	&rule.EnforceMapStyleRule{},
	&rule.EnforceRepeatedArgTypeStyleRule{},
	&rule.EnforceSliceStyleRule{},
	&rule.MaxControlNestingRule{},
	&rule.CommentsDensityRule{},
	&rule.FileLengthLimitRule{},
	&rule.FilenameFormatRule{},
	&rule.RedundantBuildTagRule{},
	&rule.UseErrorsNewRule{},
	&rule.RedundantTestMainExitRule{},
}, defaultRules...)

// allFormatters is a list of all available formatters to output the linting results.
// Keep the list sorted and in sync with available formatters in README.md.
var allFormatters = []lint.Formatter{
	&formatter.Checkstyle{},
	&formatter.Default{},
	&formatter.Friendly{},
	&formatter.JSON{},
	&formatter.NDJSON{},
	&formatter.Plain{},
	&formatter.Sarif{},
	&formatter.Stylish{},
	&formatter.Unix{},
}

func getFormatters() map[string]lint.Formatter {
	result := map[string]lint.Formatter{}
	for _, f := range allFormatters {
		result[f.Name()] = f
	}
	return result
}

// GetLintingRules yields the linting rules that must be applied by the linter
func GetLintingRules(config *lint.Config, extraRules []lint.Rule) ([]lint.Rule, error) {
	rulesMap := map[string]lint.Rule{}
	for _, r := range allRules {
		rulesMap[r.Name()] = r
	}
	for _, r := range extraRules {
		if _, ok := rulesMap[r.Name()]; ok {
			continue
		}
		rulesMap[r.Name()] = r
	}

	var lintingRules []lint.Rule
	for name, ruleConfig := range config.Rules {
		actualName := actualRuleName(name)
		r, ok := rulesMap[actualName]
		if !ok {
			return nil, fmt.Errorf("cannot find rule: %s", name)
		}

		if ruleConfig.Disabled {
			continue // skip disabled rules
		}

		if r, ok := r.(lint.ConfigurableRule); ok {
			if err := r.Configure(ruleConfig.Arguments); err != nil {
				return nil, fmt.Errorf("cannot configure rule: %q: %w", name, err)
			}
		}

		lintingRules = append(lintingRules, r)
	}

	return lintingRules, nil
}

func actualRuleName(name string) string {
	switch name {
	case "imports-blacklist":
		return "imports-blocklist"
	default:
		return name
	}
}

func parseConfig(path string, config *lint.Config) error {
	file, err := os.ReadFile(path)
	if err != nil {
		return errors.New("cannot read the config file")
	}
	err = toml.Unmarshal(file, config)
	if err != nil {
		return fmt.Errorf("cannot parse the config file: %w", err)
	}
	for k, r := range config.Rules {
		err := r.Initialize()
		if err != nil {
			return fmt.Errorf("error in config of rule [%s] : [%w]", k, err)
		}
		config.Rules[k] = r
	}

	return nil
}

func normalizeConfig(config *lint.Config) {
	if len(config.Rules) == 0 {
		config.Rules = map[string]lint.RuleConfig{}
	}
	if config.EnableAllRules {
		// Add to the configuration all rules not yet present in it
		for _, r := range allRules {
			ruleName := r.Name()
			_, alreadyInConf := config.Rules[ruleName]
			if alreadyInConf {
				continue
			}
			// Add the rule with an empty conf for
			config.Rules[ruleName] = lint.RuleConfig{}
		}
	}

	severity := config.Severity
	if severity != "" {
		for k, v := range config.Rules {
			if v.Severity == "" {
				v.Severity = severity
			}
			config.Rules[k] = v
		}
		for k, v := range config.Directives {
			if v.Severity == "" {
				v.Severity = severity
			}
			config.Directives[k] = v
		}
	}
}

const defaultConfidence = 0.8

// GetConfig yields the configuration
func GetConfig(configPath string) (*lint.Config, error) {
	config := &lint.Config{}
	switch {
	case configPath != "":
		config.Confidence = defaultConfidence
		err := parseConfig(configPath, config)
		if err != nil {
			return nil, err
		}

	default: // no configuration provided
		config = defaultConfig()
	}

	normalizeConfig(config)
	return config, nil
}

// GetFormatter yields the formatter for lint failures
func GetFormatter(formatterName string) (lint.Formatter, error) {
	formatters := getFormatters()
	if formatterName == "" {
		return formatters["default"], nil
	}
	f, ok := formatters[formatterName]
	if !ok {
		return nil, fmt.Errorf("unknown formatter %v", formatterName)
	}
	return f, nil
}

func defaultConfig() *lint.Config {
	defaultConfig := lint.Config{
		Confidence: defaultConfidence,
		Severity:   lint.SeverityWarning,
		Rules:      map[string]lint.RuleConfig{},
	}
	for _, r := range defaultRules {
		defaultConfig.Rules[r.Name()] = lint.RuleConfig{}
	}
	return &defaultConfig
}
