package pint

import (
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/dendrascience/pint-go/defs"
)

// Registry holds unit definitions and performs conversions.
// After NewRegistry (or Load), it is safe for concurrent reads.
// Define is not concurrent with Convert.
type Registry struct {
	mu sync.RWMutex

	caseSensitive    bool
	defaultAsDelta   bool
	autoOffsetToBase bool
	onRedefinition   string // "raise", "warn", "ignore"

	defaultsGroup  string
	defaultsSystem string

	units      map[string]*unitDef // name/symbol/alias → def
	unitByName map[string]*unitDef // canonical name → def
	prefixes   map[string]*prefixDef
	prefixList []prefixDef
	dimensions map[string]*dimDef

	groups   map[string]*Group
	systems  map[string]*System
	contexts map[string]*Context

	active []activeContext

	parseCache map[parseKey]UnitsContainer
	rootCache  map[string]rootResult
	dimCache   map[string]UnitsContainer
	convCache  map[string]float64

	sourceFiles []string
}

type parseKey struct {
	s        string
	asDelta  bool
	caseSens bool
}

type rootResult struct {
	factor float64
	units  UnitsContainer
	ok     bool // false if non-multiplicative blocked
}

type unitDef struct {
	name      string
	symbol    string
	aliases   []string
	converter Converter
	reference UnitsContainer
	isBase    bool
}

func (u *unitDef) isMultiplicative() bool { return u.converter.IsMultiplicative() }
func (u *unitDef) isLogarithmic() bool    { return u.converter.IsLogarithmic() }

type prefixDef struct {
	name    string
	symbol  string
	aliases []string
	value   float64
}

type dimDef struct {
	name      string
	isBase    bool
	reference UnitsContainer
}

// Option configures a Registry.
type Option func(*Registry)

func WithCaseSensitive(v bool) Option    { return func(r *Registry) { r.caseSensitive = v } }
func WithDefaultAsDelta(v bool) Option   { return func(r *Registry) { r.defaultAsDelta = v } }
func WithAutoOffsetToBase(v bool) Option { return func(r *Registry) { r.autoOffsetToBase = v } }
func WithOnRedefinition(s string) Option { return func(r *Registry) { r.onRedefinition = s } }

// NewRegistry builds a registry loaded with Pint's default English definitions.
func NewRegistry(opts ...Option) (*Registry, error) {
	r := newEmptyRegistry(opts...)
	if err := r.loadEmbedded(defs.DefaultFile); err != nil {
		return nil, err
	}
	r.buildCaches()
	return r, nil
}

// NewEmptyRegistry creates a registry with no units defined.
func NewEmptyRegistry(opts ...Option) *Registry {
	return newEmptyRegistry(opts...)
}

