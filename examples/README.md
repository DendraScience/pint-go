# Examples

Runnable programs that show pint-go the way Pint's own docs do.

```bash
go run ./examples/demo
go run ./examples/convert 20degC degF
go run ./examples/bench
```

## `demo`

A terminal walkthrough of the library. Sections follow Pint's tutorial and user guides (`docs/getting/tutorial.rst`, `nonmult`, `log_units`, `contexts`, `systems`, `pitheorem`, and `pint-convert`).

```bash
go run ./examples/demo
go run ./examples/demo -only temp      # temperature / Dendra abs vs delta
go run ./examples/demo -only context   # spectroscopy nm → THz
go run ./examples/demo -only converter # Convert / ConvertN hot path
```

Sections: `parse`, `arithmetic`, `convert`, `compact`, `format`, `errors`, `temp`, `log`, `context`, `systems`, `define`, `converter`, `pi`.

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
