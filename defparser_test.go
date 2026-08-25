package pint

import (
	"testing"

	"github.com/dendrascience/pint-go/defs"
)

func TestParseDefinitionFileOnly(t *testing.T) {
	b, err := defs.Files.ReadFile(defs.DefaultFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("read %d bytes", len(b))
	ds, err := parseDefinitionFile(string(b), defs.DefaultFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("parsed %d defs", len(ds))
}
