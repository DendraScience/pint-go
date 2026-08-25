package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInventoryComplete(t *testing.T) {
	pint := os.Getenv("PINT_DIR")
	if pint == "" {
		_, this, _, _ := runtime.Caller(0)
		guess := filepath.Join(filepath.Dir(this), "..", "..", "..", "pint")
		if _, err := os.Stat(filepath.Join(guess, "pint", "testsuite")); err == nil {
			pint = guess
		}
	}
	if pint == "" {
		t.Skip("PINT_DIR not set and sibling pint checkout not found")
	}
	suite := filepath.Join(pint, "pint", "testsuite")
	tests, err := extractTests(suite)
	if err != nil {
		t.Fatal(err)
	}
	_, this, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(this), "..", "..")
	inv, err := parseInventoryIDs(filepath.Join(root, "testdata", "inventory.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, id := range tests {
		if _, ok := inv[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("inventory missing %d Pint tests (first: %s)", len(missing), missing[0])
	}
}
