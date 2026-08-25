package pint

import (
	"fmt"
	"strings"
)

// Unit is a parsed unit bound to a registry (magnitude 1).
type Unit struct {
	reg   *Registry
	units UnitsContainer
}

func (r *Registry) Unit(s string) (Unit, error) {
	u, err := r.ParseUnits(s)
	if err != nil {
		return Unit{}, err
	}
	return Unit{reg: r, units: u}, nil
}

func (u Unit) String() string { return u.units.String() }

func (u Unit) Units() UnitsContainer { return u.units }

func (u Unit) Quantity(mag float64) Quantity {
	return Quantity{reg: u.reg, mag: mag, units: u.units}
}

func (u Unit) Dimensionality() (UnitsContainer, error) {
	return u.reg.dimensionality(u.units)
}

func (u Unit) Compatible(other Unit) bool {
	a, err := u.Dimensionality()
	if err != nil {
		return false
	}
	b, err := other.Dimensionality()
	if err != nil {
		return false
	}
	return a.Equal(b)
}

func (u Unit) Mul(o Unit) Unit {
	return Unit{reg: u.reg, units: u.units.Mul(o.units)}
}

func (u Unit) Div(o Unit) Unit {
	return Unit{reg: u.reg, units: u.units.Div(o.units)}
}

func (u Unit) Pow(p float64) Unit {
	return Unit{reg: u.reg, units: u.units.Pow(p)}
}

func (u Unit) Format(spec string) string {
	switch spec {
	case "~", "compact":
		return u.units.compactString()
	case "D", "debug":
		return fmt.Sprintf("<Unit(%s)>", u.units.String())
	default:
		return u.units.String()
	}
}

func formatQuantity(q Quantity, spec string) string {
	us := q.units.String()
	if strings.Contains(spec, "~") {
		us = q.units.compactString()
	}
	if us == "" || us == "dimensionless" {
		return fmt.Sprintf("%g", q.mag)
	}
	return fmt.Sprintf("%g %s", q.mag, us)
}
