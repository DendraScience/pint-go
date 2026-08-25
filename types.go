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

type transform struct {
	src, dst UnitsContainer
	equation string
}

type activeContext struct {
	ctx    *Context
	values map[string]float64
}
