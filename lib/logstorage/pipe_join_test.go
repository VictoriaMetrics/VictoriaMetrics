package logstorage

import (
	"testing"
)

func TestParsePipeJoinSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`join by (foo) (error)`)
	f(`join by (foo, bar) (a:b | fields x, y)`)
	f(`join by (foo) (a:b) prefix c`)
	f(`join by (foo) (bar | join by (x, z) (y))`)
	f(`join by (x) (y) inner`)
	f(`join by (x) (y) inner prefix a.b`)
}

func TestParsePipeJoinFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`join`)
	f(`join by () (abc)`)
	f(`join by (*) (abc)`)
	f(`join by (f, *) (abc)`)
	f(`join by (x)`)
	f(`join by`)
	f(`join (`)
	f(`join by (foo) bar`)
	f(`join by (x) ()`)
	f(`join by (x) (`)
	f(`join by (x) (abc`)
	f(`join (x) (y) prefix`)
	f(`join (x) (y) prefix |`)
	f(`join by (x) (y) prefix x inner`)
}

func TestPipeJoinUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("join on (x, y) (abc)", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("join on (x, y) (abc) inner", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("join on (x, y) (abc)", "*", "f2,x", "*", "f2")

	// needed fields do not intersect with src
	f("join on (x, y) (abc)", "f1,f2", "", "f1,f2,x,y", "")

	// needed fields intersect with src
	f("join on (x, y) (abc)", "f2,x", "", "f2,x,y", "")
}
