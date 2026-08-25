package pint

import (
	"strings"
	"testing"
)

func TestErrorMessages(t *testing.T) {
	if (&DefinitionSyntaxError{Msg: "foo"}).Error() != "foo" {
		t.Fatal("syntax")
	}
	re := &RedefinitionError{Name: "foo", DefinitionType: "bar"}
	if re.Error() != "Cannot redefine 'foo' (bar)" {
		t.Fatal(re.Error())
	}
	ue := &UndefinedUnitError{Names: []string{"meter"}}
	if !strings.Contains(ue.Error(), "meter") {
		t.Fatal(ue.Error())
	}
	de := &DimensionalityError{Units1: "a", Units2: "b"}
	if !strings.Contains(de.Error(), "Cannot convert") {
		t.Fatal(de.Error())
	}
	oe := &OffsetUnitCalculusError{Units: []string{"kilogram"}}
	if !strings.Contains(oe.Error(), "offset") {
		t.Fatal(oe.Error())
	}
	le := &LogarithmicUnitCalculusError{Units: []string{"decibel"}}
	if !strings.Contains(le.Error(), "logarithmic") {
		t.Fatal(le.Error())
	}
}
