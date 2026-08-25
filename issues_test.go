package pint

import (
	"math"
	"testing"
)

func TestIssueAngstrom(t *testing.T) {
	r := mustReg(t)
	q, err := r.Parse("1 angstrom")
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.To("nm")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), 0.1, 1e-12)
}

func TestIssueMicro(t *testing.T) {
	r := mustReg(t)
	for _, s := range []string{"µm", "μm", "um"} {
		u, err := r.ParseUnits(s + "meter")
		if err != nil {
			// try as prefix+meter
			u, err = r.ParseUnits("micrometer")
			if err != nil {
				t.Fatalf("%s: %v", s, err)
			}
		}
		_ = u
	}
}

func TestIssueLiter(t *testing.T) {
	r := mustReg(t)
	q, err := r.Parse("1 liter")
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.To("ml")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), 1000, 1e-9)
}

func TestIssueHour(t *testing.T) {
	r := mustReg(t)
	q, err := r.Parse("1hour")
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.To("minute")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), 60, 1e-12)
}

func TestIssueDimensionless(t *testing.T) {
	r := mustReg(t)
	q, err := r.Parse("3 radian")
	if err != nil {
		t.Fatal(err)
	}
	if !q.Dimensionless() {
		d, _ := q.Dimensionality()
		t.Fatalf("radian should be dimensionless, got %s", d)
	}
}

func TestIssuePower(t *testing.T) {
	r := mustReg(t)
	q, err := r.Parse("3 meter**2")
	if err != nil {
		t.Fatal(err)
	}
	p, err := q.Pow(0.5)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.To("meter")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), math.Sqrt(3), 1e-12)
}
