# Conversion profiles and service-scoped contexts

**Status:** library API is implemented. Product binding (datastream + query RPC): `dendra-api-services` `docs/query-engine/conversion-profiles.md`. Tasks: `dendra-api-services` `docs/FOLLOWUPS.md` § Conversion profiles. Do not keep a second task list here.

A **profile** in Dendra is the datastream’s conversion-context map. Querying the stream is the session. It is not a pint-go type. Do not add `RegisterProfile`. The engine copies that map into a `Scope` and passes it in. The query RPC does not override the map.

---

## Problem

Pint contexts are extra conversion edges between *different* dimensions (PSU ↔ g/kg, nm ↔ THz, mol ↔ g). They are **registered** at load and **off** until enabled. Python Pint’s `_convert` consults the active graph *before* the same-dimension factor.

`Converter` / `ConverterOp` compiles `(from, to)` once. Context hops use a **bound `Scope`**, not live `Registry.active`. `EnableContext` mutates `Registry.active` and is CLI-only.

Do **not** `NewRegistry` per request.

---

## Scope params are quantities

Python Pint types a context kwarg when you pass it (`mw=18 * ureg("g/mol")`). pint-go does the same. A bare float is not a molecular weight.

```go
type Param struct {
    Magnitude float64
    Unit      string // empty = dimensionless
}

type ContextUse struct {
    Name   string
    Params map[string]Param
}

type Scope []ContextUse
```

- `Unit` empty: dimensionless (spectroscopy `n`).
- Chemistry `mw` is `Param{18, "g/mol"}`, not `18`. Then `2 mol * (18 g/mol)` is `36 g`, and `Converter("mole", "kilogram", …)` is `0.036` — not `36`.
- `solvent_mass` / `volume` / `dS_A` work the same: the caller sets the unit. The `@context` header does not.
- Header defaults (`mw=0`) stay dimensionless numbers from the definition file. They are not usable chemistry. Missing or zero-divisor params fail at `Converter` / `Compatible` (`NonFiniteConversionError` when the hop is Inf/NaN; `DimensionalityError` when a dimensionless param does not produce the dest dimension).
- Unknown param keys are an error at bind.

`evalContextEq` substitutes params the same way as `value`: `(mag * units)`. Do **not** retag the hop result onto dest units. If the equation does not yield the hop’s destination dimension, that is an error.

`Fingerprint` includes each param’s magnitude **and** unit string. Unchanged if Dendra pins `unit_name` in toml and fills `Param.Unit` when building the `Scope`. `18 g/mol` and `0.018 kg/mol` still fingerprint differently; Dendra will not produce the second form.

```go
func (r *Registry) Converter(src, dst string, scope ...Scope) (*ConverterOp, error)
func (r *Registry) Compatible(from, to string, scope ...Scope) (bool, error)
func (r *Registry) Convert(value float64, src, dst string, scope ...Scope) (float64, error)
func (r *Registry) EnableContext(name string, params map[string]Param) error
```

Zero extra args = `r.active` (CLI). An explicit `Scope` (even empty) is **that list only** — it is not appended to `r.active`. `ConverterOp` stores the resolved `[]activeContext`. `Convert` does not read live `r.active`.

---

## Preserve Python Pint logic

Keep private `convert` as the Pint-shaped engine:

1. Same units → identity.
2. If a context path exists in the **provided** active list, apply hops (`applyTransform` / `evalContextEq`), then convert the result into `dst`.
3. Same dimension → factor / offset / log as today.

Order matches `ContextRegistry._convert` in Pint (`pint/facets/context/registry.py`): graph first, then `super()._convert`.

Do not replace the BFS with a one-off salinity formula. Multi-hop (`sp` + `energy` → nm → kcal/mol) must keep working.

---

## What is global (and what is not)

Global on the `Registry` after `Load`: unit/prefix/dimension maps, and the `@context` **graphs** (names, aliases, declared param keys, transform equations). Those are the catalog.

