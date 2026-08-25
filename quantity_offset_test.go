package pint

import (
	"math"
	"testing"
)

func TestOffsetTable(t *testing.T) {
	r := mustReg(t)
	cases := []struct {
		v          float64
		from, to   string
		want, rtol float64
	}{
		{0, "degC", "kelvin", 273.15, 1e-12},
		{100, "degC", "kelvin", 373.15, 1e-12},
		{32, "degF", "degC", 0, 1e-9},
		{100, "degC", "degF", 212, 1e-9},
		{0, "kelvin", "degC", -273.15, 1e-12},
		{12, "kelvin", "degR", 21.6, 1e-9},
	}
	for _, tc := range cases {
		got, err := r.Convert(tc.v, tc.from, tc.to)
		if err != nil {
			t.Fatalf("%g %s->%s: %v", tc.v, tc.from, tc.to, err)
		}
		if math.Abs(got-tc.want) > tc.rtol+1e-6*math.Abs(tc.want) {
			t.Fatalf("%g %s->%s = %g want %g", tc.v, tc.from, tc.to, got, tc.want)
		}
	}
}

func TestOffsetDeltaTable(t *testing.T) {
	r := mustReg(t)
	v, err := r.Convert(100, "delta_degC", "delta_degF")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(v-180) > 1e-9 {
		t.Fatal(v)
	}
	v, err = r.Convert(100, "kelvin", "delta_degF")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(v-180) > 1e-9 {
		t.Fatal(v)
	}
}

func TestOffsetCalculusError(t *testing.T) {
	r := mustReg(t)
	a, _ := r.Quantity(1, "degC")
	b, _ := r.Quantity(1, "degC")
	if _, err := a.Mul(b); err == nil {
		t.Fatal("expected offset calculus error")
	}
}

func TestOffsetMulAuto(t *testing.T) {
	r, err := NewRegistry(WithAutoOffsetToBase(true))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.Quantity(0, "degC")
	b, _ := r.Quantity(2, "meter")
	p, err := a.Mul(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.To("kelvin * meter")
	if err != nil {
		t.Fatalf("%s: %v", p, err)
	}
	assertClose(t, got.Magnitude(), 273.15*2, 1e-9)
}

func TestConverterAffineTemp(t *testing.T) {
	r := mustReg(t)
	c, err := r.Converter("degC", "degF")
	if err != nil {
		t.Fatal(err)
	}
	v, err := c.Convert(20)
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, 68, 1e-9)
	src := []float64{0, 100}
	dst := make([]float64, 2)
	if err := c.ConvertN(dst, src); err != nil {
		t.Fatal(err)
	}
	assertClose(t, dst[0], 32, 1e-9)
	assertClose(t, dst[1], 212, 1e-9)
}
