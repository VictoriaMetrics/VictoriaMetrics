package metricsql

// ExpandWithExprs expands WITH expressions inside q and returns the resulting
// PromQL without WITH expressions.
func ExpandWithExprs(q string) (string, error) {
	e, err := Parse(q)
	if err != nil {
		return "", err
	}
	buf := e.AppendString(nil)
	return string(buf), nil
}
