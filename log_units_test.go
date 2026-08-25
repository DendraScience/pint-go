package pint

import (
	"math"
	"testing"
)

func TestLogConvert(t *testing.T) {
	r := mustReg(t)
	v, err := r.Convert(0, "dBm", "dBu")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, 30, 1e-7)
	v, err = r.Convert(0, "dBW", "dBm")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, 30, 1e-7)
}

func TestDBConversion(t *testing.T) {
	r := mustReg(t)
	for _, tc := range []struct{ db, lin float64 }{
		{0, 1}, {-10, 0.1}, {10, 10}, {30, 1e3}, {60, 1e6},
	} {
		q, err := r.Quantity(tc.db, "dB")
		if err != nil {
			t.Fatal(err)
		}
		got, err := q.To("dimensionless")
		if err != nil {
			t.Fatal(err)
		}
		assertClose(t, got.Magnitude(), tc.lin, 1e-9)
	}
}

func TestOctaveDecade(t *testing.T) {
	r := mustReg(t)
	q, _ := r.Quantity(1, "octave")
	got, err := q.To("dimensionless")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), 2, 1e-9)
	q, _ = r.Quantity(1, "decade")
	got, err = q.To("dimensionless")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), 10, 1e-9)
}

func TestDBmToMW(t *testing.T) {
	r := mustReg(t)
	q, _ := r.Quantity(0, "dBm")
	got, err := q.To("mW")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), 1, 1e-9)
}

func TestLogArithmetic(t *testing.T) {
	r, err := NewRegistry(WithAutoOffsetToBase(true))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.Parse("10 dB")
	b, _ := r.Parse("20 dB")
	s, err := a.Add(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.To("dB")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), 30, 1e-9)
}

func TestLogMixRegularError(t *testing.T) {
	r := mustReg(t)
	a, _ := r.Quantity(-10, "dB")
	b, _ := r.Quantity(1, "cm")
	if _, err := a.Div(b); err == nil {
		t.Fatal("expected calculus error")
	}
}

func TestLogXfailCompound(t *testing.T) {
	t.Skip("Pint xfail: compound log unit multiply definition")
}

func TestLogXfailOctaveAdd(t *testing.T) {
	t.Skip("Pint xfail: frequency + octave")
}

func TestDecadeOctave(t *testing.T) {
	r := mustReg(t)
	q, _ := r.Quantity(1, "decade")
	got, err := q.To("octave")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), math.Log2(10), 1e-9)
}