func newEmptyRegistry(opts ...Option) *Registry {
	r := &Registry{
		caseSensitive:  true,
		defaultAsDelta: true,
		onRedefinition: "raise",
		units:          map[string]*unitDef{},
		unitByName:     map[string]*unitDef{},
		prefixes:       map[string]*prefixDef{"": {name: "", value: 1}},
		prefixList:     []prefixDef{{name: "", value: 1}},
		dimensions:     map[string]*dimDef{},
		groups:         map[string]*Group{"root": {Name: "root", members: map[string]struct{}{}}},
		systems:        map[string]*System{},
		contexts:       map[string]*Context{},
		parseCache:     map[parseKey]UnitsContainer{},
		rootCache:      map[string]rootResult{},
		dimCache:       map[string]UnitsContainer{},
		convCache:      map[string]float64{},
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func (r *Registry) loadEmbedded(name string) error {
	b, err := defs.Files.ReadFile(name)
	if err != nil {
		return err
	}
	return r.loadSource(string(b), name)
}

func (r *Registry) loadSource(src, origin string) error {
	defs, err := parseDefinitionFile(src, origin)
	if err != nil {
		return err
	}
	for _, d := range defs {
		if err := r.applyDef(d); err != nil {
			return fmt.Errorf("%s: %w", origin, err)
		}
	}
	r.sourceFiles = append(r.sourceFiles, origin)
	return nil
}

func (r *Registry) applyDef(d parsedDef) error {
	switch d.kind {
	case defImport:
		return r.loadEmbedded(d.filename)
	case defDefaults:
		if d.value != "" {
			r.defaultsGroup = d.value
		}
		if d.symbol != "" {
			r.defaultsSystem = d.symbol
		}
		return nil
	case defPrefix:
		return r.addPrefix(d)
	case defUnit:
		return r.addUnit(d, "")
	case defDimension:
		return r.addDimension(d.name, true, UnitsContainer{})
	case defDerivedDimension:
		su, err := parseScaledUnits(d.value)
		if err != nil {
			return err
		}
		if su.Scale != 1 {
			return &DefinitionSyntaxError{Msg: "derived dimension cannot have a scale"}
		}
		return r.addDimension(d.name, false, su.Units)
	case defAlias:
		return r.addAliases(d.name, d.aliases)
	case defGroup:
		return r.addGroup(d)
	case defContext:
		return r.addContext(d)
	case defSystem:
		return r.addSystem(d)
	}
	return nil
}

func (r *Registry) addPrefix(d parsedDef) error {
	p := &prefixDef{name: d.name, symbol: d.symbol, aliases: d.aliases, value: d.mods["scale"]}
	return r.registerPrefix(p)
}

func (r *Registry) registerPrefix(p *prefixDef) error {
	keys := []string{p.name}
	if p.symbol != "" && p.symbol != p.name {
		keys = append(keys, p.symbol)
	}
	keys = append(keys, p.aliases...)
	for _, k := range keys {
		if k == "" {
			continue
		}
		if _, ok := r.prefixes[k]; ok {
			if err := r.redef("prefix", k); err != nil {
				return err
			}
		}
		r.prefixes[k] = p
	}
	r.prefixList = append(r.prefixList, *p)
	return nil
}

func (r *Registry) addDimension(name string, base bool, ref UnitsContainer) error {
	if !isDim(name) {
		return &DefinitionError{Name: name, DefinitionType: "DimensionDefinition", Msg: "is not a valid dimension name"}
	}
	if _, ok := r.dimensions[name]; ok {
		if err := r.redef("dimension", name); err != nil {
			return err
		}
	}
	r.dimensions[name] = &dimDef{name: name, isBase: base, reference: ref}
	return nil
}

func (r *Registry) addUnit(d parsedDef, group string) error {
	su, err := parseScaledUnits(d.value)
	if err != nil {
		return fmt.Errorf("unit %s: %w", d.name, err)
	}
	if !isPythonIdent(d.name) {
		return &DefinitionError{Name: d.name, DefinitionType: "UnitDefinition", Msg: "is not a valid unit name (must follow Python identifier rules)"}
	}
	conv := converterFromModifiers(su.Scale, d.mods)
	isBase := false
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
			return &DefinitionError{Name: d.name, DefinitionType: "UnitDefinition", Msg: "Cannot mix dimensions and units in the same definition."}
		}
		isBase = allDim
		if isBase && su.Scale != 1 {
			return &DefinitionError{Name: d.name, DefinitionType: "UnitDefinition", Msg: "Base unit definitions cannot have a scale different to 1."}
		}
	}
	u := &unitDef{
		name:      d.name,
		symbol:    d.symbol,
		aliases:   d.aliases,
		converter: conv,
		reference: su.Units,
		isBase:    isBase,
	}
	if err := r.registerUnit(u); err != nil {
		return err
	}
	if isBase {
		for _, dim := range su.Units.Names() {
			if _, ok := r.dimensions[dim]; !ok {
				if err := r.addDimension(dim, true, UnitsContainer{}); err != nil {
					return err
				}
			}
		}
	}
	r.groups["root"].members[u.name] = struct{}{}
	if group != "" {
		g := r.getOrCreateGroup(group)
		g.members[u.name] = struct{}{}
	}
	r.maybeAddDelta(u)
	return nil
}

