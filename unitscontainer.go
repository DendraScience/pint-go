package pint

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// unitExp is one name→exponent pair inside a UnitsContainer.
type unitExp struct {
	Name string
	Exp  float64
}

// UnitsContainer is an immutable map from unit (or dimension) name to exponent.
// Names are stored in sorted order for stable equality, hashing, and formatting.
type UnitsContainer struct {
	items []unitExp
}

// NewUnitsContainer builds a container from a name→exponent map, dropping zeros.
func NewUnitsContainer(m map[string]float64) UnitsContainer {
	if len(m) == 0 {
		return UnitsContainer{}
	}
	items := make([]unitExp, 0, len(m))
	for k, v := range m {
		if v != 0 {
			items = append(items, unitExp{Name: k, Exp: v})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return UnitsContainer{items: items}
}

func unitPair(name string, exp float64) UnitsContainer {
	if exp == 0 || name == "" {
		return UnitsContainer{}
	}
	return UnitsContainer{items: []unitExp{{Name: name, Exp: exp}}}
}

// Empty reports whether the container has no units (dimensionless).
func (u UnitsContainer) Empty() bool { return len(u.items) == 0 }

// Len returns the number of distinct names.
func (u UnitsContainer) Len() int { return len(u.items) }

// Get returns the exponent for name, or 0 if absent.
func (u UnitsContainer) Get(name string) float64 {
	for _, it := range u.items {
		if it.Name == name {
			return it.Exp
		}
		if it.Name > name {
			return 0
		}
	}
	return 0
}

// Has reports whether name is present with a non-zero exponent.
func (u UnitsContainer) Has(name string) bool { return u.Get(name) != 0 }

// Names returns the sorted unit names.
func (u UnitsContainer) Names() []string {
	out := make([]string, len(u.items))
	for i, it := range u.items {
		out[i] = it.Name
	}
	return out
}

// Items returns a copy of the name→exponent map.
func (u UnitsContainer) Items() map[string]float64 {
	m := make(map[string]float64, len(u.items))
	for _, it := range u.items {
		m[it.Name] = it.Exp
	}
	return m
}

// Equal reports whether two containers have the same names and exponents.
func (u UnitsContainer) Equal(o UnitsContainer) bool {
	if len(u.items) != len(o.items) {
		return false
	}
	for i := range u.items {
		if u.items[i].Name != o.items[i].Name || u.items[i].Exp != o.items[i].Exp {
			return false
		}
	}
	return true
}

// Add returns a container with exp added to name.
func (u UnitsContainer) Add(name string, exp float64) UnitsContainer {
	if exp == 0 || name == "" {
		return u
	}
	m := u.Items()
	m[name] += exp
	return NewUnitsContainer(m)
}

// Remove returns a container without the given names.
func (u UnitsContainer) Remove(names ...string) UnitsContainer {
	if len(names) == 0 {
		return u
	}
	drop := make(map[string]struct{}, len(names))
	for _, n := range names {
		drop[n] = struct{}{}
	}
	m := make(map[string]float64, len(u.items))
	for _, it := range u.items {
		if _, ok := drop[it.Name]; !ok {
			m[it.Name] = it.Exp
		}
	}
	return NewUnitsContainer(m)
}

// Rename returns a container with oldName replaced by newName.
func (u UnitsContainer) Rename(oldName, newName string) UnitsContainer {
	m := u.Items()
	if exp, ok := m[oldName]; ok {
		delete(m, oldName)
		m[newName] += exp
	}
	return NewUnitsContainer(m)
}

// Mul returns the product of two containers (exponents added).
func (u UnitsContainer) Mul(o UnitsContainer) UnitsContainer {
	if o.Empty() {
		return u
	}
	if u.Empty() {
		return o
	}
	m := u.Items()
	for _, it := range o.items {
		m[it.Name] += it.Exp
	}
	return NewUnitsContainer(m)
}

// Div returns u / o (exponents subtracted).
func (u UnitsContainer) Div(o UnitsContainer) UnitsContainer {
	if o.Empty() {
		return u
	}
	m := u.Items()
	for _, it := range o.items {
		m[it.Name] -= it.Exp
	}
	return NewUnitsContainer(m)
}

// Pow returns the container with every exponent multiplied by p.
func (u UnitsContainer) Pow(p float64) UnitsContainer {
	if p == 1 {
		return u
	}
	if p == 0 || u.Empty() {
		return UnitsContainer{}
	}
	m := make(map[string]float64, len(u.items))
	for _, it := range u.items {
		m[it.Name] = it.Exp * p
	}
	return NewUnitsContainer(m)
}

// Key is a stable string used as a map key.
func (u UnitsContainer) Key() string { return u.String() }

func formatExp(e float64) string {
	if e == 1 {
		return ""
	}
	if e == math.Trunc(e) && math.Abs(e) < 1e12 {
		return fmt.Sprintf("%d", int64(e))
	}
	return fmt.Sprintf("%g", e)
}

// String renders as "meter" or "meter ** 2 / second" in Pint's default style.
func (u UnitsContainer) String() string {
	if u.Empty() {
		return "dimensionless"
	}
	var num, den []string
	for _, it := range u.items {
		if it.Exp > 0 {
			s := it.Name
			if it.Exp != 1 {
				s += " ** " + formatExp(it.Exp)
			}
			num = append(num, s)
		} else {
			e := -it.Exp
			s := it.Name
			if e != 1 {
				s += " ** " + formatExp(e)
			}
			den = append(den, s)
		}
	}
	ns := strings.Join(num, " * ")
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

func (u UnitsContainer) compactString() string {
	if u.Empty() {
		return ""
	}
	var b strings.Builder
	for i, it := range u.items {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(it.Name)
		if it.Exp != 1 {
			fmt.Fprintf(&b, "^%s", formatExp(it.Exp))
		}
	}
	return b.String()
}

func almostEqual(a, b, rtol, atol float64) bool {
	if a == b {
		return true
	}
	diff := math.Abs(a - b)
	return diff <= atol+rtol*math.Max(math.Abs(a), math.Abs(b))
}
