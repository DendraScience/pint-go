// Package pinttest holds shared fixtures matching Pint's conftest.py.
package pinttest

import (
	"testing"

	"github.com/dendrascience/pint-go"
)

// Tiny is Pint's _TINY definition file (prefixes + meter/second + angstrom/minute).
const Tiny = `
yocto- = 1e-24 = y-
zepto- = 1e-21 = z-
atto- =  1e-18 = a-
femto- = 1e-15 = f-
pico- =  1e-12 = p-
nano- =  1e-9  = n-
micro- = 1e-6  = μ- = µ- = u- = mu- = mc-
milli- = 1e-3  = m-
centi- = 1e-2  = c-
deci- =  1e-1  = d-
deca- =  1e+1  = da- = deka-
hecto- = 1e2   = h-
kilo- =  1e3   = k-
mega- =  1e6   = M-
giga- =  1e9   = G-
tera- =  1e12  = T-
peta- =  1e15  = P-
exa- =   1e18  = E-
zetta- = 1e21  = Z-
yotta- = 1e24  = Y-

meter = [length] = m = metre
second = [time] = s = sec

angstrom = 1e-10 * meter = Å = ångström = Å
minute = 60 * second = min
`

func Empty(t *testing.T) *pint.Registry {
	t.Helper()
	return pint.NewEmptyRegistry()
}

func TinyReg(t *testing.T) *pint.Registry {
	t.Helper()
	r, err := pint.NewRegistryFrom(Tiny)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func Default(t *testing.T) *pint.Registry {
	t.Helper()
	r, err := pint.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func AutoOffset(t *testing.T) *pint.Registry {
	t.Helper()
	r, err := pint.NewRegistry(pint.WithAutoOffsetToBase(true))
	if err != nil {
		t.Fatal(err)
	}
	return r
}
