package pint

import (
	"fmt"
	"math"
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

// Compatible reports whether two unit expressions share dimensionality.
func (r *Registry) Compatible(from, to string) (bool, error) {
	a, err := r.ParseUnits(from)
	if err != nil {
		return false, err
	}
	b, err := r.ParseUnits(to)
	if err != nil {
		return false, err
	}
	da, err := r.dimensionality(a)
	if err != nil {
		return false, err
	}
	db, err := r.dimensionality(b)
	if err != nil {
		return false, err
	}
	if da.Equal(db) {
		return true, nil
	}
	if r.contextPath(da, db) != nil {
		return true, nil
	}
	return false, nil
}

// Convert converts value from src units to dst units.
func (r *Registry) Convert(value float64, src, dst string) (float64, error) {
	c, err := r.Converter(src, dst)
	if err != nil {
		return 0, err
	}
	return c.Convert(value)
}

// Converter is a parsed conversion from one unit to another.
type ConverterOp struct {
	reg    *Registry
	src    UnitsContainer
	dst    UnitsContainer
	scale  float64
	offset float64
	simple bool // y = scale*x + offset
}

// Converter returns a reusable conversion between unit expressions.
func (r *Registry) Converter(src, dst string) (*ConverterOp, error) {
	su, err := r.ParseUnits(src)
	if err != nil {
		return nil, err
	}
	du, err := r.ParseUnits(dst)
	if err != nil {
		return nil, err
	}
	op := &ConverterOp{reg: r, src: su, dst: du}
	if su.Equal(du) {
		op.scale, op.simple = 1, true
		return op, nil
	}
	// Try multiplicative factor (no offset units).
	if r.onlyMultiplicative(su) && r.onlyMultiplicative(du) {
		f, err := r.conversionFactor(su, du)
		if err != nil {
			return nil, err
		}
		op.scale, op.simple = f, true
		return op, nil
	}
	// Offset (affine) conversions: y = scale*x + offset. Logarithmic is not affine.
	if !r.hasLogarithmic(su) && !r.hasLogarithmic(du) {
		c0, err := r.convert(0, su, du)
		if err != nil {
			return nil, err
		}
		c1, err := r.convert(1, su, du)
		if err != nil {
			return nil, err
		}
		op.scale = c1 - c0
		op.offset = c0
		op.simple = true
	}
	return op, nil
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
	return c.reg.convert(v, c.src, c.dst)
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
		y, err := c.reg.convert(x, c.src, c.dst)
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
		if path := r.contextPath(srcDim, dstDim); path != nil {
			q := Quantity{reg: r, mag: value, units: src}
			for i := 0; i+1 < len(path); i++ {
				nq, err := r.applyTransform(q, path[i], path[i+1])
				if err != nil {
					return 0, err
				}
				q = nq
			}
			return r.convert(q.mag, q.units, dst)
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
	acc := map[string]float64{}
	if err := r.dimRecurse(u, 1, acc); err != nil {
		return UnitsContainer{}, err
	}
	delete(acc, "[]")
	out := NewUnitsContainer(acc)
	r.mu.Lock()
	r.dimCache[key] = out
	r.mu.Unlock()
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

func (r *Registry) contextPath(src, dst UnitsContainer) []UnitsContainer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.active) == 0 {
		return nil
	}
	type node struct{ u UnitsContainer }
	// BFS over transforms of active contexts.
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
		for _, ac := range r.active {
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

func (r *Registry) applyTransform(q Quantity, srcDim, dstDim UnitsContainer) (Quantity, error) {
	r.mu.RLock()
	var eq string
	vals := map[string]float64{}
	for _, ac := range r.active {
		for k, v := range ac.values {
			vals[k] = v
		}
		for _, tr := range ac.ctx.transforms {
			if tr.src.Equal(srcDim) && tr.dst.Equal(dstDim) {
				eq = tr.equation
			}
		}
	}
	r.mu.RUnlock()
	if eq == "" {
		return Quantity{}, fmt.Errorf("no transform from %s to %s", srcDim, dstDim)
	}
	return r.evalContextEq(eq, q, vals)
}

func (r *Registry) evalContextEq(eq string, value Quantity, vars map[string]float64) (Quantity, error) {
	// Substitute numeric variables, then parse. Pint binds `value` as a Quantity.
	s := eq
	for k, v := range vars {
		s = replaceIdent(s, k, fmt.Sprintf("(%.17g)", v))
	}
	var val string
	if value.units.Empty() {
		val = fmt.Sprintf("(%.17g)", value.mag)
	} else {
		val = fmt.Sprintf("(%.17g * %s)", value.mag, value.units.String())
	}
	s = replaceIdent(s, "value", val)
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
func (r *Registry) EnableContext(name string, vars map[string]float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx, ok := r.contexts[name]
	if !ok {
		return fmt.Errorf("unknown context %q", name)
	}
	vals := map[string]float64{}
	for k, v := range ctx.Defaults {
		vals[k] = v
	}
	for k, v := range vars {
		vals[k] = v
	}
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
