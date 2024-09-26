package logstorage

import (
	"math/rand"
	"slices"
	"strings"
	"sync"
	"testing"
)

func expectParsePipeFailure(t *testing.T, pipeStr string) {
	t.Helper()

	lex := newLexer(pipeStr)
	p, err := parsePipe(lex)
	if err == nil && lex.isEnd() {
		t.Fatalf("expecting error when parsing [%s]; parsed result: [%s]", pipeStr, p)
	}
}

func expectParsePipeSuccess(t *testing.T, pipeStr string) {
	t.Helper()

	lex := newLexer(pipeStr)
	p, err := parsePipe(lex)
	if err != nil {
		t.Fatalf("cannot parse [%s]: %s", pipeStr, err)
	}
	if !lex.isEnd() {
		t.Fatalf("unexpected tail after parsing [%s]: [%s]", pipeStr, lex.s)
	}

	pipeStrResult := p.String()
	if pipeStrResult != pipeStr {
		t.Fatalf("unexpected string representation of pipe; got\n%s\nwant\n%s", pipeStrResult, pipeStr)
	}
}

func expectPipeResults(t *testing.T, pipeStr string, rows, rowsExpected [][]Field) {
	t.Helper()

	lex := newLexer(pipeStr)
	p, err := parsePipe(lex)
	if err != nil {
		t.Fatalf("unexpected error when parsing %q: %s", pipeStr, err)
	}

	workersCount := 5
	stopCh := make(chan struct{})
	cancel := func() {}
	ppTest := newTestPipeProcessor()
	pp := p.newPipeProcessor(workersCount, stopCh, cancel, ppTest)

	brw := newTestBlockResultWriter(workersCount, pp)
	for _, row := range rows {
		brw.writeRow(row)
	}
	brw.flush()
	pp.flush()

	ppTest.expectRows(t, rowsExpected)
}

func newTestBlockResultWriter(workersCount int, ppNext pipeProcessor) *testBlockResultWriter {
	return &testBlockResultWriter{
		workersCount: workersCount,
		ppNext:       ppNext,
	}
}

type testBlockResultWriter struct {
	workersCount int
	ppNext       pipeProcessor
	rcs          []resultColumn
	br           blockResult

	rowsCount int
}

func (brw *testBlockResultWriter) writeRow(row []Field) {
	if !brw.areSameFields(row) {
		brw.flush()

		brw.rcs = brw.rcs[:0]
		for _, field := range row {
			brw.rcs = appendResultColumnWithName(brw.rcs, field.Name)
		}
	}

	for i, field := range row {
		brw.rcs[i].addValue(field.Value)
	}
	brw.rowsCount++
	if rand.Intn(5) == 0 {
		brw.flush()
	}
}

func (brw *testBlockResultWriter) areSameFields(row []Field) bool {
	if len(brw.rcs) != len(row) {
		return false
	}
	for i, rc := range brw.rcs {
		if rc.name != row[i].Name {
			return false
		}
	}
	return true
}

func (brw *testBlockResultWriter) flush() {
	brw.br.setResultColumns(brw.rcs, brw.rowsCount)
	brw.rowsCount = 0
	workerID := rand.Intn(brw.workersCount)
	brw.ppNext.writeBlock(uint(workerID), &brw.br)
	brw.br.reset()
	for i := range brw.rcs {
		brw.rcs[i].resetValues()
	}
}

func newTestPipeProcessor() *testPipeProcessor {
	return &testPipeProcessor{}
}

type testPipeProcessor struct {
	resultRowsLock sync.Mutex
	resultRows     [][]Field
}

func (pp *testPipeProcessor) writeBlock(_ uint, br *blockResult) {
	cs := br.getColumns()
	var columnValues [][]string
	for _, c := range cs {
		values := c.getValues(br)
		columnValues = append(columnValues, values)
	}

	for i := 0; i < br.rowsLen; i++ {
		row := make([]Field, len(columnValues))
		for j, values := range columnValues {
			r := &row[j]
			r.Name = strings.Clone(cs[j].name)
			r.Value = strings.Clone(values[i])
		}
		pp.resultRowsLock.Lock()
		pp.resultRows = append(pp.resultRows, row)
		pp.resultRowsLock.Unlock()
	}
}

func (pp *testPipeProcessor) flush() error {
	return nil
}

func (pp *testPipeProcessor) expectRows(t *testing.T, expectedRows [][]Field) {
	t.Helper()

	assertRowsEqual(t, pp.resultRows, expectedRows)
}

func assertRowsEqual(t *testing.T, resultRows, expectedRows [][]Field) {
	t.Helper()

	if len(resultRows) != len(expectedRows) {
		t.Fatalf("unexpected number of rows; got %d; want %d\nrows got\n%s\nrows expected\n%s",
			len(resultRows), len(expectedRows), rowsToString(resultRows), rowsToString(expectedRows))
	}

	sortTestRows(resultRows)
	sortTestRows(expectedRows)

	for i, resultRow := range resultRows {
		expectedRow := expectedRows[i]
		if len(resultRow) != len(expectedRow) {
			t.Fatalf("unexpected number of fields at row #%d; got %d; want %d\nrow got\n%s\nrow expected\n%s",
				i, len(resultRow), len(expectedRow), rowToString(resultRow), rowToString(expectedRow))
		}
		for j, resultField := range resultRow {
			expectedField := expectedRow[j]
			if resultField.Name != expectedField.Name {
				t.Fatalf("unexpected field name at row #%d; got %q; want %q\nrow got\n%s\nrow expected\n%s",
					i, resultField.Name, expectedField.Name, rowToString(resultRow), rowToString(expectedRow))
			}
			if resultField.Value != expectedField.Value {
				t.Fatalf("unexpected value for field %q at row #%d; got %q; want %q\nrow got\n%s\nrow expected\n%s",
					resultField.Name, i, resultField.Value, expectedField.Value, rowToString(resultRow), rowToString(expectedRow))
			}
		}
	}
}

func sortTestRows(rows [][]Field) {
	for _, row := range rows {
		sortTestFields(row)
	}
	slices.SortFunc(rows, func(a, b []Field) int {
		reverse := false
		if len(a) > len(b) {
			reverse = true
			a, b = b, a
		}
		for i, fA := range a {
			fB := b[i]
			result := cmpTestFields(fA, fB)
			if result == 0 {
				continue
			}
			if reverse {
				result = -result
			}
			return result
		}
		if len(a) == len(b) {
			return 0
		}
		if reverse {
			return 1
		}
		return -1
	})
}

func sortTestFields(fields []Field) {
	slices.SortFunc(fields, cmpTestFields)
}

func cmpTestFields(a, b Field) int {
	if a.Name == b.Name {
		if a.Value == b.Value {
			return 0
		}
		if a.Value < b.Value {
			return -1
		}
		return 1
	}
	if a.Name < b.Name {
		return -1
	}
	return 1
}

func rowsToString(rows [][]Field) string {
	a := make([]string, len(rows))
	for i, row := range rows {
		a[i] = rowToString(row)
	}
	return strings.Join(a, "\n")
}

func rowToString(row []Field) string {
	a := make([]string, len(row))
	for i, f := range row {
		a[i] = f.String()
	}
	return "{" + strings.Join(a, ",") + "}"
}
