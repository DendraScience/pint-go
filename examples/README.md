# Examples

Runnable programs that show pint-go the way Pint's own docs do.

```bash
go run ./examples/demo
go run ./examples/convert 20degC degF
go run ./examples/bench
go run ./examples/complete 'watt / h'
```

## `demo`

A terminal walkthrough of the library. Sections follow Pint's tutorial and user guides (`docs/getting/tutorial.rst`, `nonmult`, `log_units`, `contexts`, `systems`, `pitheorem`, and `pint-convert`).

```bash
go run ./examples/demo
go run ./examples/demo -only temp      # temperature / Dendra abs vs delta
go run ./examples/demo -only context   # spectroscopy nm → THz
go run ./examples/demo -only converter # Convert / ConvertN hot path
go run ./examples/demo -only complete  # ParseUnitName / Complete
```

Sections: `parse`, `arithmetic`, `convert`, `compact`, `format`, `errors`, `temp`, `log`, `context`, `systems`, `define`, `converter`, `complete`, `pi`.

## `convert`

Analog of Pint's `pint-convert` CLI (`docs/dev/pint-convert.rst`). Parse a quantity and print it in destination units, a named system, or SI.

```bash
go run ./examples/convert 225lb
go run ./examples/convert 102kg lb
go run ./examples/convert km mi
go run ./examples/convert 3.2kPa mbar
go run ./examples/convert 20degC degF
go run ./examples/convert --sys US 102kg
go run ./examples/convert 540nm kcal/mol
go run ./examples/convert 532nm THz
```

Like `pint-convert`, spectroscopy, energy, Boltzmann, Gaussian, and ESU contexts are on by default. `--context` adds others (`chem`, `textile`, …). The library registry still starts with no contexts enabled.

Mixed-unit sums such as `5 foot + 9 inch` are `Quantity.Add` in the demo, not a single parse string (Pint's CLI evaluates those as quantities; pint-go's expression parser adds only like units).

## `bench`

A quick throughput baseline. Pint has no equivalent CLI. One case is a single pair over a large slice (`ConvertN`); the other round-robins six pre-built converters.

```bash
go run ./examples/bench
go run ./examples/bench -n 2000000
```

Microbenchmarks live in the library: `go test -bench=. -benchmem .`

## `complete`

Prints `Registry.Complete` JSON for a partial unit string (canonical parse plus suggestions). Pint-go only; not in Python Pint.

```bash
go run ./examples/complete 'watt'
go run ./examples/complete 'kilopa'
go run ./examples/complete 'watt / h'
go run ./examples/complete -n 10 'meter  per se'
```

## `wasm`

Browser example wrapping Complete, ParseUnitName, Convert, Compatible, and Load. See [wasm/README.md](wasm/README.md). The JS helpers are example-only; the stable surface is the Go API.
