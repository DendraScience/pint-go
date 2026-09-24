package pint

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// DefaultCompleteLimit is used when Complete's limit argument is <= 0.
	DefaultCompleteLimit = 20

	// CompletionKindUnit is Completion.Kind when the last token matched a canonical name.
	CompletionKindUnit = "unit"
	// CompletionKindAlias is Completion.Kind when the last token matched an alias.
	CompletionKindAlias = "alias"
	// CompletionKindSymbol is Completion.Kind when the last token matched a symbol.
	CompletionKindSymbol = "symbol"
)

// Matching policy for Complete (tune these, not one-off prefix names):
//
//  1. Fill the current unit slot (the last identifier). Trailing operator
//     runes and infix words in completeInfixOperatorWords are an empty slot.
//  2. Catalog keys (name, alias, symbol) that the slot starts with.
//  3. Prefix+unit compositions the parser would accept, without getName.
//     Attach only the longest matching prefix spelling (pico, not pebi's Pi).
//     Require minPrefixUnitPartial runes of the unit after that spelling so
//     "milli"/"centi"/"kilo" do not enumerate every milli* unit. "millim"
//     and "kilopa" do compose. Short symbols still work: "kPa", "mm".
//  4. If the glued suggestion parses, emit the canonical full expression
//     (catalog name when the product is a defined unit). Drop suggestions
//     that parse to the same unit as Parsed.

// minPrefixUnitPartial is how much of the unit is required after an SI/binary
// prefix spelling. 0 lists every milli* row for "milli". 1 waits for millim,
// centim, kilop, …
const minPrefixUnitPartial = 1

// completeInfixOperatorWords are last-token words treated like "/".
var completeInfixOperatorWords = []string{"per"}

// UnitName is a parsed unit expression with canonical name and display fields.
// JSON names are part of the API.
type UnitName struct {
	Name           string   `json:"name"`
	Symbol         string   `json:"symbol,omitempty"`
	Dimensionality string   `json:"dimensionality,omitempty"`
	Aliases        []string `json:"aliases,omitempty"`
}

// Completion is one selectable full-expression suggestion for [Registry.Complete].
// Text is what to store or insert. Kind is CompletionKindUnit, CompletionKindAlias,
// or CompletionKindSymbol. Canonical is the parsed unit name when the glued
// expression parses; otherwise it is the last-token registry name.
// JSON names are part of the API.
type Completion struct {
	Text      string `json:"text"`
	Kind      string `json:"kind"`
	Canonical string `json:"canonical"`
	Symbol    string `json:"symbol,omitempty"`
}

// CompleteResult is the result of [Registry.Complete]. Suggestions is never nil.
// JSON names are part of the API.
type CompleteResult struct {
	Parsed      *UnitName    `json:"parsed,omitempty"`
	Suggestions []Completion `json:"suggestions"`
	TotalCount  int          `json:"total_count"`
	Limit       int          `json:"limit"`
}

