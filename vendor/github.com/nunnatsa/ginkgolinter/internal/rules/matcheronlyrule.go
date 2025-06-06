package rules

var matcherOnlyRules = Rules{
	&HaveLen0{},
	&EqualBoolRule{},
	&EqualNilRule{},
	&DoubleNegativeRule{},
}

func getMatcherOnlyRules() Rules {
	return matcherOnlyRules
}
