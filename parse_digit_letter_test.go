package pint

import "testing"

func TestParseUnitNameWaterDensity4C(t *testing.T) {
	r := mustReg(t)
	un, err := r.ParseUnitName("water_density_4C")
	if err != nil {
		t.Fatal(err)
	}
	if un.Name != "water_density_4C" {
		t.Fatalf("Name=%q", un.Name)
	}
}

func TestParseUnitNameKilowaterDensity4C(t *testing.T) {
	r := mustReg(t)
	un, err := r.ParseUnitName("kilowater_density_4C")
	if err != nil {
		t.Fatal(err)
	}
	if un.Name != "kilowater_density_4C" {
		t.Fatalf("Name=%q", un.Name)
	}
}

func TestParseUnitNameMeterH2O(t *testing.T) {
	r := mustReg(t)
	un, err := r.ParseUnitName("meter_H2O")
	if err != nil {
		t.Fatal(err)
	}
	if un.Name != "meter_H2O" {
		t.Fatalf("Name=%q", un.Name)
	}
}

func TestParseUnitsKeepsDigitLetterIdentInExpression(t *testing.T) {
	r := mustReg(t)
	u, err := r.ParseUnits("meter * water_density_4C")
	if err != nil {
		t.Fatal(err)
	}
	if u.String() != "meter * water_density_4C" {
		t.Fatalf("got %q", u.String())
	}
}

func TestPreprocessKeepsDigitLetterIdents(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1hour", "1 hour"},
		{"4degC", "4 degC"},
		{"water_density_4C", "water_density_4C"},
		{"kilowater_density_4C", "kilowater_density_4C"},
		{"meter_H2O", "meter_H2O"},
		{"water_density_60F", "water_density_60F"},
		{"1e3meter", "1e3 meter"},
	}
	for _, tc := range cases {
		got := Preprocess(tc.in)
		if got != tc.want {
			t.Errorf("Preprocess(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
