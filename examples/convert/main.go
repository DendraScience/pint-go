// Command convert is a small pint-convert analog.
//
// Flow, same as Pint's CLI:
//  1. Build a registry from the bundled definition files.
//  2. Turn on the contexts pint-convert always enables (spectroscopy, energy, …)
//     so "540nm kcal/mol" works even though length and energy/mole are different
//     dimensions.
//  3. Parse the input as a quantity ("225lb", "km", "20degC").
//  4. Convert: explicit dest units, else a named system, else SI.
//
// The library itself does not enable those contexts. Only this CLI does.
//
//	go run ./examples/convert 225lb
//	go run ./examples/convert 102kg lb
//	go run ./examples/convert --sys US 102kg
//	go run ./examples/convert 540nm kcal/mol
//	go run ./examples/convert 20degC degF
//
// Mixed-unit sums such as 7ft+2in need Quantity.Add (see examples/demo).
// Parse only adds terms that already share the same unit.
//
// See Pint: docs/dev/pint-convert.rst
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/dendrascience/pint-go"
)

// pint-convert enables these on startup (pint/pint_convert.py).
// sp: length ↔ frequency ↔ energy. energy: J ↔ kcal/mol. boltzmann: K ↔ J.
var pintConvertContexts = []string{"Gau", "ESU", "sp", "energy", "boltzmann"}

func main() {
	sys := flag.String("sys", "", "convert to a named system (SI, cgs, US, imperial, mks, …)")
	prec := flag.Int("p", 12, "significant figures")
	var extra []string
	flag.Func("context", "also enable a context (repeatable; chem, textile, …). Gau, ESU, sp, energy, and boltzmann are already on", func(s string) error {
		extra = append(extra, s)
		return nil
	})
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 || len(args) > 2 {
		usage()
		os.Exit(2)
	}

	// NewRegistry loads default_en.txt + constants_en.txt. Contexts stay off
	// until EnableContext — same as Pint's UnitRegistry.
	ureg, err := pint.NewRegistry()
	if err != nil {
		fatalf("NewRegistry: %v", err)
	}
	if err := enableContexts(ureg, pintConvertContexts...); err != nil {
		fatalf("%v", err)
	}
	if err := enableContexts(ureg, extra...); err != nil {
		fatalf("%v", err)
	}

	// Parse accepts "3.2kPa", "km" (magnitude 1), "20degC", unicode like "4.2×10⁻¹² ft/s".
	q, err := ureg.Parse(args[0])
	if err != nil {
		fatalf("parse %q: %v", args[0], err)
	}

	// Two args: 102kg lb. One arg: fold into SI (or --sys US, cgs, …).
	var out pint.Quantity
	switch {
	case len(args) == 2:
		out, err = q.To(args[1])
	case *sys != "":
		out, err = toSystem(ureg, q, *sys)
	default:
		out, err = toSystem(ureg, q, "SI")
	}
	if err != nil {
		fatalf("%v", err)
	}

	src := formatQty(q, *prec)
	dst := formatQty(out, *prec)
	fmt.Printf("%s = %s\n", src, dst)
}

// toSystem rewrites q into that system's base units (SI → kg, m, s, …; cgs → g, cm, s).
func toSystem(ureg *pint.Registry, q pint.Quantity, sys string) (pint.Quantity, error) {
	_, units, err := ureg.BaseUnits(q.Units().String(), sys)
	if err != nil {
		return pint.Quantity{}, err
	}
	return q.ToUnits(units)
}

func formatQty(q pint.Quantity, prec int) string {
	mag := strconv.FormatFloat(q.Magnitude(), 'g', prec, 64)
	u := q.Units().String()
	if u == "" || u == "dimensionless" {
		return mag
	}
	return mag + " " + u
}

func usage() {
	fmt.Fprintf(os.Stderr, `convert — pint-go analog of Pint's pint-convert

Usage:
  convert [flags] QUANTITY [DEST]

If DEST is omitted, QUANTITY is converted to SI (or --sys).

Examples:
  convert 225lb
  convert 102kg lb
  convert km mi
  convert 3.2kPa mbar
  convert 20degC degF
  convert --sys US 102kg
  convert 540nm kcal/mol
  convert 532nm THz
  convert --context textile 1 tex

Flags:
`)
	flag.PrintDefaults()
}

func enableContexts(ureg *pint.Registry, names ...string) error {
	for _, name := range names {
		// nil = use the context's default parameters (e.g. spectroscopy n=1).
		if err := ureg.EnableContext(name, nil); err != nil {
			return err
		}
	}
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "convert: "+format+"\n", args...)
	os.Exit(1)
}
