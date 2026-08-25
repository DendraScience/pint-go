package pint

import "testing"

func TestUnitsContainerCreation(t *testing.T) {
	x := NewUnitsContainer(map[string]float64{"meter": 1, "second": 2})
	y := NewUnitsContainer(map[string]float64{"meter": 1, "second": 2})
	if !x.Equal(y) || x.Get("meter") != 1 {
		t.Fatal(x)
	}
}

func TestUnitsContainerString(t *testing.T) {
	if NewUnitsContainer(nil).String() != "dimensionless" {
		t.Fatal("empty")
	}
	x := NewUnitsContainer(map[string]float64{"meter": 1, "second": 2})
	if x.String() != "meter * second ** 2" {
		t.Fatalf("got %q", x.String())
	}
}

func TestUnitsContainerBool(t *testing.T) {
	if NewUnitsContainer(map[string]float64{"meter": 1}).Empty() {
		t.Fatal("should be nonempty")
	}
	if !NewUnitsContainer(nil).Empty() {
		t.Fatal("should be empty")
	}
}

func TestUnitsContainerArithmetic(t *testing.T) {
	x := unitPair("meter", 1)
	y := unitPair("second", 1)
	z := NewUnitsContainer(map[string]float64{"meter": 1, "second": -2})
	if !x.Mul(y).Equal(NewUnitsContainer(map[string]float64{"meter": 1, "second": 1})) {
		t.Fatal("mul")
	}
	if !x.Div(y).Equal(NewUnitsContainer(map[string]float64{"meter": 1, "second": -1})) {
		t.Fatal("div")
	}
	if !z.Pow(2).Equal(NewUnitsContainer(map[string]float64{"meter": 2, "second": -4})) {
		t.Fatal("pow")
	}
}

func TestPreprocessorSquareCube(t *testing.T) {
	cases := []struct{ in, want string }{
		{"bcd^3", "bcd**3"},
		{"bcd squared", "bcd**2"},
		{"bcd cubed", "bcd**3"},
		{"square bcd", "bcd**2"},
		{"cubic bcd", "bcd**3"},
		{"miles per hour", "miles/hour"},
		{"1,234,567", "1234567"},
		{"1hour", "1 hour"},
	}
	for _, tc := range cases {
		got := Preprocess(tc.in)
		if got != tc.want {
			t.Errorf("Preprocess(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
