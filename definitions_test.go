package pint

import (
	"math"
	"testing"
)

func TestDefinitionInvalid(t *testing.T) {
	for _, line := range []string{"x = [time] * meter", "[x] = [time] * meter"} {
		_, err := ParseDefinition(line)
		if err == nil {
			t.Fatalf("expected error for %q", line)
		}
	}
}

func TestDefinitionPrefix(t *testing.T) {
	for _, line := range []string{"m- = 1e-3", "m- = 10**-3", "m- = 0.001"} {
		d, err := ParseDefinition(line)
		if err != nil {
			t.Fatal(err)
		}
		if d.Kind != "prefix" || d.Name != "m" {
			t.Fatalf("%+v", d)
		}
		if d.Converter.ToReference(1000) != 1 {
			t.Fatalf("to_ref %g", d.Converter.ToReference(1000))
		}
	}
	d, err := ParseDefinition("kilo- = 1e-3 = k-")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "kilo" || d.Symbol != "k" {
		t.Fatalf("%+v", d)
	}
}

func TestDefinitionBaseUnit(t *testing.T) {
	d, err := ParseDefinition("meter = [length]")
	if err != nil {
		t.Fatal(err)
	}
	if !d.IsBase || !d.Reference.Equal(unitPair("[length]", 1)) {
		t.Fatalf("%+v", d)
	}
}

func TestDefinitionUnit(t *testing.T) {
	d, err := ParseDefinition("coulomb = ampere * second")
	if err != nil {
		t.Fatal(err)
	}
	if d.IsBase || d.Converter.Scale() != 1 {
		t.Fatal(d)
	}
	d, err = ParseDefinition("degF = 9 / 5 * kelvin; offset: 255.372222")
	if err != nil {
		t.Fatal(err)
	}
	oc, ok := d.Converter.(OffsetConverter)
	if !ok || math.Abs(oc.Scale()-1.8) > 1e-12 || math.Abs(oc.Offset()-255.372222) > 1e-9 {
		t.Fatalf("%+v", oc)
	}
}

func TestDefinitionLogUnit(t *testing.T) {
	d, err := ParseDefinition("decibel = 1 ; logbase: 10; logfactor: 10 = dB")
	if err != nil {
		t.Fatal(err)
	}
	lc, ok := d.Converter.(LogarithmicConverter)
	if !ok || lc.Logbase() != 10 || lc.Logfactor() != 10 {
		t.Fatalf("%+v", d.Converter)
	}
}

func TestDefinitionDimension(t *testing.T) {
	d, err := ParseDefinition("[speed] = [length]/[time]")
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != "dimension" || d.Reference.Get("[length]") != 1 || d.Reference.Get("[time]") != -1 {
		t.Fatalf("%+v", d)
	}
}

func TestDefinitionAlias(t *testing.T) {
	d, err := ParseDefinition("@alias meter = metro = metr")
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != "alias" || d.Name != "meter" || len(d.Aliases) != 2 {
		t.Fatalf("%+v", d)
	}
}
