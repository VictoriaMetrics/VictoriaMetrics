package promrelabel

import (
	"bytes"
	"html"
	"testing"

	"github.com/valyala/fastjson"
)

// TestWriteRelabelDebugSupportFormats verifies the relabeling debug input, rules and output.
func TestWriteRelabelDebugSupportFormats(t *testing.T) {
	f := func(input, rule, expect string) {
		// execute
		outputWriter := bytes.NewBuffer(nil)
		writeRelabelDebug(outputWriter, false, "", input, rule, "json", nil)

		// the response is in JSON with HTML content, extract the `resultingLabels` in JSON and unescape it.
		resultingLabels := fastjson.GetString(outputWriter.Bytes(), `resultingLabels`)
		resultingLabels = html.UnescapeString(resultingLabels)

		// verify
		if resultingLabels != expect {
			t.Fatalf(`expected "%s", got "%s"`, expect, resultingLabels)
		}
	}

	// test pure parsing
	// ruleTestParsing rule should NOT drop anything. it should ask `writeRelabelDebug` to respond with whatever the input is (after parsing).
	ruleTestParsing := `
- action: labeldrop
  regex: "a_not_exist_label"
`
	f(`metric_name`, ruleTestParsing, `metric_name`)
	f(`metric_name{label1="value1"}`, ruleTestParsing, `metric_name{label1="value1"}`)
	f(`{__name__="metric_name", label1="value1"}`, ruleTestParsing, `metric_name{label1="value1"}`)
	f(`__name__="metric_name", label1="value1"`, ruleTestParsing, `metric_name{label1="value1"}`)
	f(`_name__="metric_name"`, ruleTestParsing, `{_name__="metric_name"}`)

	// special case: incorrect input format
	f(`{_name__="metric_name"`, ruleTestParsing, ``)
	f(`_name__="metric_name}"`, ruleTestParsing, ``)
	f(`metrics_name}"`, ruleTestParsing, ``)
}