Not global: which contexts are in play, and the param quantities. That is the `Scope` argument.

Unknown context name on a `Scope` → error at compile. Do not invent `@profile` syntax in definition files.

Built-in contexts already in `default_en.txt` (names / aliases): `spectroscopy`/`sp` (`n`), `boltzmann`, `energy`, `chemistry`/`chem` (`mw`, `volume`, `solvent_mass`), `textile`, `Gaussian`/`Gau`, `ESU`/`esu`. Extra files (e.g. environmental `sal`) are `Load`ed into the same catalog.

---

## `Load` and comments

`parseDefinitionFile` strips `#` comments and skips blank / comment-only lines. Only real defs enter `r.contexts` / `r.units`. Do not store the raw source on the `Registry`.

`Load` takes definition **text**. The API process reads `api.toml` paths and passes the file bytes in. Path resolution is the service’s job.

---

## Enumerate contexts

```go
type ContextInfo struct {
    Name   string
    Params []string // declared @context(…) keys, sorted; not the header default values
}

func (r *Registry) Contexts() []ContextInfo
```

Do **not** treat header defaults (`mw=0`) as values the UI should prefill. List each context once by canonical `Name`. Aliases are not listed. Context and param **descriptions** and Dendra’s pinned param `unit_name` are Dendra toml, not Pint `@context` attributes — `Contexts()` does not return units.

---

## Autocomplete (`Complete`)

`Complete` fills the **current unit token**. It does not decide convertibility. **Do not** add context/profile selection to `Complete`.

What *does* need the scope:

- `Compatible` / `CompatibleUnits` — with `sal` on, `CompatibleUnits("PSU")` includes mass-fraction units.
- WASM: `pintComplete` unchanged. `pintCompatible` / `pintConvert` take optional scope JSON:

```text
[{ "name": "chem", "params": { "mw": { "magnitude": 18, "unit": "g/mol" } } }]
[{ "name": "sp", "params": { "n": { "magnitude": 1 } } }]
```

---

## Concurrency and freeze

After `NewRegistry` / `Load` / `Define`, the catalog is read-mostly. Those mutators are not concurrent with `Convert`. Do not call `EnableContext` on a registry that is serving `Converter` from many goroutines.

---

## Tests (pint-go)

- `EnableContext("sp")` + `Quantity.To` / `convert`: still works (Pint parity).
- `Converter("nm", "THz")` **without** scope or enable: error.
- `Converter("nm", "THz", Scope{{Name: "sp"}})`: succeeds; `Convert` matches `To`.
- `Converter("PSU", "g/kg")` after `Load` of a `sal` context: fails without scope; succeeds with `dS_A` as a mass-fraction quantity.
- `Compatible` and `Converter` agree for the same scope.
- Two ops compiled with different `dS_A` / `mw` do not share results. Fingerprints include param units (`18 g/mol` ≠ `0.018 kg/mol`).
- `2 mole` → `kilogram` with `mw=18 g/mol` is `0.036`. Same with `mw=0.018 kg/mol`. Dimensionless `mw=18` is an error.
- `EnableContext` on registry A must not change `Convert` on an op built with an explicit scope.
- Multi-hop: `sp` + `energy` + `boltzmann` via a multi-entry `Scope`.
- `chem` with `mw=0 g/mol`: `NonFiniteConversionError`. Header default `mw=0` (dimensionless) does not compile.
- `Complete("kilopa")` unchanged whether a scope is passed or not.
- `Contexts()` lists each context once by canonical name plus param keys; after `Load` of a `sal` file, `sal` appears. Comment-only lines do not create contexts.
- `Load` of a comment-heavy source does not grow unit/context maps except for the real defs.

---

## Out of scope (v1)

- Per-request `NewRegistry`.
- `@profile` in definition files.
- Inferring param units from the hop dimension (ambiguous: g/mol vs kg/mol).
- Auto-inferring which context to use from the unit pair alone.
- Changing Pint’s definition-file language.
