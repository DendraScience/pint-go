# Complete: prefix noise when the slot already parses

**Status:** not implemented. Found while using the Main App unit picker (`nm` → `THz` with spectroscopy). Tasks: `dendra-api-services` `docs/FOLLOWUPS.md` § pint-go Complete typeahead. Do not keep a second task list here.

`Complete` is a pint-go extension (Python Pint has no equivalent). The Main App `PintUnitAutocomplete` shows `Parsed` as the first row and `Suggestions` under it. This note is about **what `Suggestions` contains**, not how the Vue control labels rows.

A separate UI bug (list title `nanometer (nm)` re-parsed as `nanometer ** 2` on blur) was fixed in `dendra-web-apps` `PintUnitAutocomplete.vue`. That does not change this catalog policy.

---

## Problem

Typing a **finished short symbol** that also looks like `prefix + unit start` dumps dozens of composed names. The unit you meant is only in `Parsed`.

`Complete("nm", 20)`:

| | |
| --- | --- |
| **Parsed** | `nanometer` (correct) |
| **Suggestions** | `nanomil`, `nanomile`, `nanomole`, `nanominim`, `nanomolar`, `nanometer_Hg`, `nanometer_H2O`, … |

Those rows are **nano + every unit whose name/alias/symbol starts with `m`**. They are not “more `nm` spellings.” `THz` / `terahertz` does not hit this (`Complete("THz")` is Parsed only).

Same shape for other one-letter SI prefixes glued to a one-letter unit start (`mm` is millimeter *and* milli+m…; check before changing policy).

---

## Why it happens

`collectCompleteCands` does two things:

1. Catalog keys that the slot **starts with**.
2. `longestPrefixAttach`: longest prefix **spelling** (`n` = nano) plus `minPrefixUnitPartial` (1) runes of a unit. That is how `kilopa` → `kilopascal` works without listing every `kilo*` for `kilo`.

`nm` **parses** as nanometer, so `Parsed` is set. The same fragment still matches (2): prefix `n`, rest `m`. `sameParsedUnit` drops suggestions that parse to `nanometer`, but `nanomile` is a different unit, so it stays.

`minPrefixUnitPartial = 1` is doing its job for `kilopa`. It is the wrong gate once the **whole slot already parses**.

---

## Suggested change

When the **current slot** (the last identifier, not a trailing `/` or `per`) already parses as a unit, **do not run prefix composition** for that slot.

Keep:

- `Parsed` as today.
- Catalog prefix-matches that are **strict extensions** (`watt` → `watt_hour`; `nanometer` → `nanometer_Hg`). Those keys start with the typed fragment as a catalog name/alias/symbol, not as `nano` + `meter_Hg`.
- Prefix composition for slots that do **not** parse: `kilopa`, `millim`, `kPa`.

Do not return an empty suggestion list just because `Parsed` is set — `watt` must still suggest `watt_hour`.

Concrete rule to try:

```text
if ParseUnitName(fragment) succeeds:
    skip longestPrefixAttach
else:
    longestPrefixAttach as today
```

`Complete("nm")` would then be Parsed `nanometer` plus any catalog key that **starts with** `nm` (if any), not `nano`+`mile`. Confirm `mm`, `km`, `ms`, `ns` by hand; those are the other short symbol / prefix-split collisions.

---

## Related: `ParseUnitName` symbol on glued prefixes

`ParseUnitName("nm")` today: `Name=nanometer`, `Symbol=nanometer` (not `nm`). The picker used to render `nanometer (nanometer)`. The UI now omits a symbol that equals the name.

Optional later: if the expression is a single synthesized prefixed unit, copy the short symbol (`nm`) from the prefix+unit catalog when present. Not required to fix the suggestion list. Do not invent a symbol for compounds (`meter / second`).

---

## Tests (when implementing)

Add next to `complete_test.go`. Existing prefix tests (`kilopa`, `millim`, `kPa`, bare `milli` does not compose) must stay green.

- `Complete("nm")`: `Parsed.Name == "nanometer"`; no `nanomile` / `nanomole` / `nanominim`.
- `Complete("THz")`: Parsed `terahertz`; suggestions empty or only true extensions (today: empty).
- `Complete("watt")`: still includes `watt_hour`.
- `Complete("kilopa")`: still includes `kilopascal`; `Parsed == nil`.
- `Complete("nanometer")`: `nanometer_Hg` still allowed (catalog prefix of the canonical name).

---

## Out of scope

- Context picker on `Complete` (still none; `Compatible` / `Convert` take `Scope`).
- Vue list chrome (name + distinct symbol; named quantity is FOLLOWUPS § pint-go Complete typeahead).
- Changing `minPrefixUnitPartial` globally to 2 — that would break `kPa` / `mm`-style composition for slots that do **not** already parse.