// ParseUnitName returns the canonical unit expression for s, plus display
// fields (symbol, dimensionality, aliases). Aliases such as degC become
// degree_Celsius. A compound that is exactly a catalog definition
// (mile / hour, mile per hour) becomes that catalog name (mile_per_hour).
// Empty or unparseable input returns an error.
//
// ParseUnitName is a pint-go extension; Python Pint has no equivalent.
// A successful parse may synthesize a prefixed unit into the registry
// (same as [Registry.ParseUnits]). Persist the returned Name.
func (r *Registry) ParseUnitName(s string) (UnitName, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return UnitName{}, fmt.Errorf("empty unit name")
	}
	u, err := r.ParseUnits(s)
	if err != nil {
		return UnitName{}, err
	}
	name := u.String()
	dim, err := r.dimensionality(u)
	if err != nil {
		return UnitName{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	if u.Len() >= 2 {
		if n := r.unitByExpr[name]; n != "" {
			name = n
			u = unitPair(n, 1)
		}
	}
	out := UnitName{Name: name, Dimensionality: dim.String()}
	if u.Len() == 1 {
		n := u.Names()[0]
		if u.Get(n) == 1 {
			if def := r.unitByName[n]; def != nil {
				if def.symbol != "" && def.symbol != "_" {
					out.Symbol = def.symbol
				}
				if len(def.aliases) > 0 {
					out.Aliases = append([]string(nil), def.aliases...)
				}
			}
		}
	}
	if out.Symbol == "" {
		compact := r.formatUnits(u, true)
		if compact != "" && compact != "dimensionless" {
			out.Symbol = compact
		}
	}
	return out, nil
}

type completeCand struct {
	name, symbol, kind string
	rank, remain       int
}

// Complete returns autocomplete rows for a partial unit expression.
// It is a pint-go extension; Python Pint has no equivalent.
//
// If partial parses as a whole, Parsed is that canonical unit. Suggestions
// are full expressions that replace the current unit slot (the last
// identifier), not next-token fragments. A trailing operator (/ * or infix
// "per") is an empty slot until the next unit is started. limit <= 0 uses
// [DefaultCompleteLimit]. Complete does not return an error: invalid input
// yields Parsed == nil and may yield no suggestions.
//
// Matching may compose SI/binary prefixes with units (kilopa → kilopascal)
// without writing those names during the catalog scan. A successful parse of
// the input or of a glued suggestion may synthesize a prefixed unit, same as
// [Registry.ParseUnits].
func (r *Registry) Complete(partial string, limit int) CompleteResult {
	if limit <= 0 {
		limit = DefaultCompleteLimit
	}
	out := CompleteResult{Limit: limit, Suggestions: []Completion{}}
	if un, err := r.ParseUnitName(partial); err == nil {
		cp := un
		out.Parsed = &cp
	}

	prefix, fragment := splitCompleteSlot(partial)

	r.mu.RLock()
	best := r.collectCompleteCands(fragment)
	r.mu.RUnlock()

	if p := out.Parsed; p != nil {
		if c, ok := best[p.Name]; ok {
			c.rank = 0
			best[p.Name] = c
		}
	}

	cands := make([]completeCand, 0, len(best))
	for _, c := range best {
		cands = append(cands, c)
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].rank != cands[j].rank {
			return cands[i].rank < cands[j].rank
		}
		if cands[i].remain != cands[j].remain {
			return cands[i].remain < cands[j].remain
		}
		return cands[i].name < cands[j].name
	})

	parsedName := ""
	if out.Parsed != nil {
		parsedName = out.Parsed.Name
	}
	out.TotalCount = len(cands)
	for _, c := range cands {
		text := joinCompletePrefix(prefix, c.name)
		if sameParsedUnit(r, parsedName, prefix, text) {
			out.TotalCount--
			continue
		}
		if len(out.Suggestions) >= limit {
			continue
		}
		out.Suggestions = append(out.Suggestions, suggestionFromJoin(r, prefix, text, c.name, c.kind, c.symbol))
	}
	return out
}

// collectCompleteCands matches the current unit slot. Caller holds RLock.
// Policy: catalog prefix-match; then longest prefix spelling + unit partial
// (minPrefixUnitPartial); no getName.
func (r *Registry) collectCompleteCands(fragment string) map[string]completeCand {
	best := map[string]completeCand{}
	if fragment == "" {
		return best
	}
	fragLower := strings.ToLower(fragment)
	add := func(name, symbol, kind string, rank int) {
		remain := len(name) - len(fragment)
		if remain < 0 {
			remain = 0
		}
		prev, ok := best[name]
		if ok {
			if prev.symbol == "" && symbol != "" {
				prev.symbol = symbol
				best[name] = prev
			}
			if prev.rank < rank || (prev.rank == rank && prev.remain <= remain) {
				return
			}
		}
		best[name] = completeCand{name: name, symbol: symbol, kind: kind, rank: rank, remain: remain}
	}

	for key, def := range r.units {
		if !strings.HasPrefix(strings.ToLower(key), fragLower) {
			continue
		}
		kind, rank := completeKeyKind(key, def)
		add(def.name, cleanSymbol(def.symbol), kind, rank)
	}

	if p, rest, ok := r.longestPrefixAttach(fragment); ok {
		restLower := strings.ToLower(rest)
		for key, def := range r.units {
			if !def.isMultiplicative() {
				continue
			}
			if strings.HasPrefix(strings.ToLower(def.name), strings.ToLower(p.name)) {
				continue
			}
			if !strings.HasPrefix(strings.ToLower(key), restLower) {
				continue
			}
			kind, rank := completeKeyKind(key, def)
			add(p.name+def.name, composedSymbol(p, def), kind, rank)
		}
	}
	return best
}

