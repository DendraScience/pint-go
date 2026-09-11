package pint

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Preprocess applies Pint's string preprocessor (unicode exponents, "per", commas, …).
func Preprocess(s string) string { return stringPreprocessor(s) }

// Load parses additional definition text into the registry.
func (r *Registry) Load(src string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.loadSource(src, "<load>"); err != nil {
		return err
	}
	r.buildCaches()
	return nil
}

// NewRegistryFrom builds a registry from a definition string (no default files).
func NewRegistryFrom(src string, opts ...Option) (*Registry, error) {
	r := NewEmptyRegistry(opts...)
	if err := r.Load(src); err != nil {
		return nil, err
	}
	return r, nil
}

// HasUnit reports whether name can be parsed as a unit.
func (r *Registry) HasUnit(name string) bool {
	_, err := r.ParseUnits(name)
	return err == nil
}

// Definition is a parsed prefix, unit, dimension, or alias line.
type Definition struct {
	Kind      string // "prefix", "unit", "dimension", "alias"
	Name      string
	Symbol    string
	Aliases   []string
	Converter Converter
	Reference UnitsContainer
	IsBase    bool
}

// ParseDefinition parses a single definition line without a registry.
func ParseDefinition(line string) (*Definition, error) {
	d, err := parseDefinitionLine(line)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, &DefinitionSyntaxError{Msg: "empty definition"}
	}
	out := &Definition{Name: d.name, Symbol: d.symbol, Aliases: d.aliases}
	switch d.kind {
	case defPrefix:
		out.Kind = "prefix"
		out.Converter = NewScaleConverter(d.mods["scale"])
	case defUnit:
		out.Kind = "unit"
		su, err := parseScaledUnits(d.value)
		if err != nil {
			return nil, err
		}
		out.Converter = converterFromModifiers(su.Scale, d.mods)
		out.Reference = su.Units
		if !su.Units.Empty() {
			allDim, anyDim := true, false
			for _, n := range su.Units.Names() {
				if isDim(n) {
					anyDim = true
				} else {
					allDim = false
				}
			}
			if anyDim && !allDim {
				return nil, &DefinitionSyntaxError{Msg: "Cannot mix dimensions and units in the same definition."}
			}
			out.IsBase = allDim
		}
	case defDimension, defDerivedDimension:
		out.Kind = "dimension"
		if d.value != "" {
			su, err := parseScaledUnits(d.value)
			if err != nil {
				return nil, err
			}
			allDim, anyDim := true, false
			for _, n := range su.Units.Names() {
				if isDim(n) {
					anyDim = true
				} else {
					allDim = false
				}
			}
			if anyDim && !allDim {
				return nil, &DefinitionSyntaxError{Msg: "Cannot mix dimensions and units in the same definition."}
			}
			out.Reference = su.Units
		} else {
			out.IsBase = true
		}
	case defAlias:
		out.Kind = "alias"
	default:
		return nil, &DefinitionSyntaxError{Msg: "unsupported definition kind"}
	}
	return out, nil
}

// InferBaseUnit strips prefixes from each name (millimeter*nanometer → meter**2).
func (r *Registry) InferBaseUnit(u UnitsContainer) (UnitsContainer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	acc := map[string]float64{}
	for _, n := range u.Names() {
		exp := u.Get(n)
		cands := r.parseUnitName(n, r.caseSensitive)
		base := n
		for _, c := range cands {
			if c.prefix != "" {
				base = c.unit
				break
			}
			base = c.unit
		}
		acc[base] += exp
	}
	return NewUnitsContainer(acc), nil
}

var siPrefixOrder = []struct {
	name  string
	value float64
}{
	{"yotta", 1e24}, {"zetta", 1e21}, {"exa", 1e18}, {"peta", 1e15},
	{"tera", 1e12}, {"giga", 1e9}, {"mega", 1e6}, {"kilo", 1e3},
	{"hecto", 1e2}, {"deca", 1e1},
	{"", 1},
	{"deci", 1e-1}, {"centi", 1e-2}, {"milli", 1e-3}, {"micro", 1e-6},
	{"nano", 1e-9}, {"pico", 1e-12}, {"femto", 1e-15}, {"atto", 1e-18},
	{"zepto", 1e-21}, {"yocto", 1e-24},
}

