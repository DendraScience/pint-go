<p align="center">
  <img src="docs/logo.jpg" alt="pint-go" width="280">
</p>

# pint-go

A pure-Go unit library with [Pint](https://github.com/hgrecco/pint)’s definition files and conversion semantics. No CGO.

Pint is the **behavior and definition contract**, not a line-by-line Python clone. This package does not copy operator overloading, NumPy, Dask, Babel, pickle, or uncertainties. Magnitudes are `float64`.

## Install

```bash
go get github.com/dendrascience/pint-go
```

## Usage

```go
ureg, err := pint.NewRegistry() // embeds default_en.txt + constants_en.txt
q, err := ureg.Parse("3.2 kPa")
q2, err := q.To("mbar")

ok, err := ureg.Compatible("degC", "degF")

c, err := ureg.Converter("degC", "degF") // parse once
f, err := c.Convert(20)                  // 68
err = c.ConvertN(dst, src)               // []float64, no per-sample parse

d, err := ureg.Converter("delta_degC", "delta_degF")
```

`Registry` is safe for concurrent reads after `NewRegistry`. `Define` / `Load` are not concurrent with convert.

## Examples

```bash
go run ./examples/demo                 # tutorial-style walkthrough
go run ./examples/convert 20degC degF  # pint-convert analog
go run ./examples/bench                # ConvertN / mixed-pair throughput
```

See [examples/README.md](examples/README.md).

### Absolute vs delta temperature

```text
20 degC → degF        = 68     (absolute)
5  delta_degC → delta_degF = 9  (interval)
```

Offset units (`degC`, `degF`) synthesize `delta_*` companions, matching Pint’s offset calculus. Logarithmic units (dB, octave, …) and definition-file contexts/groups/systems are supported.

## Dendra notes

This library is meant for query-path conversion (including WASM later). Hot path: `Converter` + `ConvertN` over `[]float64`. Do not parse unit strings per sample.

## Staying in sync with Pint

Definitions change more often than the engine. Vendored files are copied **verbatim**.

1. Pin a Pint git SHA in `PINT_SHA` (currently `6fc0533`, ~0.26 unreleased).
2. `go run ./tools/syncpint --pint /path/to/pint` copies `default_en.txt` and `constants_en.txt`, rebuilds `testdata/inventory.yml`, and prints new/removed tests.
3. Upgrade playbook: bump SHA → `syncpint` → fix parser/engine until core tests pass → classify any new tests in the inventory.

Do not git-submodule the whole Pint tree into consumers; only the two `.txt` files plus SHA.

`testdata/inventory.yml` lists every Pint `test_*` as `ported` or `skipped` with a reason. `go test ./tools/syncpint` fails if a sibling Pint checkout (or `PINT_DIR`) has tests missing from the inventory.

Skipped Python-ecosystem suites (NumPy, Dask, Matplotlib, Babel, pickle, Decimal, …) are named, not silent. Pint’s own xfails for three compound log-arithmetic cases are documented expected failures.

## License

BSD-3-Clause. Vendored definition files retain the Pint copyright; see `LICENSE` and `LICENSE.pint`.
