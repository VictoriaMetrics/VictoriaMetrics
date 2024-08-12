package snapshotutil

import (
	"testing"
)

func TestValidate_Failure(t *testing.T) {
	f := func(snapshotName string) {
		t.Helper()

		err := Validate(snapshotName)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// empty snapshot name
	f("")

	// short snapshot name
	f("foo")

	// short first part of the snapshot name
	f("2022050312163-16EB56ADB4110CF2")

	// invalid time part snapshot name
	f("00000000000000-16EB56ADB4110CF2")

	// not enough parts of the snapshot name
	f("2022050312163816EB56ADB4110CF2")
}

func TestValidate_Success(t *testing.T) {
	f := func(snapshotName string) {
		t.Helper()

		err := Validate(snapshotName)
		if err != nil {
			t.Fatalf("checkSnapshotName() error: %s", err)
		}
	}

	// short second part of the snapshot name - this is OK
	f("20220503121638-16EB56ADB4110CF")

	//correct snapshot name
	f("20220503121638-16EB56ADB4110CF2")
}
