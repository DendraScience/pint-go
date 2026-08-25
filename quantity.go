package pint

import (
	"fmt"
	"math"
	"strings"
)

// Quantity is a magnitude together with units, bound to a Registry.
type Quantity struct {
	reg   *Registry
	mag   float64
	units UnitsContainer
}

func (q Quantity) Magnitude() float64    { return q.mag }
func (q Quantity) Units() UnitsContainer { return q.units }
func (q Quantity) Registry() *Registry   { return q.reg }

func (q Quantity) String() string {
	if q.units.Empty() {
		return fmt.Sprintf("%g dimensionless", q.mag)
	}
	return fmt.Sprintf("%g %s", q.mag, q.units.String())
}

// To converts to dest units.
func (q Quantity) To(dest string) (Quantity, error) {
	du, err := q.reg.ParseUnits(dest)
	if err != nil {
		return Quantity{}, err
	}
	v, err := q.reg.convert(q.mag, q.units, du)
	if err != nil {
		return Quantity{}, err
	}
	return Quantity{reg: q.reg, mag: v, units: du}, nil
}

// ToRootUnits converts to the registry's root (reference) units.
func (q Quantity) ToRootUnits() (Quantity, error) {
	rr, err := q.reg.rootUnits(q.units, false)
	if err != nil {
		return Quantity{}, err
	}
	v, err := q.reg.convert(q.mag, q.units, rr.units)
	if err != nil {
		return Quantity{}, err
	}
	return Quantity{reg: q.reg, mag: v, units: rr.units}, nil
}

// Dimensionality returns the quantity's dimensions.
func (q Quantity) Dimensionality() (UnitsContainer, error) {
	return q.reg.dimensionality(q.units)
}

func (q Quantity) Dimensionless() bool {
	d, err := q.Dimensionality()
	return err == nil && d.Empty()
}

func (q Quantity) Compatible(other Quantity) bool {
	a, err := q.Dimensionality()
	if err != nil {
		return false
	}
	b, err := other.Dimensionality()
	if err != nil {
		return false
	}
	return a.Equal(b)
}

func (q Quantity) Add(o Quantity) (Quantity, error) { return q.addSub(o, false) }
func (q Quantity) Sub(o Quantity) (Quantity, error) { return q.addSub(o, true) }

func (q Quantity) isLog() bool {
	nm := q.nonMult()
	if len(nm) != 1 || q.units.Len() != 1 {
		return false
	}
	def := q.reg.unitByName[nm[0]]
	return def != nil && def.isLogarithmic()
}

// ToUnits converts to a parsed unit container.
func (q Quantity) ToUnits(du UnitsContainer) (Quantity, error) {
	v, err := q.reg.convert(q.mag, q.units, du)
	if err != nil {
		return Quantity{}, err
	}
	return Quantity{reg: q.reg, mag: v, units: du}, nil
}

func (q Quantity) Format(spec string) string {
	compact := strings.Contains(spec, "~")
	us := q.reg.formatUnits(q.units, compact)
	if us == "" || us == "dimensionless" {
		return fmt.Sprintf("%g", q.mag)
	}
	return fmt.Sprintf("%g %s", q.mag, us)
}

func (q Quantity) MulFloat(x float64) Quantity {
	return Quantity{reg: q.reg, mag: q.mag * x, units: q.units}
}

func (q Quantity) addSub(o Quantity, sub bool) (Quantity, error) {
	if q.reg != o.reg {
		return Quantity{}, fmt.Errorf("quantities from different registries")
	}
	// Logarithmic units: convert to linear, multiply/divide, convert back.
	if q.isLog() && o.isLog() {
		qb, err := q.ToRootUnits()
		if err != nil {
			return Quantity{}, err
		}
		ob, err := o.ToRootUnits()
		if err != nil {
			return Quantity{}, err
		}
		var res Quantity
		if sub {
			res, err = qb.Div(ob)
		} else {
			res, err = qb.Mul(ob)
		}
		if err != nil {
			return Quantity{}, err
		}
		if qb.Dimensionless() && ob.Dimensionless() {
			return res.ToUnits(q.units)
		}
		if qb.Dimensionless() {
			return res.ToUnits(o.units)
		}
		if ob.Dimensionless() {
			return res.ToUnits(q.units)
		}
		return res, nil
	}
	da, err := q.Dimensionality()
	if err != nil {
		return Quantity{}, err
	}
	db, err := o.Dimensionality()
	if err != nil {
		return Quantity{}, err
	}
	if !da.Equal(db) {
		return Quantity{}, &DimensionalityError{Units1: q.units.String(), Units2: o.units.String(), Dim1: da.String(), Dim2: db.String()}
	}
	selfNM := q.nonMult()
	otherNM := o.nonMult()
	op := func(a, b float64) float64 {
		if sub {
			return a - b
		}
		return a + b
	}

	if len(selfNM) == 0 && len(otherNM) == 0 {
		ov, err := q.reg.convert(o.mag, o.units, q.units)
		if err != nil {
			return Quantity{}, err
		}
		return Quantity{reg: q.reg, mag: op(q.mag, ov), units: q.units}, nil
	}

	// degC - degC → delta_degC
	if sub && len(selfNM) == 1 && q.units.Get(selfNM[0]) == 1 && !q.hasCompatibleDelta(selfNM[0]) &&
		len(otherNM) == 1 && otherNM[0] == selfNM[0] {
		ov, err := q.reg.convert(o.mag, o.units, q.units)
		if err != nil {
			return Quantity{}, err
		}
		return Quantity{reg: q.reg, mag: op(q.mag, ov), units: q.units.Rename(selfNM[0], "delta_"+selfNM[0])}, nil
	}

	// degC + delta_degC
	if len(selfNM) == 1 && q.units.Get(selfNM[0]) == 1 && o.hasCompatibleDelta(selfNM[0]) {
		du := q.units.Rename(selfNM[0], "delta_"+selfNM[0])
		ov, err := q.reg.convert(o.mag, o.units, du)
		if err != nil {
			return Quantity{}, err
		}
		return Quantity{reg: q.reg, mag: op(q.mag, ov), units: q.units}, nil
	}
	if len(otherNM) == 1 && o.units.Get(otherNM[0]) == 1 && q.hasCompatibleDelta(otherNM[0]) {
		du := o.units.Rename(otherNM[0], "delta_"+otherNM[0])
		sv, err := q.reg.convert(q.mag, q.units, du)
		if err != nil {
			return Quantity{}, err
		}
		return Quantity{reg: q.reg, mag: op(sv, o.mag), units: o.units}, nil
	}

	return Quantity{}, &OffsetUnitCalculusError{Units: []string{q.units.String(), o.units.String()}}
}

