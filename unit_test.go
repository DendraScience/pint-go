package pint

import (
	"math"
	"testing"
)

func TestUnitCreation(t *testing.T) {
	r := mustReg(t)
	u, err := r.Unit("meter")
	if err != nil {
		t.Fatal(err)
	}
	if u.String() != "meter" && u.Units().Get("meter") != 1 {
		t.Fatalf("%s", u)
	}
}

func TestUnitMulDivPow(t *testing.T) {
	r := mustReg(t)
	m, _ := r.Unit("meter")
	s, _ := r.Unit("second")
	p := m.Div(s)
	if p.Units().Get("meter") != 1 || p.Units().Get("second") != -1 {
		t.Fatal(p)
	}
	sq := m.Pow(2)
	if sq.Units().Get("meter") != 2 {
		t.Fatal(sq)
	}
}

func TestParseAliasPluralPrefix(t *testing.T) {
	r := mustReg(t)
	for _, s := range []string{"meter", "metre", "meters", "kilometer", "kilometre"} {
		if _, err := r.ParseUnits(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

func TestParseMulDiv(t *testing.T) {
	r := mustReg(t)
	u, err := r.ParseUnits("meter*meter")
	if err != nil {
		t.Fatal(err)
	}
	if u.Get("meter") != 2 {
		t.Fatal(u)
	}
	u, err = r.ParseUnits("meter/second")
	if err != nil {
		t.Fatal(err)
	}
	if u.Get("meter") != 1 || u.Get("second") != -1 {
		t.Fatal(u)
	}
}

func TestParsePretty(t *testing.T) {
	r := mustReg(t)
	u, err := r.ParseUnits("meter²")
	if err != nil {
		t.Fatal(err)
	}
	if u.Get("meter") != 2 {
		t.Fatalf("got %s", u)
	}
}

func TestParseFactor(t *testing.T) {
	r := mustReg(t)
	q, err := r.Parse("4.2*meter")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(q.Magnitude()-4.2) > 1e-12 {
		t.Fatal(q)
	}
}

func TestParsePi(t *testing.T) {
	r := mustReg(t)
	q, err := r.Parse("2 * pi * radian")
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.To("radian")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got.Magnitude()-2*math.Pi) > 1e-9 {
		t.Fatalf("got %g", got.Magnitude())
	}
}

func TestParseScientificUnicode(t *testing.T) {
	r := mustReg(t)
	q, err := r.Parse("4.2×10⁻¹² ft/s")
	if err != nil {
		t.Fatal(err)
	}
	if q.Magnitude() == 0 {
		t.Fatal(q)
	}
}

func TestDimensionalityLength(t *testing.T) {
	r := mustReg(t)
	d, err := r.Dimensionality("meter")
	if err != nil {
		t.Fatal(err)
	}
	if d.Get("[length]") != 1 {
		t.Fatal(d)
	}
}

func TestCaseSensitivity(t *testing.T) {
	r := mustReg(t)
	if _, err := r.ParseUnits("Meter"); err == nil {
		t.Fatal("case-sensitive default should reject Meter")
	}
	r2, err := NewRegistry(WithCaseSensitive(false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r2.ParseUnits("Meter"); err != nil {
		t.Fatal(err)
	}
}

func TestRedefinition(t *testing.T) {
	r := mustReg(t)
	if err := r.Define("meter = [length]"); err == nil {
		t.Fatal("expected redefinition error")
	}
}

func TestLoadTiny(t *testing.T) {
	r := NewEmptyRegistry()
	if err := r.Load(`
kilo- = 1e3 = k-
meter = [length] = m
`); err != nil {
		t.Fatal(err)
	}
	v, err := r.Convert(1, "km", "m")
	if err != nil {
		t.Fatal(err)
	}
	if v != 1000 {
		t.Fatal(v)
	}
}
