package pint

import "testing"

func TestInferBaseUnit(t *testing.T) {
	r := mustReg(t)
	u, err := r.ParseUnits("millimeter * nanometer")
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.InferBaseUnit(u)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := r.ParseUnits("meter**2")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestInferAddingToZero(t *testing.T) {
	r := mustReg(t)
	u, err := r.ParseUnits("m * mm / m / um * s")
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.InferBaseUnit(u)
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("second") != 1 || got.Len() != 1 {
		t.Fatalf("got %s", got)
	}
}

func TestToCompact(t *testing.T) {
	r := mustReg(t)
	q, err := r.Parse("1e9 meter * millimeter / second / millisecond")
	if err != nil {
		t.Fatal(err)
	}
	c, err := q.ToCompact()
	if err != nil {
		t.Fatal(err)
	}
	if c.Magnitude() == 0 {
		t.Fatal(c)
	}
}

func TestPiTheoremSimple(t *testing.T) {
	r := mustReg(t)
	groups, err := r.PiTheorem(map[string]string{"V": "m/s", "T": "s", "L": "m"})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) == 0 {
		t.Fatal("expected a dimensionless group")
	}
}

func TestPintEvalExpressions(t *testing.T) {
	r := mustReg(t)
	cases := []string{"3", "1 + 2", "2 * 3 + 4", "2 * (3 + 4)", "-1", "3 * -1", "3e-1"}
	for _, s := range cases {
		if _, err := parseScaledUnits(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	q, err := r.Parse("3 kg")
	if err != nil {
		t.Fatal(err)
	}
	if q.Magnitude() != 3 {
		t.Fatal(q)
	}
}