func (r *Registry) maybeAddDelta(u *unitDef) {
	if u.isMultiplicative() || u.isLogarithmic() {
		return
	}
	oc, ok := u.converter.(OffsetConverter)
	if !ok {
		return
	}
	deltaName := "delta_" + u.name
	var dsymbol string
	if u.symbol != "" {
		dsymbol = "Δ" + u.symbol
	}
	var aliases []string
	for _, a := range u.aliases {
		aliases = append(aliases, "Δ"+a, "delta_"+a)
	}
	du := &unitDef{
		name:      deltaName,
		symbol:    dsymbol,
		aliases:   aliases,
		converter: NewScaleConverter(oc.Scale()),
		reference: u.reference,
		isBase:    false,
	}
	_ = r.registerUnit(du)
}

func (r *Registry) registerUnit(u *unitDef) error {
	keys := []string{u.name}
	if u.symbol != "" && u.symbol != u.name && u.symbol != "_" {
		keys = append(keys, u.symbol)
	}
	keys = append(keys, u.aliases...)
	for _, k := range keys {
		if k == "" || k == "_" {
			continue
		}
		if _, ok := r.units[k]; ok {
			if err := r.redef("unit", k); err != nil {
				return err
			}
		}
		r.units[k] = u
	}
	r.unitByName[u.name] = u
	return nil
}

func (r *Registry) addAliases(name string, aliases []string) error {
	u, err := r.lookupUnit(name)
	if err != nil {
		return err
	}
	u.aliases = append(u.aliases, aliases...)
	for _, a := range aliases {
		if _, ok := r.units[a]; ok {
			if err := r.redef("unit", a); err != nil {
				return err
			}
		}
		r.units[a] = u
	}
	return nil
}

func (r *Registry) redef(kind, name string) error {
	switch r.onRedefinition {
	case "ignore", "warn":
		return nil
	default:
		return &RedefinitionError{Name: name, DefinitionType: kind}
	}
}

