//go:build js && wasm

// Browser wrappers for Registry.Complete and ParseUnitName.
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/dendrascience/pint-go"
)

func main() {
	ureg, err := pint.NewRegistry()
	if err != nil {
		panic(err)
	}

	js.Global().Set("pintComplete", js.FuncOf(func(_ js.Value, args []js.Value) any {
		partial := ""
		limit := 0
		if len(args) > 0 {
			partial = args[0].String()
		}
		if len(args) > 1 && !args[1].IsUndefined() && !args[1].IsNull() {
			limit = args[1].Int()
		}
		b, err := json.Marshal(ureg.Complete(partial, limit))
		if err != nil {
			return `{"error":` + jsonError(err) + `}`
		}
		return string(b)
	}))

	js.Global().Set("pintParse", js.FuncOf(func(_ js.Value, args []js.Value) any {
		name := ""
		if len(args) > 0 {
			name = args[0].String()
		}
		un, err := ureg.ParseUnitName(name)
		if err != nil {
			return `{"error":` + jsonError(err) + `}`
		}
		b, err := json.Marshal(un)
		if err != nil {
			return `{"error":` + jsonError(err) + `}`
		}
		return string(b)
	}))

	if cb := js.Global().Get("__pintOnReady"); cb.Type() == js.TypeFunction {
		cb.Invoke()
	}

	select {}
}

func jsonError(err error) string {
	b, _ := json.Marshal(err.Error())
	return string(b)
}
