// Command demo is a walkthrough of pint-go, following Pint's tutorial
// and user guides. Each function below is one topic: parse, arithmetic,
// convert, temperature, logs, contexts, systems, ConvertN, π theorem.
//
// Start here: NewRegistry loads the same definition files Pint ships.
// Everything else hangs off that registry — quantities, units, converters.
//
//	go run ./examples/demo
//	go run ./examples/demo -only temp
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dendrascience/pint-go"
)

func main() {
	only := flag.String("only", "", "run one section: "+strings.Join(sectionNames(), ", "))
	flag.Parse()

	// One registry for the whole tour. Later sections that call Define change it.
	ureg, err := pint.NewRegistry()
	if err != nil {
		fatalf("NewRegistry: %v", err)
	}

	fmt.Println("pint-go  —  Pint units, pure Go")
	fmt.Println("definitions: default_en.txt + constants_en.txt")
	fmt.Println()
	fmt.Println("  20 degC        → degF          68     (absolute temperature)")
	fmt.Println("  5  delta_degC  → delta_degF     9     (interval)")

	ran := 0
	for _, s := range sections {
		if *only != "" && s.name != *only {
			continue
		}
		s.fn(ureg)
		ran++
	}
	if *only != "" && ran == 0 {
		fatalf("unknown section %q (want one of: %s)", *only, strings.Join(sectionNames(), ", "))
	}
}

type section struct {
	name string
	fn   func(*pint.Registry)
}

// -only names. Skip around with: go run ./examples/demo -only temp
var sections = []section{
	{"parse", parseAndQuantity},
	{"arithmetic", arithmetic},
	{"convert", converting},
	{"compact", simplify},
	{"format", formatting},
	{"errors", errorsAndChecks},
	{"temp", temperature},
	{"log", logarithmic},
	{"context", contexts},
	{"systems", groupsAndSystems},
	{"define", customUnits},
	{"converter", converterHotPath},
	{"pi", piTheorem},
}

func sectionNames() []string {
	out := make([]string, len(sections))
	for i, s := range sections {
		out[i] = s.name
	}
	return out
}

// Parse a string into a Quantity, then inspect magnitude, units, and dimensions.
// Prefixes (kilo), aliases (km), plurals, and unicode exponents are handled by
// the same parser Pint uses in spirit — see pint.Preprocess for the text rewrite.
func parseAndQuantity(ureg *pint.Registry) {
	heading("Parse & quantity", "docs/getting/tutorial.rst — Defining a Quantity")

	distance := mustQ(ureg, "24 meter")
	kv("parse", distance.String())
	kv("magnitude", fmt.Sprint(distance.Magnitude()))
	kv("units", distance.Units().String())
	dim, err := distance.Dimensionality()
	must(err)
	kv("dimensionality", dim.String())

	for _, s := range []string{"42 kilometers", "42 km", "2.54cm", "2.54 * centimeter", "4.2×10⁻¹² ft/s"} {
		kv("parse "+s, mustQ(ureg, s).String())
	}
	kv("preprocess  m²", pint.Preprocess("meter²"))
	kv("preprocess  square meter", pint.Preprocess("square meter"))
	kv("pi in an expression", mustQ(ureg, "2 * pi * radian").String())
}

// Arithmetic keeps units attached. Div/Mul combine dimensions; Add converts
// compatible units first (foot + inch). Quantity vs Unit: a Unit is magnitude 1.
func arithmetic(ureg *pint.Registry) {
	heading("Arithmetic", "docs/getting/tutorial.rst — Defining a Quantity")

	distance := mustQty(ureg, 24, "meter")
	time := mustQty(ureg, 8, "second")
	speed, err := distance.Div(time)
	must(err)
	kv("24 m / 8 s", speed.String())
	dim, err := speed.Dimensionality()
	must(err)
	kv("dimensionality", dim.String())

	km := mustQty(ureg, 42, "kilometers")
	inM, err := km.To("meter")
	must(err)
	kv("42 kilometer", inM.String())

	ht := mustQty(ureg, 5, "foot")
	inc := mustQty(ureg, 9, "inch")
	// Add converts inch → foot, then adds. Parse("5ft+9in") would not: the
	// expression parser only adds terms that already share a unit.
	sum, err := ht.Add(inc)
	must(err)
	kv("5 ft + 9 in", sum.String())
	root, err := sum.ToRootUnits()
	must(err)
	kv("  → root units", root.String())

	m := mustUnit(ureg, "meter")
	s := mustUnit(ureg, "second")
	kv("Unit meter/second", m.Div(s).String())
	kv("Unit meter**2", m.Pow(2).String())
}