func (r *Registry) addGroup(d parsedDef) error {
	g := r.getOrCreateGroup(d.name)
	g.using = append(g.using, d.using...)
	for _, m := range d.members {
		if err := r.addUnit(m, d.name); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) getOrCreateGroup(name string) *Group {
	if g, ok := r.groups[name]; ok {
		return g
	}
	g := &Group{Name: name, members: map[string]struct{}{}}
	r.groups[name] = g
	r.groups["root"].using = append(r.groups["root"].using, name)
	return g
}

func (r *Registry) addSystem(d parsedDef) error {
	sys := &System{Name: d.name, Using: d.using, Rules: d.rules}
	r.systems[d.name] = sys
	return nil
}

func (r *Registry) addContext(d parsedDef) error {
	ctx := &Context{Name: d.name, Aliases: d.aliases, Defaults: d.defaults}
	for _, rel := range d.rels {
		srcU, err := r.parseDimContainer(rel.src)
		if err != nil {
			return err
		}
		dstU, err := r.parseDimContainer(rel.dst)
		if err != nil {
			return err
		}
		src, err := r.dimensionality(srcU)
		if err != nil {
			return err
		}
		dst, err := r.dimensionality(dstU)
		if err != nil {
			return err
		}
		tr := transform{src: src, dst: dst, equation: rel.equation}
		ctx.transforms = append(ctx.transforms, tr)
		if rel.bidirect {
			ctx.transforms = append(ctx.transforms, transform{src: dst, dst: src, equation: invertEq(rel.equation)})
		}
	}
	for _, rd := range d.redefs {
		ctx.redefs = append(ctx.redefs, rd)
	}
	r.contexts[d.name] = ctx
	for _, a := range d.aliases {
		r.contexts[a] = ctx
	}
	return nil
}

func invertEq(eq string) string {
	// For simple `const / value` bidirectional relations Pint reuses the same equation
	// (value is the quantity being converted). Keep the same string.
	return eq
}

func (r *Registry) parseDimContainer(s string) (UnitsContainer, error) {
	su, err := parseScaledUnits(s)
	if err != nil {
		return UnitsContainer{}, err
	}
	return su.Units, nil
}

func (r *Registry) buildCaches() {
	r.parseCache = map[parseKey]UnitsContainer{}
	r.rootCache = map[string]rootResult{}
	r.dimCache = map[string]UnitsContainer{}
	r.convCache = map[string]float64{}
}

func (r *Registry) lookupUnit(name string) (*unitDef, error) {
	if u, ok := r.units[name]; ok {
		return u, nil
	}
	if !r.caseSensitive {
		lower := strings.ToLower(name)
		for k, u := range r.units {
			if strings.ToLower(k) == lower {
				return u, nil
			}
		}
	}
	return nil, &UndefinedUnitError{Names: []string{name}}
}

func (r *Registry) getName(name string) (string, error) {
	if name == "dimensionless" {
		return "", nil
	}
	if u, ok := r.units[name]; ok {
		return u.name, nil
	}
	cands := r.parseUnitName(name, r.caseSensitive)
	if len(cands) == 0 {
		return "", &UndefinedUnitError{Names: []string{name}}
	}
	prefix, uname := cands[0].prefix, cands[0].unit
	if prefix != "" {
		u := r.unitByName[uname]
		if u != nil && !u.isMultiplicative() {
			return "", &OffsetUnitCalculusError{Units: []string{name}}
		}
		full := prefix + uname
		if _, ok := r.units[full]; !ok {
			pd := r.prefixes[prefix]
			nu := &unitDef{
				name:      full,
				converter: NewScaleConverter(pd.value),
				reference: unitPair(uname, 1),
			}
			r.units[full] = nu
			r.unitByName[full] = nu
		}
		return full, nil
	}
	return uname, nil
}

type unitTriplet struct {
	prefix, unit, suffix string
}

func (r *Registry) parseUnitName(name string, caseSensitive bool) []unitTriplet {
	type cand struct {
		prefix, unit, suffix string
	}
	var out []unitTriplet
	seen := map[string]struct{}{}
	for _, p := range r.prefixList {
		pres := []string{p.name, p.symbol}
		pres = append(pres, p.aliases...)
		for _, pre := range pres {
			if pre == "" && p.name != "" {
				continue
			}
			if pre != "" && !hasPrefix(name, pre, caseSensitive) {
				continue
			}
			rest := name
			if pre != "" {
				rest = name[len(pre):]
			}
			for _, suf := range []string{"", "s"} {
				core := rest
				if suf != "" {
					if !strings.HasSuffix(rest, suf) {
						continue
					}
					core = rest[:len(rest)-len(suf)]
					if len(core) == 1 {
						continue
					}
				}
				uname, ok := r.matchUnitCore(core, caseSensitive)
				if !ok {
					continue
				}
				key := p.name + "|" + uname + "|" + suf
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, unitTriplet{prefix: p.name, unit: uname, suffix: suf})
			}
		}
	}
	// Prefer prefixed forms when equivalent (kilo+gram vs kilogram).
	if len(out) > 1 {
		pref := make([]unitTriplet, 0, len(out))
		var bare []unitTriplet
		for _, c := range out {
			if c.prefix != "" {
				pref = append(pref, c)
			} else {
				bare = append(bare, c)
			}
		}
		if len(pref) > 0 {
			return pref
		}
		return bare
	}
	_ = unicode.ReplacementChar
	return out
}

func hasPrefix(s, pre string, caseSensitive bool) bool {
	if len(pre) > len(s) {
		return false
	}
	if caseSensitive {
		return strings.HasPrefix(s, pre)
	}
	return strings.EqualFold(s[:len(pre)], pre)
}

func (r *Registry) matchUnitCore(core string, caseSensitive bool) (string, bool) {
	if u, ok := r.units[core]; ok {
		return u.name, true
	}
	if !caseSensitive {
		lower := strings.ToLower(core)
		for k, u := range r.units {
			if strings.ToLower(k) == lower {
				return u.name, true
			}
		}
	}
	return "", false
}

func isDim(name string) bool {
	return len(name) >= 2 && name[0] == '[' && name[len(name)-1] == ']'
}
