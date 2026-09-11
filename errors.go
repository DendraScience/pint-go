package pint

import (
	"fmt"
	"strings"
)

const (
	offsetErrorDocs = "https://pint.readthedocs.io/en/stable/user/nonmult.html"
	logErrorDocs    = "https://pint.readthedocs.io/en/stable/user/log_units.html"
)

// Error is the common interface for pint errors.
type Error interface {
	error
	PintError()
}

type pintError struct{ msg string }

func (e pintError) Error() string { return e.msg }
func (e pintError) PintError()    {}

// DefinitionSyntaxError is raised when a textual definition has a syntax error.
type DefinitionSyntaxError struct{ Msg string }

func (e *DefinitionSyntaxError) Error() string { return e.Msg }
func (e *DefinitionSyntaxError) PintError()    {}

// DefinitionError is raised when a definition is not properly constructed.
type DefinitionError struct {
	Name           string
	DefinitionType string
	Msg            string
}

func (e *DefinitionError) Error() string {
	return fmt.Sprintf("Cannot define '%s' (%s): %s", e.Name, e.DefinitionType, e.Msg)
}
func (e *DefinitionError) PintError() {}

// RedefinitionError is raised when a unit or prefix is redefined.
type RedefinitionError struct {
	Name           string
	DefinitionType string
}

func (e *RedefinitionError) Error() string {
	return fmt.Sprintf("Cannot redefine '%s' (%s)", e.Name, e.DefinitionType)
}
func (e *RedefinitionError) PintError() {}

// UndefinedUnitError is raised when units are not defined in the registry.
type UndefinedUnitError struct{ Names []string }

func (e *UndefinedUnitError) Error() string {
	if len(e.Names) == 1 {
		return fmt.Sprintf("'%s' is not defined in the unit registry", e.Names[0])
	}
	quoted := make([]string, len(e.Names))
	for i, n := range e.Names {
		quoted[i] = "'" + n + "'"
	}
	return fmt.Sprintf("(%s) are not defined in the unit registry", strings.Join(quoted, ", "))
}
func (e *UndefinedUnitError) PintError() {}

// NonFiniteConversionError is raised when a bound context hop would convert to
// Inf or NaN (for example chemistry with mw=0). Fail closed; do not return Inf.
type NonFiniteConversionError struct {
	From, To string
}

func (e *NonFiniteConversionError) Error() string {
	return fmt.Sprintf("conversion %s -> %s is not finite for the bound context parameters", e.From, e.To)
}
func (e *NonFiniteConversionError) PintError() {}

// DimensionalityError is raised when converting between incompatible units.
type DimensionalityError struct {
	Units1, Units2 string
	Dim1, Dim2     string
	Extra          string
}

func (e *DimensionalityError) Error() string {
	d1, d2 := "", ""
	if e.Dim1 != "" || e.Dim2 != "" {
		d1 = " (" + e.Dim1 + ")"
		d2 = " (" + e.Dim2 + ")"
	}
	return fmt.Sprintf("Cannot convert from '%s'%s to '%s'%s%s", e.Units1, d1, e.Units2, d2, e.Extra)
}
func (e *DimensionalityError) PintError() {}

// OffsetUnitCalculusError is raised on ambiguous operations with offset units.
type OffsetUnitCalculusError struct{ Units []string }

func (e *OffsetUnitCalculusError) Error() string {
	return fmt.Sprintf("Ambiguous operation with offset unit (%s). See %s for guidance.",
		strings.Join(e.Units, ", "), offsetErrorDocs)
}
func (e *OffsetUnitCalculusError) PintError() {}

// LogarithmicUnitCalculusError is raised on inappropriate operations with logarithmic units.
type LogarithmicUnitCalculusError struct{ Units []string }

func (e *LogarithmicUnitCalculusError) Error() string {
	return fmt.Sprintf("Ambiguous operation with logarithmic unit (%s). See %s for guidance.",
		strings.Join(e.Units, ", "), logErrorDocs)
}
func (e *LogarithmicUnitCalculusError) PintError() {}
