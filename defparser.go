package pint

import (
	"fmt"
	"strings"
	"unicode"
)

type defKind int

const (
	defComment defKind = iota
	defDefaults
	defImport
	defPrefix
	defUnit
	defDimension
	defDerivedDimension
	defAlias
	defGroup
	defContext
	defSystem
)

type parsedDef struct {
	kind     defKind
	name     string
	symbol   string
	aliases  []string
	value    string // raw RHS for unit/prefix/dimension
	mods     map[string]float64
	using    []string
	filename string // @import
	// context
	defaults map[string]float64
	rels     []contextRel
	redefs   []parsedDef
	// group
	members []parsedDef
	// system
	rules []systemRule
}

type contextRel struct {
	src, dst string
	equation string
	bidirect bool
}

type systemRule struct {
	newUnit string
	oldUnit string // empty if omitted
}

func splitComment(line string) (code, comment string) {
	in := line
	if i := strings.IndexByte(in, '#'); i >= 0 {
		return strings.TrimSpace(in[:i]), strings.TrimSpace(in[i+1:])
	}
	return strings.TrimSpace(in), ""
}

func splitEq(s string) []string {
	parts := strings.Split(s, "=")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

func parseSymbolAliases(parts []string) (symbol string, aliases []string) {
	if len(parts) == 0 {
		return "", nil
	}
	if parts[0] == "_" {
		parts = parts[1:]
	} else {
		symbol = parts[0]
		parts = parts[1:]
	}
	for _, a := range parts {
		if a == "" || a == "_" {
			continue
		}
		aliases = append(aliases, a)
	}
	return symbol, aliases
}

func parseModifiers(rest string) (map[string]float64, error) {
	mods := map[string]float64{}
	if strings.TrimSpace(rest) == "" {
		return mods, nil
	}
	for _, part := range strings.Split(rest, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("invalid modifier %q", part)
		}
		n, err := parseNumeric(strings.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("modifier %s: %w", strings.TrimSpace(k), err)
		}
		mods[strings.TrimSpace(k)] = n
	}
	return mods, nil
}

func parseDefinitionLine(line string) (*parsedDef, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	if strings.HasPrefix(line, "@alias ") {
		parts := splitEq(strings.TrimPrefix(line, "@alias "))
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid @alias: %s", line)
		}
		return &parsedDef{kind: defAlias, name: parts[0], aliases: parts[1:]}, nil
	}
	if strings.HasPrefix(line, "@import ") {
		return &parsedDef{kind: defImport, filename: strings.TrimSpace(strings.TrimPrefix(line, "@import "))}, nil
	}
	parts := splitEq(line)
	if len(parts) == 1 {
		name := parts[0]
		if strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]") {
			return &parsedDef{kind: defDimension, name: name}, nil
		}
		return nil, fmt.Errorf("invalid definition: %s", line)
	}
	name := parts[0]
	if strings.HasSuffix(name, "-") {
		pname := strings.TrimSuffix(name, "-")
		val := parts[1]
		symbol, aliases := parseSymbolAliases(parts[2:])
		for i, a := range aliases {
			aliases[i] = strings.TrimSuffix(a, "-")
		}
		symbol = strings.TrimSuffix(symbol, "-")
		n, err := parseNumeric(val)
		if err != nil {
			return nil, fmt.Errorf("prefix %s: %w", pname, err)
		}
		return &parsedDef{kind: defPrefix, name: pname, symbol: symbol, aliases: aliases, value: fmt.Sprintf("%g", n), mods: map[string]float64{"scale": n}}, nil
	}
	if strings.HasPrefix(name, "[") {
		if len(parts) > 2 {
			return nil, fmt.Errorf("derived dimensions cannot have aliases")
		}
		return &parsedDef{kind: defDerivedDimension, name: strings.TrimSpace(name), value: parts[1]}, nil
	}
	rhs := parts[1]
	value, modStr := rhs, ""
	if i := strings.IndexByte(rhs, ';'); i >= 0 {
		value = strings.TrimSpace(rhs[:i])
		modStr = strings.TrimSpace(rhs[i+1:])
	}
	mods, err := parseModifiers(modStr)
	if err != nil {
		return nil, err
	}
	symbol, aliases := parseSymbolAliases(parts[2:])
	return &parsedDef{kind: defUnit, name: name, symbol: symbol, aliases: aliases, value: value, mods: mods}, nil
}

type blockKind int

const (
	blockNone blockKind = iota
	blockDefaults
	blockGroup
	blockContext
	blockSystem
)