// ToCompact rewrites the quantity with SI prefixes so the magnitude is near 1–1000.
func (q Quantity) ToCompact() (Quantity, error) {
	inf, err := q.reg.InferBaseUnit(q.units)
	if err != nil {
		return Quantity{}, err
	}
	got, err := q.ToUnits(inf)
	if err != nil {
		return Quantity{}, err
	}
	if inf.Empty() || got.mag == 0 || math.IsInf(got.mag, 0) || math.IsNaN(got.mag) {
		return got, nil
	}
	var target string
	var exp float64
	for _, n := range inf.Names() {
		e := inf.Get(n)
		if e > 0 {
			target, exp = n, e
			break
		}
	}
	if target == "" || exp == 0 {
		return got, nil
	}
	bestScore := math.Inf(1)
	best := got
	for _, p := range siPrefixOrder {
		scale := math.Pow(p.value, exp)
		if scale == 0 || math.IsInf(scale, 0) {
			continue
		}
		newMag := got.mag / scale
		a := math.Abs(newMag)
		score := math.Abs(math.Log10(a) - 1.5)
		if a < 1 || a >= 1000 {
			score += 10
		}
		if score >= bestScore {
			continue
		}
		nu := inf
		if p.name != "" {
			full := p.name + target
			if _, err := q.reg.ParseUnits(full); err != nil {
				continue
			}
			nu = inf.Rename(target, full)
		}
		bestScore = score
		best = Quantity{reg: q.reg, mag: newMag, units: nu}
	}
	return best, nil
}

// Group returns a named group.
func (r *Registry) Group(name string) (*Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.groups[name]
	if !ok {
		return nil, fmt.Errorf("unknown group %q", name)
	}
	return g, nil
}

// NewGroup creates (or returns) a group and registers it under root.
func (r *Registry) NewGroup(name string) *Group {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.getOrCreateGroup(name)
}

// AddUnits adds unit names to the group.
func (g *Group) AddUnits(names ...string) {
	if g.members == nil {
		g.members = map[string]struct{}{}
	}
	for _, n := range names {
		g.members[n] = struct{}{}
	}
}

// AddGroups records that this group includes others. A cycle is an error.
func (g *Group) AddGroups(r *Registry, names ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range names {
		if n == g.Name || wouldCycle(r, g.Name, n) {
			return fmt.Errorf("cyclic group inclusion")
		}
		g.using = append(g.using, n)
		r.getOrCreateGroup(n)
	}
	return nil
}

func wouldCycle(r *Registry, from, to string) bool {
	seen := map[string]struct{}{}
	var walk func(string) bool
	walk = func(n string) bool {
		if n == from {
			return true
		}
		if _, ok := seen[n]; ok {
			return false
		}
		seen[n] = struct{}{}
		g := r.groups[n]
		if g == nil {
			return false
		}
		for _, u := range g.using {
			if walk(u) {
				return true
			}
		}
		return false
	}
	return walk(to)
}

// Expanded returns unit names in this group including used groups.
func (g *Group) Expanded(r *Registry) map[string]struct{} {
	out := map[string]struct{}{}
	var walk func(*Group)
	seen := map[string]struct{}{}
	walk = func(gr *Group) {
		if gr == nil {
			return
		}
		if _, ok := seen[gr.Name]; ok {
			return
		}
		seen[gr.Name] = struct{}{}
		for n := range gr.members {
			out[n] = struct{}{}
		}
		for _, u := range gr.using {
			walk(r.groups[u])
		}
	}
	walk(g)
	return out
}

// System returns a named unit system.
func (r *Registry) System(name string) (*System, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.systems[name]
	if !ok {
		return nil, fmt.Errorf("unknown system %q", name)
	}
	return s, nil
}

// ContextByName returns a named conversion context.
func (r *Registry) ContextByName(name string) (*Context, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.contexts[name]
	if !ok {
		return nil, fmt.Errorf("unknown context %q", name)
	}
	return c, nil
}

