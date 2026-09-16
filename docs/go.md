# pint-go notes

This package implements the Pint model in [how-it-works.md](how-it-works.md). Install and usage snippets are in the [README](../README.md).

## Registry and numbers

`NewRegistry` loads the embedded `default_en.txt` and `constants_en.txt`. Magnitudes are `float64`. There is no CGO.

After `NewRegistry`, many goroutines may convert at once. Do not call `Define`, `Load`, or `EnableContext` while other goroutines are converting. Do not construct a new registry per request.

Extra definition text goes through `Load` (whole files, including `@context` blocks) or `Define` (single lines). `@context` is added with `Load`; `Define` does not take those blocks. `syncpint` copies the two bundled files from upstream Pint and rebuilds the test inventory. Add product units by loading extras, not by hand-editing the bundled files.

## Calls that match Pint

`Parse`, `Quantity.To`, `Compatible`, and `Convert` follow Pint. Temperature `delta_*` companions exist here as they do there.

`Converter` compiles a `(from, to)` pair once. `ConvertN` runs many magnitudes through that pair. Services that convert slices of samples should build the converter once and reuse it.

## pint-go extensions

Python Pint has no equivalent of these two.

`ParseUnitName("degC")` returns `degree_Celsius`, plus symbol, dimensionality, and aliases. Persist that canonical string.

`Complete` is typeahead for a unit picker. It fills the current unit slot: `kilopa` becomes `kilopascal`, `watt / h` becomes `watt / hour`. If the whole input already parses, that unit is `Parsed`. Other rows are `Suggestions`. JSON field names on those types are part of the API.

When a finished short symbol also looks like a prefix plus a unit start (`nm` suggesting `nanomile`), that is catalog policy inside `Complete`. It is not a Vue labeling bug. See [complete-prefix-noise.md](complete-prefix-noise.md).

## Context scope

Pint registers `@context` graphs at load and leaves them off until enabled. Python enables them on the registry, or in a `with` block.

pint-go services pass a **scope** into `Converter`, `Compatible`, and `Convert`. A scope is an ordered list of context names and parameter quantities (`Param{Magnitude, Unit}`). Empty `Unit` means dimensionless (spectroscopy `n`). Chemistry `mw` is `Param{18, "g/mol"}`, not `18`.

If you omit the scope argument, the call uses a snapshot of `Registry.active`. That path is for the CLI. An explicit scope is exactly that list. It is not appended to `active`.

Do not call `EnableContext` on a registry that many goroutines are converting through. `@context` blocks load through `Load`. `Contexts()` lists catalog rows (name plus param keys) for a UI picker.

Fingerprints and the Dendra binding are in [conversion-profiles.md](conversion-profiles.md).

## Examples

```bash
go run ./examples/demo                 # walkthrough
go run ./examples/convert 3atm kPa     # pint-convert analog
go run ./examples/complete 'kilopa'    # Complete JSON
go run ./examples/bench                # ConvertN throughput
```

The convert CLI enables spectroscopy, energy, Boltzmann, Gaussian, and ESU on startup, matching Pint's `pint-convert`. The library registry still starts with no contexts enabled. `--sys US` reduces through a named system. Conversion still walks definitions.

Browser wrappers are in [examples/wasm/README.md](../examples/wasm/README.md).
