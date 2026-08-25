// Command bench is a crude throughput baseline for pint-go. Pint has no
// comparable CLI; this is ours to notice regressions, not a formal benchmark.
//
// Two cases:
//
//	  same   — one Converter, ConvertN over a large slice (query hot path)
//	  mixed  — several Converters, Convert() round-robin (varied unit pairs)
//
//		go run ./examples/bench
//		go run ./examples/bench -n 2000000
//
// For Go's benchmark harness: go test -bench=BenchmarkConvert -benchmem .
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/dendrascience/pint-go"
)

var pairs = [][2]string{
	{"m", "ft"},
	{"kPa", "mbar"},
	{"degC", "degF"},
	{"km", "mi"},
	{"kg", "lb"},
	{"W", "horsepower"},
}

func main() {
	n := flag.Int("n", 1_000_000, "conversions per case")
	flag.Parse()
	if *n < 1 {
		fmt.Fprintln(os.Stderr, "bench: -n must be positive")
		os.Exit(2)
	}

	ureg, err := pint.NewRegistry()
	if err != nil {
		fatalf("NewRegistry: %v", err)
	}

	fmt.Printf("pint-go convert baseline  n=%d\n\n", *n)

	samePair(ureg, *n)
	mixedPairs(ureg, *n)
}

// samePair is the Dendra-shaped path: parse units once, then ConvertN.
func samePair(ureg *pint.Registry, n int) {
	c, err := ureg.Converter("m", "ft")
	if err != nil {
		fatalf("%v", err)
	}
	src := make([]float64, n)
	dst := make([]float64, n)
	for i := range src {
		src[i] = float64(i) * 0.001
	}

	start := time.Now()
	if err := c.ConvertN(dst, src); err != nil {
		fatalf("%v", err)
	}
	report("same  ConvertN  m → ft", n, time.Since(start))
}

// mixedPairs pretends each sample is a different unit pair. Converters are
// still built once; only the numeric convert runs in the timed loop.
func mixedPairs(ureg *pint.Registry, n int) {
	convs := make([]*pint.ConverterOp, len(pairs))
	for i, p := range pairs {
		c, err := ureg.Converter(p[0], p[1])
		if err != nil {
			fatalf("%s → %s: %v", p[0], p[1], err)
		}
		convs[i] = c
	}

	start := time.Now()
	var sink float64
	for i := 0; i < n; i++ {
		v, err := convs[i%len(convs)].Convert(float64(i) * 0.001)
		if err != nil {
			fatalf("%v", err)
		}
		sink = v
	}
	_ = sink
	report("mixed Convert    6 pairs", n, time.Since(start))
}

func report(name string, n int, d time.Duration) {
	ns := float64(d.Nanoseconds()) / float64(n)
	rate := float64(n) / d.Seconds()
	fmt.Printf("  %-28s  %8.1f ns/conv  %10.0f conv/s  %s\n", name, ns, rate, d.Round(time.Microsecond))
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "bench: "+format+"\n", args...)
	os.Exit(1)
}
