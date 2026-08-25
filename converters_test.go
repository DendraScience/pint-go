package pint

import (
	"math"
	"testing"
)

func TestConverterScale(t *testing.T) {
	c := NewScaleConverter(20)
	if !c.IsMultiplicative() || c.IsLogarithmic() {
		t.Fatal("scale flags")
	}
	if c.FromReference(c.ToReference(100)) != 100 {
		t.Fatal("roundtrip")
	}
}

func TestConverterOffset(t *testing.T) {
	c := NewOffsetConverter(20, 2)
	if c.IsMultiplicative() || c.IsLogarithmic() {
		t.Fatal("offset flags")
	}
	if c.FromReference(c.ToReference(100)) != 100 {
		t.Fatal("roundtrip")
	}
}

func TestConverterLog(t *testing.T) {
	c := NewLogarithmicConverter(1, 10, 1)
	if c.IsMultiplicative() || !c.IsLogarithmic() {
		t.Fatal("log flags")
	}
	cases := []struct{ in, out float64 }{{0, 1}, {1, 10}, {2, 100}}
	for _, tc := range cases {
		if math.Abs(c.ToReference(tc.in)-tc.out) > 1e-9 {
			t.Fatalf("to_ref(%g)=%g want %g", tc.in, c.ToReference(tc.in), tc.out)
		}
		if math.Abs(c.FromReference(tc.out)-tc.in) > 1e-9 {
			t.Fatalf("from_ref(%g)=%g want %g", tc.out, c.FromReference(tc.out), tc.in)
		}
	}
}

func TestConverterFromModifiers(t *testing.T) {
	if _, ok := converterFromModifiers(1, nil).(ScaleConverter); !ok {
		t.Fatal("scale")
	}
	if c, ok := converterFromModifiers(2, map[string]float64{"offset": 3}).(OffsetConverter); !ok || c.Offset() != 3 {
		t.Fatal("offset")
	}
	if c, ok := converterFromModifiers(4, map[string]float64{"logbase": 5, "logfactor": 6}).(LogarithmicConverter); !ok || c.Logbase() != 5 {
		t.Fatal("log")
	}
}
