package pint

import "math"

// Converter transforms a magnitude to and from a unit's reference.
type Converter interface {
	ToReference(v float64) float64
	FromReference(v float64) float64
	Scale() float64
	IsMultiplicative() bool
	IsLogarithmic() bool
}

// ScaleConverter is a linear transformation without offset: y = scale * x.
type ScaleConverter struct{ scale float64 }

func NewScaleConverter(scale float64) ScaleConverter { return ScaleConverter{scale: scale} }

func (c ScaleConverter) ToReference(v float64) float64   { return v * c.scale }
func (c ScaleConverter) FromReference(v float64) float64 { return v / c.scale }
func (c ScaleConverter) Scale() float64                  { return c.scale }
func (c ScaleConverter) IsMultiplicative() bool          { return true }
func (c ScaleConverter) IsLogarithmic() bool             { return false }

// OffsetConverter is an affine transformation: y = scale * x + offset.
type OffsetConverter struct {
	scale  float64
	offset float64
}

func NewOffsetConverter(scale, offset float64) OffsetConverter {
	return OffsetConverter{scale: scale, offset: offset}
}

func (c OffsetConverter) ToReference(v float64) float64   { return v*c.scale + c.offset }
func (c OffsetConverter) FromReference(v float64) float64 { return (v - c.offset) / c.scale }
func (c OffsetConverter) Scale() float64                  { return c.scale }
func (c OffsetConverter) Offset() float64                 { return c.offset }
func (c OffsetConverter) IsMultiplicative() bool          { return c.offset == 0 }
func (c OffsetConverter) IsLogarithmic() bool             { return false }

// LogarithmicConverter converts between linear and log units:
//
//	Q_log = logfactor * log(Q_lin / scale) / log(logbase)
type LogarithmicConverter struct {
	scale     float64
	logbase   float64
	logfactor float64
}

func NewLogarithmicConverter(scale, logbase, logfactor float64) LogarithmicConverter {
	return LogarithmicConverter{scale: scale, logbase: logbase, logfactor: logfactor}
}

func (c LogarithmicConverter) ToReference(v float64) float64 {
	return c.scale * math.Exp(math.Log(c.logbase)*(v/c.logfactor))
}

func (c LogarithmicConverter) FromReference(v float64) float64 {
	return c.logfactor * math.Log(v/c.scale) / math.Log(c.logbase)
}

func (c LogarithmicConverter) Scale() float64         { return c.scale }
func (c LogarithmicConverter) Logbase() float64       { return c.logbase }
func (c LogarithmicConverter) Logfactor() float64     { return c.logfactor }
func (c LogarithmicConverter) IsMultiplicative() bool { return false }
func (c LogarithmicConverter) IsLogarithmic() bool    { return true }

func converterFromModifiers(scale float64, mods map[string]float64) Converter {
	if off, ok := mods["offset"]; ok && off != 0 {
		return NewOffsetConverter(scale, off)
	}
	if lb, ok := mods["logbase"]; ok {
		lf := 1.0
		if v, ok := mods["logfactor"]; ok {
			lf = v
		}
		return NewLogarithmicConverter(scale, lb, lf)
	}
	return NewScaleConverter(scale)
}
