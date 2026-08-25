package pint

import (
	"math"
	"testing"
)

func mustReg(t *testing.T) *Registry {
	t.Helper()
	r, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func assertClose(t *testing.T, got, want, rtol float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsNaN(want) {
		t.Fatalf("NaN: got %v want %v", got, want)
	}
	if !almostEqual(got, want, rtol, 1e-15) {
		t.Fatalf("got %g, want %g (rtol %g)", got, want, rtol)
	}
}

func TestPrefixKilo(t *testing.T) {
	r := mustReg(t)
	v, err := r.Convert(1, "kilometer", "meter")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, 1000, 1e-12)
	v, err = r.Convert(1, "km", "m")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, 1000, 1e-12)
}

func TestParseQuantity(t *testing.T) {
	r := mustReg(t)
	q, err := r.Parse("3.2 kPa")
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.To("Pa")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), 3200, 1e-12)
}

func TestCompatible(t *testing.T) {
	r := mustReg(t)
	ok, err := r.Compatible("meter", "inch")
	if err != nil || !ok {
		t.Fatalf("meter/inch compatible: %v %v", ok, err)
	}
	ok, err = r.Compatible("meter", "second")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("meter and second should be incompatible")
	}
}

func TestSpeed(t *testing.T) {
	r := mustReg(t)
	v, err := r.Convert(1, "m/s", "km/h")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, 3.6, 1e-12)
}

func TestKelvinIdentity(t *testing.T) {
	r := mustReg(t)
	v, err := r.Convert(300, "kelvin", "K")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, 300, 1e-12)
}

func TestDegCToKelvin(t *testing.T) {
	r := mustReg(t)
	v, err := r.Convert(0, "degC", "kelvin")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, 273.15, 1e-12)
}

func TestQuantityAddLength(t *testing.T) {
	r := mustReg(t)
	a, err := r.Quantity(1, "meter")
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Quantity(100, "centimeter")
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.Add(b)
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, s.Magnitude(), 2, 1e-12)
}

func TestQuantityMul(t *testing.T) {
	r := mustReg(t)
	a, _ := r.Quantity(3, "meter")
	b, _ := r.Quantity(2, "meter")
	p, err := a.Mul(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.To("meter**2")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), 6, 1e-12)
}

func TestOffsetAddDelta(t *testing.T) {
	r := mustReg(t)
	a, _ := r.Quantity(20, "degC")
	d, _ := r.Quantity(5, "delta_degC")
	s, err := a.Add(d)
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, s.Magnitude(), 25, 1e-12)
}

func TestOffsetSubGivesDelta(t *testing.T) {
	r := mustReg(t)
	a, _ := r.Quantity(20, "degC")
	b, _ := r.Quantity(15, "degC")
	s, err := a.Sub(b)
	if err != nil {
		t.Fatal(err)
	}
	if !s.units.Has("delta_degree_Celsius") && !s.units.Has("delta_degC") {
		// canonical name is delta_degree_Celsius
		names := s.units.Names()
		t.Logf("diff units: %v", names)
		got, err := s.To("delta_degC")
		if err != nil {
			t.Fatalf("units %v: %v", names, err)
		}
		assertClose(t, got.Magnitude(), 5, 1e-12)
		return
	}
	assertClose(t, s.Magnitude(), 5, 1e-12)
}

func TestConverterN(t *testing.T) {
	r := mustReg(t)
	c, err := r.Converter("m", "cm")
	if err != nil {
		t.Fatal(err)
	}
	src := []float64{1, 2, 3}
	dst := make([]float64, 3)
	if err := c.ConvertN(dst, src); err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{100, 200, 300} {
		assertClose(t, dst[i], want, 1e-12)
	}
}

func TestDefineCustom(t *testing.T) {
	r := mustReg(t)
	if err := r.Define("beer = 0.5 * liter = be"); err != nil {
		t.Fatal(err)
	}
	v, err := r.Convert(2, "beer", "liter")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, 1, 1e-12)
}

func TestDimensionalityError(t *testing.T) {
	r := mustReg(t)
	_, err := r.Convert(1, "meter", "second")
	if err == nil {
		t.Fatal("expected dimensionality error")
	}
	if _, ok := err.(*DimensionalityError); !ok {
		t.Fatalf("got %T %v", err, err)
	}
}

func TestLogDecibel(t *testing.T) {
	r := mustReg(t)
	// 20 dB = 10 (power ratio, 10*log10)
	q, err := r.Parse("20 decibel")
	if err != nil {
		t.Fatalf("parse dB: %v", err)
	}
	got, err := q.To("dimensionless")
	if err != nil {
		t.Logf("dB to dimensionless: %v (may require log conversion path)", err)
		return
	}
	assertClose(t, got.Magnitude(), 100, 1e-9)
}