// To() returns a new quantity; the original is unchanged. Compatible() is
// dimensionality, not a string match — knot and m/s are compatible, joule is not.
func converting(ureg *pint.Registry) {
	heading("Converting", "docs/getting/tutorial.rst — Converting to different units")

	speed := mustQ(ureg, "3 meter/second")
	inchMin, err := speed.To("inch/minute")
	must(err)
	kv("3 m/s → inch/minute", inchMin.String())
	kv("original unchanged", speed.String())

	ok, err := ureg.Compatible("meter/second", "knot")
	must(err)
	kv("compatible m/s, knot", fmt.Sprint(ok))
	ok, err = ureg.Compatible("meter/second", "joule")
	must(err)
	kv("compatible m/s, joule", fmt.Sprint(ok))

	names, err := ureg.CompatibleUnits("meter/second", "")
	must(err)
	if len(names) > 8 {
		names = names[:8]
	}
	kv("compatible units (first)", strings.Join(names, ", "))

	kPa := mustQ(ureg, "3.2 kPa")
	mbar, err := kPa.To("mbar")
	must(err)
	kv("3.2 kPa → mbar", mbar.String())
}

// ToCompact picks an SI prefix so the number is human-scale (Hz → THz).
// InferBaseUnit strips prefixes (mm·nm → m²).
func simplify(ureg *pint.Registry) {
	heading("Compact & root units", "docs/getting/tutorial.rst — Simplifying units")

	wl := mustQ(ureg, "1550 nm")
	c := mustQ(ureg, "speed_of_light")
	hz, err := c.Div(wl)
	must(err)
	hz, err = hz.To("Hz")
	must(err)
	kv("c / 1550 nm", hz.String())
	compact, err := hz.ToCompact()
	must(err)
	kv("to_compact", compact.String())

	u, err := ureg.ParseUnits("millimeter * nanometer")
	must(err)
	inf, err := ureg.InferBaseUnit(u)
	must(err)
	kv("infer mm·nm", inf.String())
}

// Format("~") uses symbols (m/s²) instead of full names (meter / second ** 2).
func formatting(ureg *pint.Registry) {
	heading("Formatting", "docs/getting/tutorial.rst — String formatting")

	q := mustQty(ureg, 1.3, "meter/second**2")
	kv("String()", q.String())
	kv(`Format("~")`, q.Format("~"))
	u := mustUnit(ureg, "meter**2/second")
	kv(`Unit.Format("~")`, u.Format("~"))
}

// Failures you should expect: unknown names, mismatched dimensions, and
// multiplying offset temperatures (degC * degC is ambiguous — use kelvin or delta).
func errorsAndChecks(ureg *pint.Registry) {
	heading("Errors Pint would raise", "docs/getting/tutorial.rst")

	_, err := ureg.Parse("23 snail_speed")
	kv("undefined unit", errString(err))

	speed := mustQ(ureg, "3 meter/second")
	_, err = speed.To("joule")
	kv("m/s → joule", errString(err))

	a := mustQty(ureg, 1, "degC")
	b := mustQty(ureg, 1, "degC")
	_, err = a.Mul(b)
	kv("degC * degC", errString(err))

	kv("HasUnit(meter)", fmt.Sprint(ureg.HasUnit("meter")))
	kv("HasUnit(snail_speed)", fmt.Sprint(ureg.HasUnit("snail_speed")))
}

