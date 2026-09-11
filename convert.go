package pint

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ParseUnits parses a unit expression into a canonical UnitsContainer.
func (r *Registry) ParseUnits(s string) (UnitsContainer, error) {
	return r.parseUnits(s, r.defaultAsDelta)
}

func (r *Registry) parseUnits(s string, asDelta bool) (UnitsContainer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.parseUnitsLocked(s, asDelta, r.caseSensitive)
}

func (r *Registry) parseUnitsLocked(s string, asDelta, caseSens bool) (UnitsContainer, error) {
	key := parseKey{s: s, asDelta: asDelta, caseSens: caseSens}
	if u, ok := r.parseCache[key]; ok {
		return u, nil
	}
	su, err := parseScaledUnits(s)
	if err != nil {
		return UnitsContainer{}, err
	}
	if su.Scale != 1 {
		return UnitsContainer{}, fmt.Errorf("unit expression cannot have a scaling factor")
	}
	out := UnitsContainer{}
	many := su.Units.Len() > 1
	for _, name := range su.Units.Names() {
		exp := su.Units.Get(name)
		cname, err := r.getName(name)
		if err != nil {
			return UnitsContainer{}, err
		}
		if cname == "" {
			continue
		}
		if asDelta && (many || exp != 1) {
			if u := r.unitByName[cname]; u != nil && !u.isMultiplicative() {
				cname = "delta_" + cname
			}
		}
		out = out.Add(cname, exp)
	}
	if asDelta {
		r.parseCache[key] = out
	}
	return out, nil
}

// Parse parses a quantity expression such as "3.2 kPa".
func (r *Registry) Parse(s string) (Quantity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	su, err := parseScaledUnits(s)
	if err != nil {
		return Quantity{}, err
	}
	units := UnitsContainer{}
	for _, name := range su.Units.Names() {
		cname, err := r.getName(name)
		if err != nil {
			return Quantity{}, err
		}
		if cname == "" {
			continue
		}
		units = units.Add(cname, su.Units.Get(name))
	}
	return Quantity{reg: r, mag: su.Scale, units: units}, nil
}

// MustParse parses s or panics.
func (r *Registry) MustParse(s string) Quantity {
	q, err := r.Parse(s)
	if err != nil {
		panic(err)
	}
	return q
}

// Quantity builds a quantity from a magnitude and unit expression.
func (r *Registry) Quantity(mag float64, unit string) (Quantity, error) {
	u, err := r.ParseUnits(unit)
	if err != nil {
		return Quantity{}, err
	}
	return Quantity{reg: r, mag: mag, units: u}, nil
}

// Fingerprint is a stable cache key for this scope: context names in order,
// each with sorted k=magnitude\x1dunit params. Empty Scope is "".
// Equivalent quantities written differently (18 g/mol vs 0.018 kg/mol) do not
// collapse; the caller may canonicalize before building the Scope.
func (s Scope) Fingerprint() string {
	if len(s) == 0 {
		return ""
	}
	var b strings.Builder
	for i, use := range s {
		if i > 0 {
			b.WriteByte(0x1e)
		}
		b.WriteString(use.Name)
		if len(use.Params) == 0 {
			continue
		}
		keys := make([]string, 0, len(use.Params))
		for k := range use.Params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte(0x1f)
		for j, k := range keys {
			if j > 0 {
				b.WriteByte(',')
			}
			p := use.Params[k]
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(strconv.FormatFloat(p.Magnitude, 'g', 17, 64))
			b.WriteByte(0x1d)
			b.WriteString(p.Unit)
		}
	}
	return b.String()
}

func copyActive(src []activeContext) []activeContext {
	out := make([]activeContext, len(src))
	for i, ac := range src {
		vals := make(map[string]Quantity, len(ac.values))
		for k, v := range ac.values {
			vals[k] = v
		}
		out[i] = activeContext{ctx: ac.ctx, values: vals}
	}
	return out
}

