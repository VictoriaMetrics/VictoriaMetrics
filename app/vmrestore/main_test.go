package main

import (
	"testing"
)

func TestParseRestorePartitions(t *testing.T) {
	// Empty value means restore the whole backup.
	re, err := parseRestorePartitions("")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if re != nil {
		t.Fatalf("expected nil regexp for empty value, got %v", re)
	}

	// Valid regexp must fully match the partition name.
	re, err = parseRestorePartitions("2026_01")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !re.MatchString("2026_01") {
		t.Fatalf("regexp must match 2026_01")
	}
	if re.MatchString("2026_010") || re.MatchString("x2026_01") {
		t.Fatalf("regexp must fully match the partition name")
	}

	// Alternation.
	re, err = parseRestorePartitions("2026_(01|02)")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !re.MatchString("2026_01") || !re.MatchString("2026_02") {
		t.Fatalf("regexp must match both 2026_01 and 2026_02")
	}
	if re.MatchString("2026_03") {
		t.Fatalf("regexp must not match 2026_03")
	}

	// Invalid regexp must return an error.
	if _, err := parseRestorePartitions("2026_(01"); err == nil {
		t.Fatalf("expected error for invalid regexp, got nil")
	}
}
