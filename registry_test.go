package pint

import (
	"math"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	q, err := r.Parse("3 meter")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, err := q.To("millimeter")
	if err != nil {
		t.Fatalf("To: %v", err)
	}
	if math.Abs(got.Magnitude()-3000) > 1e-9 {
		t.Fatalf("3 meter -> mm = %g, want 3000", got.Magnitude())
	}
}

func TestConvertLength(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	v, err := r.Convert(1, "inch", "centimeter")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(v-2.54) > 1e-12 {
		t.Fatalf("1 inch = %g cm, want 2.54", v)
	}
}

func TestTemperatureAbsolute(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	v, err := r.Convert(20, "degC", "degF")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(v-68) > 1e-9 {
		t.Fatalf("20 degC = %g degF, want 68", v)
	}
}

func TestTemperatureDelta(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	v, err := r.Convert(5, "delta_degC", "delta_degF")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(v-9) > 1e-9 {
		t.Fatalf("5 delta_degC = %g delta_degF, want 9", v)
	}
}