func parseDefinitionFile(src, origin string) ([]parsedDef, error) {
	lines := strings.Split(src, "\n")
	var out []parsedDef
	kind := blockNone
	var header parsedDef
	var body []string

	flush := func() error {
		if kind == blockNone {
			return nil
		}
		d, err := finishBlock(kind, header, body)
		if err != nil {
			return err
		}
		if d != nil {
			out = append(out, *d)
		}
		kind = blockNone
		header = parsedDef{}
		body = nil
		return nil
	}

	for i, raw := range lines {
		code, _ := splitComment(raw)
		if code == "" {
			continue
		}
		if code == "@end" {
			if err := flush(); err != nil {
				return nil, fmt.Errorf("%s:%d: %w", origin, i+1, err)
			}
			continue
		}
		if strings.HasPrefix(code, "@defaults") {
			if err := flush(); err != nil {
				return nil, err
			}
			kind = blockDefaults
			header = parsedDef{kind: defDefaults}
			continue
		}
		if strings.HasPrefix(code, "@group") {
			if err := flush(); err != nil {
				return nil, err
			}
			h, err := parseGroupHeader(code)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", origin, i+1, err)
			}
			kind = blockGroup
			header = *h
			continue
		}
		if strings.HasPrefix(code, "@context") {
			if err := flush(); err != nil {
				return nil, err
			}
			h, err := parseContextHeader(code)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", origin, i+1, err)
			}
			kind = blockContext
			header = *h
			continue
		}
		if strings.HasPrefix(code, "@system") {
			if err := flush(); err != nil {
				return nil, err
			}
			h, err := parseSystemHeader(code)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", origin, i+1, err)
			}
			kind = blockSystem
			header = *h
			continue
		}
		if kind != blockNone {
			body = append(body, code)
			continue
		}
		d, err := parseDefinitionLine(code)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", origin, i+1, err)
		}
		if d != nil {
			out = append(out, *d)
		}
	}
	if kind != blockNone {
		if err := flush(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func finishBlock(kind blockKind, header parsedDef, body []string) (*parsedDef, error) {
	switch kind {
	case blockDefaults:
		header.defaults = map[string]float64{}
		header.mods = map[string]float64{}
		kv := map[string]string{}
		for _, line := range body {
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				return nil, fmt.Errorf("invalid defaults line %q", line)
			}
			kv[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
		header.value = kv["group"]
		if sys, ok := kv["system"]; ok {
			header.symbol = sys // reuse symbol field for system name
		}
		return &header, nil
	case blockGroup:
		for _, line := range body {
			d, err := parseDefinitionLine(line)
			if err != nil {
				return nil, err
			}
			if d != nil && d.kind == defUnit {
				header.members = append(header.members, *d)
			}
		}
		return &header, nil
	case blockContext:
		for _, line := range body {
			if strings.Contains(line, "<->") || strings.Contains(line, "->") {
				rel, err := parseRelation(line)
				if err != nil {
					return nil, err
				}
				header.rels = append(header.rels, rel)
				continue
			}
			d, err := parseDefinitionLine(line)
			if err != nil {
				return nil, err
			}
			if d != nil && d.kind == defUnit {
				header.redefs = append(header.redefs, *d)
			}
		}
		return &header, nil
	case blockSystem:
		for _, line := range body {
			if strings.Contains(line, ":") {
				a, b, _ := strings.Cut(line, ":")
				header.rules = append(header.rules, systemRule{newUnit: strings.TrimSpace(a), oldUnit: strings.TrimSpace(b)})
			} else {
				header.rules = append(header.rules, systemRule{newUnit: strings.TrimSpace(line)})
			}
		}
		return &header, nil
	}
	return nil, nil
}

func parseGroupHeader(s string) (*parsedDef, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(s, "@group"))
	name, using := rest, []string{}
	if i := strings.Index(rest, " using "); i >= 0 {
		name = strings.TrimSpace(rest[:i])
		for _, g := range strings.Split(rest[i+len(" using "):], ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				using = append(using, g)
			}
		}
	}
	if name == "" {
		return nil, fmt.Errorf("invalid group header %q", s)
	}
	return &parsedDef{kind: defGroup, name: name, using: using}, nil
}

func parseSystemHeader(s string) (*parsedDef, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(s, "@system"))
	name, using := rest, []string{}
	if i := strings.Index(rest, " using "); i >= 0 {
		name = strings.TrimSpace(rest[:i])
		for _, g := range strings.Split(rest[i+len(" using "):], ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				using = append(using, g)
			}
		}
	} else {
		using = []string{"root"}
	}
	if name == "" {
		return nil, fmt.Errorf("invalid system header %q", s)
	}
	return &parsedDef{kind: defSystem, name: name, using: using}, nil
}

func parseContextHeader(s string) (*parsedDef, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(s, "@context"))
	defaults := map[string]float64{}
	if strings.HasPrefix(rest, "(") {
		end := strings.IndexByte(rest, ')')
		if end < 0 {
			return nil, fmt.Errorf("invalid context header %q", s)
		}
		inner := rest[1:end]
		rest = strings.TrimSpace(rest[end+1:])
		if inner != "" {
			for _, part := range strings.Split(inner, ",") {
				k, v, ok := strings.Cut(part, "=")
				if !ok {
					return nil, fmt.Errorf("invalid context default %q", part)
				}
				n, err := parseNumeric(strings.TrimSpace(v))
				if err != nil {
					return nil, err
				}
				defaults[strings.TrimSpace(k)] = n
			}
		}
	}
	parts := splitEq(rest)
	name := parts[0]
	var aliases []string
	if len(parts) > 1 {
		aliases = parts[1:]
	}
	if name == "" {
		return nil, fmt.Errorf("invalid context header %q", s)
	}
	return &parsedDef{kind: defContext, name: name, aliases: aliases, defaults: defaults}, nil
}

func parseRelation(s string) (contextRel, error) {
	rel, eq, ok := strings.Cut(s, ":")
	if !ok {
		return contextRel{}, fmt.Errorf("invalid relation %q", s)
	}
	eq = strings.TrimSpace(eq)
	rel = strings.TrimSpace(rel)
	if strings.Contains(rel, "<->") {
		a, b, _ := strings.Cut(rel, "<->")
		return contextRel{src: strings.TrimSpace(a), dst: strings.TrimSpace(b), equation: eq, bidirect: true}, nil
	}
	if strings.Contains(rel, "->") {
		a, b, _ := strings.Cut(rel, "->")
		return contextRel{src: strings.TrimSpace(a), dst: strings.TrimSpace(b), equation: eq, bidirect: false}, nil
	}
	return contextRel{}, fmt.Errorf("invalid relation %q", s)
}

func isPythonIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