// Contexts lists each loaded context once by canonical Name, with declared
// parameter keys sorted. Aliases are omitted. Safe for concurrent convert.
func (r *Registry) Contexts() []ContextInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[string]struct{}{}
	out := make([]ContextInfo, 0)
	for _, ctx := range r.contexts {
		if ctx == nil {
			continue
		}
		if _, ok := seen[ctx.Name]; ok {
			continue
		}
		seen[ctx.Name] = struct{}{}
		params := make([]string, 0, len(ctx.Defaults))
		for k := range ctx.Defaults {
			params = append(params, k)
		}
		sort.Strings(params)
		out = append(out, ContextInfo{Name: ctx.Name, Params: params})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CompatibleUnits lists canonical unit names convertible from unit.
// Same dimension always; context hops use the bound scope (or r.active).
// If groupOrSystem is non-empty, the list is restricted to that group or system's members.
func (r *Registry) CompatibleUnits(unit, groupOrSystem string, scope ...Scope) ([]string, error) {
	u, err := r.ParseUnits(unit)
	if err != nil {
		return nil, err
	}
	dim, err := r.dimensionality(u)
	if err != nil {
		return nil, err
	}
	active, err := r.bindActive(scope...)
	if err != nil {
		return nil, err
	}
	r.mu.RLock()
	pool := map[string]struct{}{}
	if groupOrSystem == "" {
		for name := range r.unitByName {
			pool[name] = struct{}{}
		}
	} else if g, ok := r.groups[groupOrSystem]; ok {
		pool = g.Expanded(r)
	} else if sys, ok := r.systems[groupOrSystem]; ok {
		for _, ug := range sys.Using {
			if gg := r.groups[ug]; gg != nil {
				for n := range gg.Expanded(r) {
					pool[n] = struct{}{}
				}
			}
		}
		if g := r.groups["root"]; g != nil && len(sys.Using) == 0 {
			pool = g.Expanded(r)
		}
	} else {
		r.mu.RUnlock()
		return nil, fmt.Errorf("unknown group or system %q", groupOrSystem)
	}
	names := make([]string, 0, len(pool))
	for name := range pool {
		names = append(names, name)
	}
	r.mu.RUnlock()
	var out []string
	for _, name := range names {
		d, err := r.dimensionality(unitPair(name, 1))
		if err != nil {
			continue
		}
		if d.Equal(dim) || r.contextPath(dim, d, active) != nil {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// BaseUnits returns the conversion factor and root (or system) units for unit.
func (r *Registry) BaseUnits(unit, system string) (float64, UnitsContainer, error) {
	u, err := r.ParseUnits(unit)
	if err != nil {
		return 0, UnitsContainer{}, err
	}
	rr, err := r.rootUnits(u, false)
	if err != nil {
		return 0, UnitsContainer{}, err
	}
	if system == "" {
		return rr.factor, rr.units, nil
	}
	r.mu.RLock()
	sys, ok := r.systems[system]
	r.mu.RUnlock()
	if !ok {
		return 0, UnitsContainer{}, fmt.Errorf("unknown system %q", system)
	}
	// Remap each root unit onto a system base of the same dimension.
	factor := rr.factor
	acc := map[string]float64{}
	for _, n := range rr.units.Names() {
		exp := rr.units.Get(n)
		repl := r.systemReplacement(sys, n)
		if repl == n {
			acc[n] += exp
			continue
		}
		ru, err := r.ParseUnits(repl)
		if err != nil {
			acc[n] += exp
			continue
		}
		f, err := r.conversionFactor(unitPair(n, 1), ru)
		if err != nil {
			acc[n] += exp
			continue
		}
		factor *= math.Pow(f, exp)
		for _, rn := range ru.Names() {
			acc[rn] += ru.Get(rn) * exp
		}
	}
	return factor, NewUnitsContainer(acc), nil
}

func (r *Registry) systemReplacement(sys *System, root string) string {
	rd, err := r.dimensionality(unitPair(root, 1))
	if err != nil {
		return root
	}
	for _, rule := range sys.Rules {
		cand := rule.newUnit
		cu, err := r.ParseUnits(cand)
		if err != nil {
			continue
		}
		cd, err := r.dimensionality(cu)
		if err != nil {
			continue
		}
		if cd.Equal(rd) {
			return cand
		}
	}
	return root
}

// PiTheorem computes dimensionless groups (Buckingham π) from named unit expressions.
func (r *Registry) PiTheorem(quantities map[string]string) ([]map[string]float64, error) {
	names := make([]string, 0, len(quantities))
	dims := make([]UnitsContainer, 0, len(quantities))
	dimSet := map[string]struct{}{}
	for n, u := range quantities {
		_ = u
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		u := quantities[n]
		uc, err := r.ParseUnits(u)
		if err != nil {
			q, qerr := r.Parse(u)
			if qerr != nil {
				return nil, err
			}
			uc = q.units
		}
		d, err := r.dimensionality(uc)
		if err != nil {
			return nil, err
		}
		dims = append(dims, d)
		for _, k := range d.Names() {
			dimSet[k] = struct{}{}
		}
	}
	dimNames := make([]string, 0, len(dimSet))
	for k := range dimSet {
		dimNames = append(dimNames, k)
	}
	sort.Strings(dimNames)
	// Matrix: rows = dimensions, cols = quantities.
	m := len(dimNames)
	n := len(names)
	if n == 0 {
		return nil, nil
	}
	A := make([][]float64, m)
	for i := range A {
		A[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			A[i][j] = dims[j].Get(dimNames[i])
		}
	}
	null := nullspace(A)
	var out []map[string]float64
	for _, vec := range null {
		row := map[string]float64{}
		neg, pos := 0, 0
		for j, v := range vec {
			v = math.Round(v*1e9) / 1e9
			if v == 0 {
				continue
			}
			if v < 0 {
				neg++
			} else {
				pos++
			}
			row[names[j]] = v
		}
		if len(row) == 0 {
			continue
		}
		if neg > pos {
			for k, v := range row {
				row[k] = -v
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func nullspace(A [][]float64) [][]float64 {
	m := len(A)
	if m == 0 {
		return nil
	}
	n := len(A[0])
	// Augment with identity for column operations: work on A|I transposed style.
	// Compute RREF of A and read free variables.
	M := make([][]float64, m)
	for i := 0; i < m; i++ {
		M[i] = make([]float64, n)
		copy(M[i], A[i])
	}
	pivots := make([]int, 0, m)
	used := map[int]bool{}
	row := 0
	for col := 0; col < n && row < m; col++ {
		piv := -1
		for i := row; i < m; i++ {
			if math.Abs(M[i][col]) > 1e-12 {
				piv = i
				break
			}
		}
		if piv < 0 {
			continue
		}
		M[row], M[piv] = M[piv], M[row]
		div := M[row][col]
		for j := 0; j < n; j++ {
			M[row][j] /= div
		}
		for i := 0; i < m; i++ {
			if i == row {
				continue
			}
			f := M[i][col]
			for j := 0; j < n; j++ {
				M[i][j] -= f * M[row][j]
			}
		}
		pivots = append(pivots, col)
		used[col] = true
		row++
	}
	var free []int
	for j := 0; j < n; j++ {
		if !used[j] {
			free = append(free, j)
		}
	}
	var ns [][]float64
	for _, f := range free {
		v := make([]float64, n)
		v[f] = 1
		for i, p := range pivots {
			// M[i][p] == 1, M[i][f] is the coefficient
			v[p] = -M[i][f]
		}
		ns = append(ns, v)
	}
	return ns
}

func (r *Registry) formatUnits(u UnitsContainer, compact bool) string {
	if u.Empty() {
		return "dimensionless"
	}
	nameOf := func(n string) string {
		if !compact {
			return n
		}
		if def := r.unitByName[n]; def != nil && def.symbol != "" && def.symbol != "_" {
			return def.symbol
		}
		return n
	}
	var num, den []string
	for _, it := range u.items {
		label := nameOf(it.Name)
		if it.Exp > 0 {
			s := label
			if it.Exp != 1 {
				if compact {
					s += "^" + formatExp(it.Exp)
				} else {
					s += " ** " + formatExp(it.Exp)
				}
			}
			num = append(num, s)
		} else {
			e := -it.Exp
			s := label
			if e != 1 {
				if compact {
					s += "^" + formatExp(e)
				} else {
					s += " ** " + formatExp(e)
				}
			}
			den = append(den, s)
		}
	}
	join := " * "
	if compact {
		join = " "
	}
	ns := strings.Join(num, join)
	if len(den) == 0 {
		if ns == "" {
			return "dimensionless"
		}
		return ns
	}
	ds := strings.Join(den, " / ")
	if ns == "" {
		return "1 / " + ds
	}
	if len(den) == 1 {
		return ns + " / " + ds
	}
	return ns + " / " + ds
}
