package pint

import (
	"testing"

	"github.com/dendrascience/pint-go/defs"
)

func TestApplyEachDef(t *testing.T) {
	r := NewEmptyRegistry()
	b, err := defs.Files.ReadFile(defs.DefaultFile)
	if err != nil {
		t.Fatal(err)
	}
	ds, err := parseDefinitionFile(string(b), defs.DefaultFile)
	if err != nil {
		t.Fatal(err)
	}
	for i, d := range ds {
		if err := r.applyDef(d); err != nil {
			t.Fatalf("def %d kind=%d name=%q: %v", i, d.kind, d.name, err)
		}
	}
}
