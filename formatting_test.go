package pint

import (
	"strings"
	"testing"
)

func TestFormatDefault(t *testing.T) {
	r := mustReg(t)
	q, _ := r.Quantity(3, "meter")
	s := q.String()
	if !strings.Contains(s, "3") || !strings.Contains(s, "meter") {
		t.Fatal(s)
	}
}

func TestFormatCompact(t *testing.T) {
	r := mustReg(t)
	u, _ := r.Unit("meter**2/second")
	s := u.Format("~")
	if s == "" {
		t.Fatal("empty compact")
	}
	q, _ := r.Quantity(1.5, "meter")
	if !strings.Contains(q.Format("~"), "1.5") {
		t.Fatal(q.Format("~"))
	}
}

func TestFormatCaret(t *testing.T) {
	r := mustReg(t)
	u, _ := r.Unit("meter**-1")
	s := u.String()
	if !strings.Contains(s, "meter") {
		t.Fatal(s)
	}
}