// longestPrefixAttach picks one prefix spelling to glue onto the slot.
// Longest spelling wins so "pico" is pico-, not pebi- (Pi) plus "co".
func (r *Registry) longestPrefixAttach(fragment string) (prefixDef, string, bool) {
	fragLower := strings.ToLower(fragment)
	bestLen := 0
	var best prefixDef
	var bestSp string
	for _, p := range r.prefixList {
		if p.name == "" {
			continue
		}
		for _, sp := range prefixSpellings(p) {
			if sp == "" {
				continue
			}
			if !strings.HasPrefix(fragLower, strings.ToLower(sp)) {
				continue
			}
			if len(sp) <= bestLen {
				continue
			}
			bestLen = len(sp)
			best = p
			bestSp = sp
		}
	}
	if bestLen == 0 {
		return prefixDef{}, "", false
	}
	rest := fragment[len(bestSp):]
	if utf8.RuneCountInString(rest) < minPrefixUnitPartial {
		return prefixDef{}, "", false
	}
	return best, rest, true
}

func completeKeyKind(key string, def *unitDef) (kind string, rank int) {
	if key == def.name {
		return CompletionKindUnit, 1
	}
	if key == def.symbol {
		return CompletionKindSymbol, 2
	}
	return CompletionKindAlias, 2
}

func prefixSpellings(p prefixDef) []string {
	out := make([]string, 0, 2+len(p.aliases))
	out = append(out, p.name, p.symbol)
	out = append(out, p.aliases...)
	return out
}

func composedSymbol(p prefixDef, def *unitDef) string {
	ps, us := cleanSymbol(p.symbol), cleanSymbol(def.symbol)
	if ps == "" || us == "" {
		return ""
	}
	return ps + us
}

func cleanSymbol(s string) string {
	if s == "" || s == "_" {
		return ""
	}
	return s
}

func splitCompleteSlot(s string) (before, fragment string) {
	runes := []rune(s)
	i := len(runes)
	for i > 0 && unicode.IsSpace(runes[i-1]) {
		i--
	}
	if i == 0 {
		return s, ""
	}
	if isOperatorRune(runes[i-1]) {
		return string(runes[:i]), ""
	}
	identEnd := i
	for i > 0 && isIdentPart(runes[i-1]) {
		i--
	}
	if i < identEnd && isIdentStart(runes[i]) {
		ident := string(runes[i:identEnd])
		before := string(runes[:i])
		if isInfixOperatorWord(ident) && strings.TrimSpace(before) != "" {
			return string(runes[:identEnd]), ""
		}
		return before, ident
	}
	return s, ""
}

func isInfixOperatorWord(s string) bool {
	for _, w := range completeInfixOperatorWords {
		if strings.EqualFold(s, w) {
			return true
		}
	}
	return false
}

func joinCompletePrefix(prefix, name string) string {
	prefix = strings.TrimRight(prefix, " \t")
	if prefix == "" {
		return name
	}
	return prefix + " " + name
}

// sameParsedUnit reports whether a last-token suggestion is the same unit as
// the already-parsed input. Cheap string match covers canonical rebuilds
// (watt → watt). Re-parse covers aliases and preprocessor forms
// ("cm per hour" → centimeter / hour) without dropping strict extensions
// (watt → watt_hour).
func sameParsedUnit(r *Registry, parsedName, prefix, text string) bool {
	if parsedName == "" {
		return false
	}
	if text == parsedName {
		return true
	}
	if prefix == "" {
		return false
	}
	un, err := r.ParseUnitName(text)
	return err == nil && un.Name == parsedName
}

// suggestionFromJoin rebuilds a last-token completion. When the glued
// expression parses (e.g. "meter  per second"), Text/Canonical/Symbol are
// the full unit, not the last token ("second").
func suggestionFromJoin(r *Registry, prefix, text, name, kind, symbol string) Completion {
	s := Completion{Text: text, Kind: kind, Canonical: name, Symbol: symbol}
	if prefix == "" {
		return s
	}
	un, err := r.ParseUnitName(text)
	if err != nil {
		return s
	}
	s.Text = un.Name
	s.Canonical = un.Name
	s.Symbol = un.Symbol
	return s
}