// bindActive snapshots the contexts used for one Converter / Compatible call.
// Zero extra args copies r.active (CLI). One Scope, even empty, is that list.
func (r *Registry) bindActive(scope ...Scope) ([]activeContext, error) {
	if len(scope) > 1 {
		return nil, fmt.Errorf("Converter accepts at most one Scope")
	}
	if len(scope) == 0 {
		r.mu.RLock()
		defer r.mu.RUnlock()
		return copyActive(r.active), nil
	}
	return r.scopeToActive(scope[0])
}

func (r *Registry) scopeToActive(scope Scope) ([]activeContext, error) {
	out := make([]activeContext, 0, len(scope))
	for _, use := range scope {
		r.mu.RLock()
		ctx, ok := r.contexts[use.Name]
		r.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("unknown context %q", use.Name)
		}
		vals, err := r.bindContextParams(ctx, use.Params)
		if err != nil {
			return nil, err
		}
		out = append(out, activeContext{ctx: ctx, values: vals})
	}
	return out, nil
}

// bindContextParams overlays Params on dimensionless header defaults.
// Parse units before any registry lock (Quantity takes r.mu).
func (r *Registry) bindContextParams(ctx *Context, params map[string]Param) (map[string]Quantity, error) {
	vals := make(map[string]Quantity, len(ctx.Defaults)+len(params))
	for k, v := range ctx.Defaults {
		vals[k] = Quantity{reg: r, mag: v}
	}
	for k, p := range params {
		if _, ok := ctx.Defaults[k]; !ok {
			return nil, fmt.Errorf("unknown context parameter %q for %q", k, ctx.Name)
		}
		q, err := r.Quantity(p.Magnitude, p.Unit)
		if err != nil {
			return nil, fmt.Errorf("context param %q: %w", k, err)
		}
		vals[k] = q
	}
	return vals, nil
}

// Compatible reports whether two unit expressions can be converted. An omitted
// scope uses r.active. An explicit Scope is bound the same way as Converter.
func (r *Registry) Compatible(from, to string, scope ...Scope) (bool, error) {
	_, err := r.Converter(from, to, scope...)
	if err == nil {
		return true, nil
	}
	var de *DimensionalityError
	if errors.As(err, &de) {
		return false, nil
	}
	return false, err
}

// Convert converts value from src units to dst units.
func (r *Registry) Convert(value float64, src, dst string, scope ...Scope) (float64, error) {
	c, err := r.Converter(src, dst, scope...)
	if err != nil {
		return 0, err
	}
	return c.Convert(value)
}

// ConverterOp is a parsed conversion from one unit to another.
// Convert does not read Registry.active; the bound scope is snapshotted at compile.
type ConverterOp struct {
	reg    *Registry
	src    UnitsContainer
	dst    UnitsContainer
	scale  float64
	offset float64
	simple bool // y = scale*x + offset
	active []activeContext
}

// Converter returns a reusable conversion between unit expressions.
// Zero extra args snapshots r.active. A service passes an explicit Scope.
func (r *Registry) Converter(src, dst string, scope ...Scope) (*ConverterOp, error) {
	su, err := r.ParseUnits(src)
	if err != nil {
		return nil, err
	}
	du, err := r.ParseUnits(dst)
	if err != nil {
		return nil, err
	}
	active, err := r.bindActive(scope...)
	if err != nil {
		return nil, err
	}
	op := &ConverterOp{reg: r, src: su, dst: du, active: active}
	if su.Equal(du) {
		op.scale, op.simple = 1, true
		return op, nil
	}
	// Try multiplicative factor (no offset units).
	if r.onlyMultiplicative(su) && r.onlyMultiplicative(du) {
		f, err := r.conversionFactor(su, du)
		if err == nil {
			op.scale, op.simple = f, true
			return op, nil
		}
		var de *DimensionalityError
		if !errors.As(err, &de) {
			return nil, err
		}
		srcDim, derr := r.dimensionality(su)
		if derr != nil {
			return nil, derr
		}
		dstDim, derr := r.dimensionality(du)
		if derr != nil {
			return nil, derr
		}
		if r.contextPath(srcDim, dstDim, active) == nil {
			return nil, err
		}
		if err := r.finishContextOp(op, src, dst); err != nil {
			return nil, err
		}
		return op, nil
	}
	// Offset (affine) conversions: y = scale*x + offset. Logarithmic is not affine.
	if !r.hasLogarithmic(su) && !r.hasLogarithmic(du) {
		c0, err := r.convertIn(0, su, du, active)
		if err != nil {
			return nil, err
		}
		c1, err := r.convertIn(1, su, du, active)
		if err != nil {
			return nil, err
		}
		op.scale = c1 - c0
		op.offset = c0
		op.simple = true
	}
	return op, nil
}

