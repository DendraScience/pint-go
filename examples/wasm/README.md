# WASM example

Build pint-go for `GOOS=js GOARCH=wasm`. The stable API is the Go types; the
`pint*` JS functions are convenience wrappers for this example (and the Main App
unit-conversion playground).

```bash
./examples/wasm/build.sh
```

Writes `examples/wasm/pint.wasm` and copies `wasm_exec.js` from GOROOT. Both are gitignored.

JS (after `Go` from `wasm_exec.js` has run the module):

```js
JSON.parse(globalThis.pintComplete('watt / h', 20))
JSON.parse(globalThis.pintParse('degC'))
JSON.parse(globalThis.pintConvert(20, 'degC', 'degF'))
JSON.parse(globalThis.pintCompatible('degC', 'degF'))
JSON.parse(globalThis.pintConvert(532, 'nm', 'THz', JSON.stringify([{name:'sp'}])))
JSON.parse(globalThis.pintCompatible('nm', 'THz', JSON.stringify([{name:'sp', params:{n:{magnitude:1}}}])))
JSON.parse(globalThis.pintConvert(2, 'mole', 'kg', JSON.stringify([{name:'chem', params:{mw:{magnitude:18, unit:'g/mol'}}}])))
JSON.parse(globalThis.pintLoad('heartbeats = 1 / minute = hb\n'))
JSON.parse(globalThis.pintReset())
```

`pintComplete(partial, limit)` returns `CompleteResult` JSON.
`pintParse(name)` returns `UnitName` JSON or `{"error":"..."}`.
`pintConvert(value, src, dst[, scopeJSON])` returns `{value, src, dst, dimensionality}` or `{"error":"..."}`.
`pintCompatible(from, to[, scopeJSON])` returns `{ok}` or `{"error":"..."}`.
Optional `scopeJSON` is `[{name, params:{key:{magnitude, unit}}}]`. Omit `unit` (or pass `""`) for dimensionless params such as spectroscopy `n`. Omit the argument (or pass `""`) for the guest's `r.active`, which is empty unless the host enabled contexts. `pintComplete` does not take a scope.
`pintLoad(src)` rebuilds the registry from the default files plus extra Pint
definitions. On error the previous registry is kept. To load several extra
files, concatenate them in order (or `pintReset` then one combined `pintLoad`).
`pintReset()` rebuilds the default registry.
