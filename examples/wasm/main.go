//go:build js && wasm

// Browser wrappers for Registry.Complete, ParseUnitName, Convert, Compatible,
// and Load. The stable API is the Go types; these JS helpers are convenience
// wrappers for the Main App playground.
//
// pintCompatible and pintConvert take an optional trailing JSON scope:
// [{name, params:{key:{magnitude, unit}}}]. pintComplete does not.
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/dendrascience/pint-go"
)

func main() {
	ureg, err := newRegistry("")
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
			return jsonErr(err)
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
			return jsonErr(err)
		}
		b, err := json.Marshal(un)
		if err != nil {
			return jsonErr(err)
		}
		return string(b)
	}))

	js.Global().Set("pintConvert", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 3 {
			return jsonErrMsg("convert requires value, from unit, and to unit")
		}
		value := args[0].Float()
		src := args[1].String()
		dst := args[2].String()
		scope, present, err := parseScopeArg(args, 3)
		if err != nil {
			return jsonErr(err)
		}
		var out float64
		if present {
			out, err = ureg.Convert(value, src, dst, scope)
		} else {
			out, err = ureg.Convert(value, src, dst)
		}
		if err != nil {
			return jsonErr(err)
		}
		dim := ""
		if d, derr := ureg.Dimensionality(src); derr == nil {
			dim = d.String()
		}
		b, err := json.Marshal(convertResult{
			Value:          out,
			Src:            src,
			Dst:            dst,
			Dimensionality: dim,
		})
		if err != nil {
			return jsonErr(err)
		}
		return string(b)
	}))

	js.Global().Set("pintCompatible", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 2 {
			return jsonErrMsg("compatible requires from and to units")
		}
		scope, present, err := parseScopeArg(args, 2)
		if err != nil {
			return jsonErr(err)
		}
		var ok bool
		if present {
			ok, err = ureg.Compatible(args[0].String(), args[1].String(), scope)
		} else {
			ok, err = ureg.Compatible(args[0].String(), args[1].String())
		}
		if err != nil {
			return jsonErr(err)
		}
		b, err := json.Marshal(compatibleResult{Ok: ok})
		if err != nil {
			return jsonErr(err)
		}
		return string(b)
	}))

	js.Global().Set("pintLoad", js.FuncOf(func(_ js.Value, args []js.Value) any {
		src := ""
		if len(args) > 0 {
			src = args[0].String()
		}
		next, err := newRegistry(src)
		if err != nil {
			return jsonErr(err)
		}
		ureg = next
		return `{"ok":true}`
	}))

	js.Global().Set("pintReset", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		next, err := newRegistry("")
		if err != nil {
			return jsonErr(err)
		}
		ureg = next
		return `{"ok":true}`
	}))

	if cb := js.Global().Get("__pintOnReady"); cb.Type() == js.TypeFunction {
		cb.Invoke()
	}

	select {}
}

type convertResult struct {
	Value          float64 `json:"value"`
	Src            string  `json:"src"`
	Dst            string  `json:"dst"`
	Dimensionality string  `json:"dimensionality,omitempty"`
}

type compatibleResult struct {
	Ok bool `json:"ok"`
}

func newRegistry(extra string) (*pint.Registry, error) {
	r, err := pint.NewRegistry()
	if err != nil {
		return nil, err
	}
	if extra == "" {
		return r, nil
	}
	if err := r.Load(extra); err != nil {
		return nil, err
	}
	return r, nil
}

func jsonErr(err error) string {
	return jsonErrMsg(err.Error())
}

func jsonErrMsg(msg string) string {
	b, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{Error: msg})
	return string(b)
}

type jsParam struct {
	Magnitude float64 `json:"magnitude"`
	Unit      string  `json:"unit"`
}

type jsContextUse struct {
	Name   string             `json:"name"`
	Params map[string]jsParam `json:"params"`
}

func parseScopeArg(args []js.Value, idx int) (pint.Scope, bool, error) {
	if idx >= len(args) || args[idx].IsUndefined() || args[idx].IsNull() {
		return nil, false, nil
	}
	raw := args[idx].String()
	if raw == "" {
		return nil, false, nil
	}
	var uses []jsContextUse
	if err := json.Unmarshal([]byte(raw), &uses); err != nil {
		return nil, true, err
	}
	scope := make(pint.Scope, len(uses))
	for i, u := range uses {
		params := make(map[string]pint.Param, len(u.Params))
		for k, p := range u.Params {
			params[k] = pint.Param{Magnitude: p.Magnitude, Unit: p.Unit}
		}
		scope[i] = pint.ContextUse{Name: u.Name, Params: params}
	}
	return scope, true, nil
}
