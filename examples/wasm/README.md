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
JSON.parse(globalThis.pintLoad('heartbeats = 1 / minute = hb\n'))
JSON.parse(globalThis.pintReset())
```

`pintComplete(partial, limit)` returns `CompleteResult` JSON.
`pintParse(name)` returns `UnitName` JSON or `{"error":"..."}`.
`pintConvert(value, src, dst)` returns `{value, src, dst, dimensionality}` or `{"error":"..."}`.
`pintCompatible(from, to)` returns `{ok}` or `{"error":"..."}`.
`pintLoad(src)` rebuilds the registry from the default files plus extra Pint
definitions. On error the previous registry is kept.
`pintReset()` rebuilds the default registry.