// finishContextOp samples the bound hop. Inf/NaN at value 1 is a parameter
// error (chem mw=0). Inf at 0 is a 1/value hop (spectroscopy); leave !simple.
func (r *Registry) finishContextOp(op *ConverterOp, src, dst string) error {
	y1, err := r.convertIn(1, op.src, op.dst, op.active)
	if err != nil {
		return err
	}
	if math.IsNaN(y1) || math.IsInf(y1, 0) {
		return &NonFiniteConversionError{From: src, To: dst}
	}
	if r.hasLogarithmic(op.src) || r.hasLogarithmic(op.dst) {
		return nil
	}
	y0, err := r.convertIn(0, op.src, op.dst, op.active)
	if err != nil || math.IsNaN(y0) || math.IsInf(y0, 0) {
		return nil
	}
	op.scale = y1 - y0
	op.offset = y0
	op.simple = true
	return nil
}

func (r *Registry) hasLogarithmic(u UnitsContainer) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, n := range u.Names() {
		def := r.unitByName[n]
		if def != nil && def.isLogarithmic() {
			return true
		}
	}
	return false
}

func (c *ConverterOp) Convert(v float64) (float64, error) {
	if c.simple {
		return v*c.scale + c.offset, nil
	}
	return c.reg.convertIn(v, c.src, c.dst, c.active)
}

// ConvertN converts src into dst, which must be the same length.
func (c *ConverterOp) ConvertN(dst, src []float64) error {
	if len(dst) != len(src) {
		return fmt.Errorf("ConvertN: length mismatch")
	}
	if c.simple {
		s, o := c.scale, c.offset
		if o == 0 && s == 1 {
			copy(dst, src)
			return nil
		}
		for i, x := range src {
			dst[i] = s*x + o
		}
		return nil
	}
	for i, x := range src {
		y, err := c.reg.convertIn(x, c.src, c.dst, c.active)
		if err != nil {
			return err
		}
		dst[i] = y
	}
	return nil
}

func (r *Registry) onlyMultiplicative(u UnitsContainer) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, n := range u.Names() {
		def := r.unitByName[n]
		if def != nil && !def.isMultiplicative() {
			return false
		}
	}
	return true
}

func (r *Registry) convert(value float64, src, dst UnitsContainer) (float64, error) {
	return r.convertIn(value, src, dst, nil)
}

// emptyActive is a bound empty scope (not nil). convertIn treats nil as r.active.
var emptyActive = []activeContext{}

