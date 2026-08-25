# WASM example

Build `Registry.Complete` / `ParseUnitName` for `GOOS=js GOARCH=wasm`. The stable API is the Go types; `pintComplete` / `pintParse` are convenience wrappers for this example.

```bash
./examples/wasm/build.sh
```

Writes `examples/wasm/pint.wasm` and copies `wasm_exec.js` from GOROOT. Both are gitignored.

JS (after `Go` from `wasm_exec.js` has run the module):

```js
JSON.parse(globalThis.pintComplete('watt / h', 20))
JSON.parse(globalThis.pintParse('degC'))
```

`pintComplete(partial, limit)` returns `CompleteResult` JSON. `pintParse(name)` returns `UnitName` JSON or `{"error":"..."}`.
