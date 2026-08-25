package pint

import "testing"

func TestContextSpectroscopy(t *testing.T) {
	r := mustReg(t)
	if err := r.EnableContext("sp", nil); err != nil {
		t.Fatal(err)
	}
	defer r.DisableContext()
	q, err := r.Quantity(532, "nm")
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.To("terahertz")
	if err != nil {
		t.Fatal(err)
	}
	// ~563.5 THz
	if got.Magnitude() < 500 || got.Magnitude() > 630 {
		t.Fatalf("532 nm -> THz = %g", got.Magnitude())
	}
}

func TestContextUnknown(t *testing.T) {
	r := mustReg(t)
	if err := r.EnableContext("nope", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestContextDefined(t *testing.T) {
	r := mustReg(t)
	if _, err := r.ContextByName("sp"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ContextByName("spectroscopy"); err != nil {
		t.Fatal(err)
	}
}

func TestContextWavelengthToKcal(t *testing.T) {
	r := mustReg(t)
	for _, name := range []string{"Gau", "ESU", "sp", "energy", "boltzmann"} {
		if err := r.EnableContext(name, nil); err != nil {
			t.Fatal(err)
		}
	}
	q, err := r.Parse("540nm")
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.To("kcal/mol")
	if err != nil {
		t.Fatal(err)
	}
	// pint-convert 540nm kcal/mol ≈ 52.947
	if got.Magnitude() < 52 || got.Magnitude() > 54 {
		t.Fatalf("540 nm -> kcal/mol = %g", got.Magnitude())
	}
}