// Offset vs interval: 20 degC is a point on the scale (→ 68 degF).
// 5 delta_degC is a difference (→ 9 delta_degF). Subtracting two degC
// values yields a delta. This is the Dendra/Pint contract.
func temperature(ureg *pint.Registry) {
	heading("Temperature (offset vs delta)", "docs/user/nonmult.rst")

	home := mustQty(ureg, 25.4, "degC")
	kv("25.4 degC → degF", mustTo(home, "degF").String())
	kv("25.4 degC → kelvin", mustTo(home, "kelvin").String())
	kv("25.4 degC → degR", mustTo(home, "degR").String())

	// Convert is the scalar API (no Quantity). Same rules as q.To.
	dendraC, err := ureg.Convert(20, "degC", "degF")
	must(err)
	kv("20 degC → degF", fmt.Sprint(dendraC))
	dendraD, err := ureg.Convert(5, "delta_degC", "delta_degF")
	must(err)
	kv("5 delta_degC → delta_degF", fmt.Sprint(dendraD))

	hot := mustQty(ureg, 25.4, "degC")
	cold := mustQty(ureg, 10, "degC")
	diff, err := hot.Sub(cold)
	must(err)
	kv("25.4 degC − 10 degC", diff.String())

	delta := mustQty(ureg, 10, "delta_degC")
	warmer, err := hot.Add(delta)
	must(err)
	kv("25.4 degC + 10 delta_degC", warmer.String())
	kv("12.3 delta_degC → K", mustTo(mustQty(ureg, 12.3, "delta_degC"), "kelvin").String())
	kv("12.3 delta_degC → delta_degF", mustTo(mustQty(ureg, 12.3, "delta_degC"), "delta_degF").String())
}

// dB / dBm / octave are logarithmic (like offset units, not a plain scale).
// 20 dBm is 100 mW; 0 dBm is 1 mW. 1 octave is a factor of 2.
func logarithmic(ureg *pint.Registry) {
	heading("Logarithmic units", "docs/user/log_units.rst")

	dbm := mustQty(ureg, 20, "dBm")
	kv("20 dBm → mW", mustTo(dbm, "mW").String())
	kv("0 dBm → mW", mustTo(mustQty(ureg, 0, "dBm"), "mW").String())
	kv("20 dB → dimensionless", mustTo(mustQty(ureg, 20, "dB"), "dimensionless").String())
	kv("1 octave", mustTo(mustQty(ureg, 1, "octave"), "dimensionless").String())
	kv("1 decade", mustTo(mustQty(ureg, 1, "decade"), "dimensionless").String())
	v, err := ureg.Convert(0, "dBm", "dBu")
	must(err)
	kv("0 dBm → dBu", fmt.Sprint(v))
}

// Contexts add extra conversion relations. Spectroscopy (sp) links length
// and frequency via c/λ, so nm → THz works only after EnableContext.
// examples/convert turns sp (and energy, …) on by default; the library does not.
func contexts(ureg *pint.Registry) {
	heading("Contexts (spectroscopy)", "docs/user/contexts.rst")

	q := mustQty(ureg, 532, "nm")
	_, err := q.To("terahertz")
	kv("532 nm → THz (no context)", errString(err))

	must(ureg.EnableContext("sp", nil)) // nil → default n=1 (index of refraction)
	defer ureg.DisableContext()         // pop this context when the section ends
	thz, err := q.To("terahertz")
	must(err)
	kv("532 nm → THz (context sp)", thz.String())

	ctx, err := ureg.ContextByName("spectroscopy")
	must(err)
	kv("context name", ctx.Name)
	if len(ctx.Aliases) > 0 {
		kv("aliases", strings.Join(ctx.Aliases, ", "))
	}
}

// A system names preferred bases (SI, cgs, US). A group is just a named
// bag of units. BaseUnits(unit, system) is how convert --sys is implemented.
func groupsAndSystems(ureg *pint.Registry) {
	heading("Groups & systems", "docs/user/systems.rst")

	sys, err := ureg.System("SI")
	must(err)
	kv("system SI", sys.Name)
	_, err = ureg.System("cgs")
	must(err)
	_, err = ureg.System("US")
	must(err)
	kv("systems", "SI, cgs, US, imperial, mks, atomic, Planck")

	f, u, err := ureg.BaseUnits("pound", "SI")
	must(err)
	kv("1 pound in SI", fmt.Sprintf("%g %s", f, u))
	f, u, err = ureg.BaseUnits("meter", "cgs")
	must(err)
	kv("1 meter in cgs", fmt.Sprintf("%g %s", f, u))

	g, err := ureg.Group("Avoirdupois")
	must(err)
	kv("group Avoirdupois", strings.Join(g.Members(), ", "))

	names, err := ureg.CompatibleUnits("meter", "USCSLengthInternational")
	must(err)
	if len(names) > 8 {
		names = names[:8]
	}
	kv("length in USCS intl", strings.Join(names, ", "))
}