func (r *Registry) convertIn(value float64, src, dst UnitsContainer, active []activeContext) (float64, error) {
	if src.Equal(dst) {
		return value, nil
	}
	srcOff, err := r.validateOffset(src)
	if err != nil {
		return 0, &DimensionalityError{Units1: src.String(), Units2: dst.String(), Extra: " - In source units, " + err.Error()}
	}
	dstOff, err := r.validateOffset(dst)
	if err != nil {
		return 0, &DimensionalityError{Units1: src.String(), Units2: dst.String(), Extra: " - In destination units, " + err.Error()}
	}

	srcDim, err := r.dimensionality(src)
	if err != nil {
		return 0, err
	}
	dstDim, err := r.dimensionality(dst)
	if err != nil {
		return 0, err
	}

	if !srcDim.Equal(dstDim) {
		if path := r.contextPath(srcDim, dstDim, active); path != nil {
			q := Quantity{reg: r, mag: value, units: src}
			for i := 0; i+1 < len(path); i++ {
				nq, err := r.applyTransform(q, path[i], path[i+1], active)
				if err != nil {
					return 0, err
				}
				nd, err := r.dimensionality(nq.units)
				if err != nil {
					return 0, err
				}
				if !nd.Equal(path[i+1]) {
					return 0, &DimensionalityError{
						Units1: nq.units.String(),
						Units2: path[i+1].String(),
						Dim1:   nd.String(),
						Dim2:   path[i+1].String(),
						Extra:  " - context transform did not produce the destination dimension; pass parameters as quantities with units",
					}
				}
				q = nq
			}
			// No further context hops; remainder is same-dimension (or error).
			return r.convertIn(q.mag, q.units, dst, emptyActive)
		}
		return 0, &DimensionalityError{Units1: src.String(), Units2: dst.String(), Dim1: srcDim.String(), Dim2: dstDim.String()}
	}

	if srcOff == "" && dstOff == "" {
		f, err := r.conversionFactor(src, dst)
		if err != nil {
			return 0, err
		}
		return value * f, nil
	}

	if srcOff != "" {
		for _, n := range dst.Names() {
			if strings.HasPrefix(n, "delta_") {
				return 0, &DimensionalityError{Units1: src.String(), Units2: dst.String()}
			}
		}
		u := r.unitByName[srcOff]
		value = u.converter.ToReference(value)
		src = src.Remove(srcOff)
		src = r.addOffsetRef(srcOff, src)
	}
	if dstOff != "" {
		for _, n := range src.Names() {
			if strings.HasPrefix(n, "delta_") {
				return 0, &DimensionalityError{Units1: src.String(), Units2: dst.String()}
			}
		}
		dstMul := dst.Remove(dstOff)
		dstMul = r.addOffsetRef(dstOff, dstMul)
		f, err := r.conversionFactor(src, dstMul)
		if err != nil {
			return 0, err
		}
		value *= f
		u := r.unitByName[dstOff]
		return u.converter.FromReference(value), nil
	}
	f, err := r.conversionFactor(src, dst)
	if err != nil {
		return 0, err
	}
	return value * f, nil
}

func (r *Registry) validateOffset(u UnitsContainer) (string, error) {
	var nm []string
	for _, n := range u.Names() {
		def := r.unitByName[n]
		if def != nil && !def.isMultiplicative() {
			nm = append(nm, n)
			if u.Get(n) != 1 {
				return "", fmt.Errorf("offset units in higher order")
			}
		}
	}
	if len(nm) > 1 {
		return "", fmt.Errorf("more than one offset unit")
	}
	if len(nm) == 1 {
		if u.Len() > 1 && !r.autoOffsetToBase {
			return "", fmt.Errorf("offset unit used in multiplicative context")
		}
		return nm[0], nil
	}
	return "", nil
}

func (r *Registry) addOffsetRef(offsetUnit string, u UnitsContainer) UnitsContainer {
	def := r.unitByName[offsetUnit]
	if def == nil {
		return u
	}
	if def.isLogarithmic() {
		if !def.reference.Empty() {
			return u.Mul(def.reference)
		}
		return u
	}
	return u.Mul(def.reference)
}

func (r *Registry) conversionFactor(src, dst UnitsContainer) (float64, error) {
	key := src.Key() + "->" + dst.Key()
	r.mu.Lock()
	if f, ok := r.convCache[key]; ok {
		r.mu.Unlock()
		return f, nil
	}
	r.mu.Unlock()

	sd, err := r.dimensionality(src)
	if err != nil {
		return 0, err
	}
	dd, err := r.dimensionality(dst)
	if err != nil {
		return 0, err
	}
	if !sd.Equal(dd) {
		return 0, &DimensionalityError{Units1: src.String(), Units2: dst.String(), Dim1: sd.String(), Dim2: dd.String()}
	}
	ratio := src.Div(dst)
	rr, err := r.rootUnits(ratio, true)
	if err != nil {
		return 0, err
	}
	r.mu.Lock()
	r.convCache[key] = rr.factor
	r.mu.Unlock()
	return rr.factor, nil
}