func (q Quantity) nonMult() []string {
	var out []string
	for _, n := range q.units.Names() {
		if def := q.reg.unitByName[n]; def != nil && !def.isMultiplicative() {
			out = append(out, n)
		}
	}
	return out
}

func (q Quantity) deltas() []string {
	var out []string
	for _, n := range q.units.Names() {
		if len(n) > 6 && n[:6] == "delta_" {
			out = append(out, n)
		}
	}
	return out
}

func (q Quantity) hasCompatibleDelta(unit string) bool {
	if q.units.Has("delta_" + unit) {
		return true
	}
	def := q.reg.unitByName[unit]
	if def == nil {
		return false
	}
	for _, d := range q.deltas() {
		dd := q.reg.unitByName[d]
		if dd != nil && dd.reference.Equal(def.reference) {
			return true
		}
	}
	return false
}

func (q Quantity) okMulDiv() bool {
	nm := q.nonMult()
	if len(nm) > 1 {
		return false
	}
	if len(nm) == 1 {
		if q.units.Len() > 1 {
			return false
		}
		if q.units.Get(nm[0]) != 1 {
			return false
		}
		// Offset/log units cannot be multiplied in place; callers may convert to root first.
		return false
	}
	return true
}

func (q Quantity) Mul(o Quantity) (Quantity, error) {
	if q.reg != o.reg {
		return Quantity{}, fmt.Errorf("quantities from different registries")
	}
	if !q.okMulDiv() || !o.okMulDiv() {
		if q.reg.autoOffsetToBase {
			var err error
			if !q.okMulDiv() {
				q, err = q.ToRootUnits()
				if err != nil {
					return Quantity{}, err
				}
			}
			if !o.okMulDiv() {
				o, err = o.ToRootUnits()
				if err != nil {
					return Quantity{}, err
				}
			}
		} else {
			return Quantity{}, &OffsetUnitCalculusError{Units: []string{q.units.String(), o.units.String()}}
		}
	}
	return Quantity{reg: q.reg, mag: q.mag * o.mag, units: q.units.Mul(o.units)}, nil
}

func (q Quantity) Div(o Quantity) (Quantity, error) {
	if q.reg != o.reg {
		return Quantity{}, fmt.Errorf("quantities from different registries")
	}
	if !q.okMulDiv() || !o.okMulDiv() {
		if q.reg.autoOffsetToBase {
			var err error
			if !q.okMulDiv() {
				q, err = q.ToRootUnits()
				if err != nil {
					return Quantity{}, err
				}
			}
			if !o.okMulDiv() {
				o, err = o.ToRootUnits()
				if err != nil {
					return Quantity{}, err
				}
			}
		} else {
			return Quantity{}, &OffsetUnitCalculusError{Units: []string{q.units.String(), o.units.String()}}
		}
	}
	return Quantity{reg: q.reg, mag: q.mag / o.mag, units: q.units.Div(o.units)}, nil
}

func (q Quantity) Pow(p float64) (Quantity, error) {
	if !q.okMulDiv() && p != 1 {
		return Quantity{}, &OffsetUnitCalculusError{Units: []string{q.units.String()}}
	}
	return Quantity{reg: q.reg, mag: math.Pow(q.mag, p), units: q.units.Pow(p)}, nil
}

func (q Quantity) Neg() Quantity {
	return Quantity{reg: q.reg, mag: -q.mag, units: q.units}
}

func (q Quantity) AlmostEqual(o Quantity, rtol, atol float64) bool {
	if !q.Compatible(o) {
		return false
	}
	ov, err := q.reg.convert(o.mag, o.units, q.units)
	if err != nil {
		return false
	}
	return almostEqual(q.mag, ov, rtol, atol)
}