// ParseDefinition inspects a Pint definition line without loading it.
// Define adds it to this registry (not concurrent with convert).
func customUnits(ureg *pint.Registry) {
	heading("Define your own", "docs/advanced/defining.rst")

	def, err := pint.ParseDefinition("hour = 60 * minute")
	must(err)
	kv("ParseDefinition", fmt.Sprintf("%s %s → %s", def.Kind, def.Name, def.Reference))

	must(ureg.Define("heartbeats = 1 / minute"))
	hr := mustQ(ureg, "72 heartbeats")
	kv("72 heartbeats", hr.String())
	perS, err := hr.To("1/second")
	must(err)
	kv("  → 1/second", perS.String())
}

// For many samples, parse units once: Converter("degC","degF"), then ConvertN
// over []float64. Do not Parse per row. Same abs-vs-delta rule as Quantity.To.
func converterHotPath(ureg *pint.Registry) {
	heading("Converter + ConvertN", "README — Dendra query path")

	c, err := ureg.Converter("degC", "degF")
	must(err)
	v, err := c.Convert(20)
	must(err)
	kv("Converter degC→degF (20)", fmt.Sprint(v))

	src := []float64{-40, 0, 20, 37, 100}
	dst := make([]float64, len(src))
	must(c.ConvertN(dst, src))
	var b strings.Builder
	for i, x := range src {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(fmt.Sprintf("%g°C=%g°F", x, dst[i]))
	}
	kv("ConvertN", b.String())

	d, err := ureg.Converter("delta_degC", "delta_degF")
	must(err)
	dv, err := d.Convert(5)
	must(err)
	kv("delta Converter (5)", fmt.Sprint(dv))
}

// PiTheorem finds dimensionless groups from named unit expressions
// (pendulum: T² g / L). Useful when you know the variables but not the formula.
func piTheorem(ureg *pint.Registry) {
	heading("Buckingham π theorem", "docs/advanced/pitheorem.rst")

	groups, err := ureg.PiTheorem(map[string]string{
		"V": "meter/second",
		"T": "second",
		"L": "meter",
	})
	must(err)
	kv("V, T, L", formatPi(groups))

	pendulum, err := ureg.PiTheorem(map[string]string{
		"T": "second",
		"M": "kilogram",
		"L": "meter",
		"g": "meter/second**2",
	})
	must(err)
	kv("pendulum T,M,L,g", formatPi(pendulum))
}

func formatPi(groups []map[string]float64) string {
	if len(groups) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(groups))
	for _, g := range groups {
		keys := make([]string, 0, len(g))
		for k := range g {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		terms := make([]string, 0, len(keys))
		for _, k := range keys {
			terms = append(terms, fmt.Sprintf("%s^%g", k, g[k]))
		}
		parts = append(parts, strings.Join(terms, " · "))
	}
	return strings.Join(parts, " ; ")
}

func heading(title, pintRef string) {
	fmt.Printf("\n── %s\n", title)
	if pintRef != "" {
		fmt.Printf("   Pint: %s\n", pintRef)
	}
}

func kv(k, v string) {
	fmt.Printf("  %-32s %s\n", k, v)
}

// mustQ is ureg.Parse (full string). mustQty is ureg.Quantity(mag, unit).
func mustQ(ureg *pint.Registry, s string) pint.Quantity {
	q, err := ureg.Parse(s)
	must(err)
	return q
}

func mustQty(ureg *pint.Registry, mag float64, unit string) pint.Quantity {
	q, err := ureg.Quantity(mag, unit)
	must(err)
	return q
}

func mustUnit(ureg *pint.Registry, s string) pint.Unit {
	u, err := ureg.Unit(s)
	must(err)
	return u
}

func mustTo(q pint.Quantity, dest string) pint.Quantity {
	out, err := q.To(dest)
	must(err)
	return out
}

func must(err error) {
	if err != nil {
		fatalf("%v", err)
	}
}

func errString(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "demo: "+format+"\n", args...)
	os.Exit(1)
}