func (r *Registry) dimensionality(u UnitsContainer) (UnitsContainer, error) {
	if u.Empty() {
		return UnitsContainer{}, nil
	}
	key := u.Key()
	r.mu.Lock()
	if d, ok := r.dimCache[key]; ok {
		r.mu.Unlock()
		return d, nil
	}
	r.mu.Unlock()
	out, err := r.computeDimensionality(u)
	if err != nil {
		return UnitsContainer{}, err
	}
	r.mu.Lock()
	r.dimCache[key] = out
	r.mu.Unlock()
	return out, nil
}

// computeDimensionality does not take r.mu. Load holds the lock and @context
// headers must not call dimensionality (that method locks).
func (r *Registry) computeDimensionality(u UnitsContainer) (UnitsContainer, error) {
	if u.Empty() {
		return UnitsContainer{}, nil
	}
	if d, ok := r.dimCache[u.Key()]; ok {
		return d, nil
	}
	acc := map[string]float64{}
	if err := r.dimRecurse(u, 1, acc); err != nil {
		return UnitsContainer{}, err
	}
	delete(acc, "[]")
	out := NewUnitsContainer(acc)
	r.dimCache[u.Key()] = out
	return out, nil
}

func (r *Registry) dimRecurse(ref UnitsContainer, exp float64, acc map[string]float64) error {
	for _, key := range ref.Names() {
		exp2 := exp * ref.Get(key)
		if isDim(key) {
			d, ok := r.dimensions[key]
			if !ok {
				return fmt.Errorf("%s is not defined as a dimension in the registry", key)
			}
			if !d.isBase {
				if err := r.dimRecurse(d.reference, exp2, acc); err != nil {
					return err
				}
			} else {
				acc[key] += exp2
			}
			continue
		}
		name, err := r.getName(key)
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}
		u := r.unitByName[name]
		if u != nil && !u.reference.Empty() || u != nil && u.isBase {
			if err := r.dimRecurse(u.reference, exp2, acc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Registry) rootUnits(u UnitsContainer, checkNonmult bool) (rootResult, error) {
	if u.Empty() {
		return rootResult{factor: 1, ok: true}, nil
	}
	key := u.Key()
	r.mu.Lock()
	if rr, ok := r.rootCache[key]; ok {
		r.mu.Unlock()
		return rr, nil
	}
	r.mu.Unlock()

	acc := map[string]float64{}
	factor := 1.0
	if err := r.rootRecurse(u, 1, acc, &factor); err != nil {
		return rootResult{}, err
	}
	units := NewUnitsContainer(acc)
	ok := true
	if checkNonmult {
		for _, n := range units.Names() {
			if def := r.unitByName[n]; def != nil && !def.isMultiplicative() {
				ok = false
				factor = math.NaN()
				break
			}
		}
	}
	rr := rootResult{factor: factor, units: units, ok: ok}
	r.mu.Lock()
	r.rootCache[key] = rr
	r.mu.Unlock()
	return rr, nil
}

func (r *Registry) rootRecurse(ref UnitsContainer, exp float64, acc map[string]float64, factor *float64) error {
	for _, key := range ref.Names() {
		exp2 := exp * ref.Get(key)
		name, err := r.getName(key)
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}
		reg := r.unitByName[name]
		if reg == nil {
			return &UndefinedUnitError{Names: []string{name}}
		}
		if reg.isBase {
			acc[name] += exp2
			continue
		}
		*factor *= math.Pow(reg.converter.Scale(), exp2)
		if !reg.reference.Empty() {
			if err := r.rootRecurse(reg.reference, exp2, acc, factor); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Registry) contextPath(src, dst UnitsContainer, active []activeContext) []UnitsContainer {
	if active == nil {
		r.mu.RLock()
		active = r.active
		r.mu.RUnlock()
	}
	if len(active) == 0 {
		return nil
	}
	// BFS over transforms of the provided (or live) active contexts.
	type item struct {
		cur  UnitsContainer
		path []UnitsContainer
	}
	seen := map[string]struct{}{src.Key(): {}}
	q := []item{{src, []UnitsContainer{src}}}
	for len(q) > 0 {
		it := q[0]
		q = q[1:]
		if it.cur.Equal(dst) {
			return it.path
		}
		for _, ac := range active {
			for _, tr := range ac.ctx.transforms {
				if !tr.src.Equal(it.cur) {
					continue
				}
				k := tr.dst.Key()
				if _, ok := seen[k]; ok {
					continue
				}
				seen[k] = struct{}{}
				np := append(append([]UnitsContainer{}, it.path...), tr.dst)
				q = append(q, item{tr.dst, np})
			}
		}
	}
	return nil
}

func (r *Registry) applyTransform(q Quantity, srcDim, dstDim UnitsContainer, active []activeContext) (Quantity, error) {
	if active == nil {
		r.mu.RLock()
		active = r.active
		r.mu.RUnlock()
	}
	var eq string
	vals := map[string]Quantity{}
	for _, ac := range active {
		for k, v := range ac.values {
			vals[k] = v
		}
		for _, tr := range ac.ctx.transforms {
			if tr.src.Equal(srcDim) && tr.dst.Equal(dstDim) {
				eq = tr.equation
			}
		}
	}
	if eq == "" {
		return Quantity{}, fmt.Errorf("no transform from %s to %s", srcDim, dstDim)
	}
	return r.evalContextEq(eq, q, vals)
}

func formatEqQuantity(q Quantity) string {
	if q.units.Empty() {
		return fmt.Sprintf("(%.17g)", q.mag)
	}
	return fmt.Sprintf("(%.17g * %s)", q.mag, q.units.String())
}

func (r *Registry) evalContextEq(eq string, value Quantity, vars map[string]Quantity) (Quantity, error) {
	// Substitute quantities, then parse. Same as Pint: value and params are Quantities.
	s := eq
	for k, v := range vars {
		s = replaceIdent(s, k, formatEqQuantity(v))
	}
	s = replaceIdent(s, "value", formatEqQuantity(value))
	return r.Parse(s)
}

// replaceIdent replaces identifier tokens, not substrings (so n does not split planck_constant).
func replaceIdent(s, ident, repl string) string {
	if ident == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], ident) {
			end := i + len(ident)
			if !identCharBefore(s, i) && !identCharAt(s, end) {
				b.WriteString(repl)
				i = end
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func identCharBefore(s string, i int) bool {
	if i <= 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return isIdentPart(r)
}

func identCharAt(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return isIdentPart(r)
}

// EnableContext activates a named conversion context (e.g. "spectroscopy").
// It mutates Registry.active. Safe for the CLI. Services bind a Scope on
// Converter instead of calling this on a shared registry.
func (r *Registry) EnableContext(name string, params map[string]Param) error {
	r.mu.RLock()
	ctx, ok := r.contexts[name]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown context %q", name)
	}
	vals, err := r.bindContextParams(ctx, params)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = append(r.active, activeContext{ctx: ctx, values: vals})
	r.convCache = map[string]float64{}
	return nil
}

// DisableContext pops the most recently enabled context.
func (r *Registry) DisableContext() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n := len(r.active); n > 0 {
		r.active = r.active[:n-1]
		r.convCache = map[string]float64{}
	}
}

// Define parses and adds a definition line (unit, prefix, or dimension).
func (r *Registry) Define(line string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, err := parseDefinitionLine(line)
	if err != nil {
		return err
	}
	if d == nil {
		return nil
	}
	if err := r.applyDef(*d); err != nil {
		return err
	}
	r.buildCaches()
	return nil
}

// Dimensionality returns the dimension container for a unit expression.
func (r *Registry) Dimensionality(unit string) (UnitsContainer, error) {
	u, err := r.ParseUnits(unit)
	if err != nil {
		return UnitsContainer{}, err
	}
	return r.dimensionality(u)
}
