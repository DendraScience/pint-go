package pint

// Group is a named collection of units, optionally including other groups.
type Group struct {
	Name    string
	members map[string]struct{}
	using   []string
}

// Members returns unit names in this group (not recursively expanded).
func (g *Group) Members() []string {
	out := make([]string, 0, len(g.members))
	for n := range g.members {
		out = append(out, n)
	}
	return out
}

// System remaps base units (e.g. SI, cgs, imperial).
type System struct {
	Name  string
	Using []string
	Rules []systemRule
}

// Context allows conversion between incompatible dimensions.
type Context struct {
	Name       string
	Aliases    []string
	Defaults   map[string]float64
	transforms []transform
	redefs     []parsedDef
}

// ContextInfo is a catalog row for [Registry.Contexts].
// Params are declared @context(...) keys, sorted, not header default values.
type ContextInfo struct {
	Name   string
	Params []string
}

// Param is one context parameter. Same contract as Python Pint: pass a
// quantity (magnitude + unit). Empty Unit is dimensionless (spectroscopy n).
// Chemistry mw is "g/mol", not a bare 18.
type Param struct {
	Magnitude float64 `json:"magnitude"`
	Unit      string  `json:"unit,omitempty"`
}

// ContextUse is one enabled conversion context and its parameter overlay.
// Params overlay [Context.Defaults] (header defaults are dimensionless).
// Missing keys keep those defaults. Unknown keys are an error at bind.
type ContextUse struct {
	Name   string
	Params map[string]Param
}

// Scope is an ordered list of contexts to enable for one Converter / Compatible
// call. The caller builds it; it is not stored on the Registry.
type Scope []ContextUse

type transform struct {
	src, dst UnitsContainer
	equation string
}

type activeContext struct {
	ctx    *Context
	values map[string]Quantity
}
