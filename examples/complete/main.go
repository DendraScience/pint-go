// Command complete prints Registry.Complete JSON for a partial unit expression.
//
//	go run ./examples/complete 'watt / h'
//	go run ./examples/complete -n 10 'watt /'
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dendrascience/pint-go"
)

func main() {
	limit := flag.Int("n", 20, "suggestion cap")
	flag.Parse()
	partial := strings.Join(flag.Args(), " ")
	if partial == "" {
		fmt.Fprintln(os.Stderr, "usage: complete [-n limit] <partial unit expression>")
		os.Exit(2)
	}

	ureg, err := pint.NewRegistry()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ureg.Complete(partial, *limit)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
