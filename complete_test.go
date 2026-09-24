package pint

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseUnitNameWatt(t *testing.T) {
	r := mustReg(t)
	un, err := r.ParseUnitName("watt")
	if err != nil {
		t.Fatal(err)
	}
	if un.Name != "watt" {
		t.Fatalf("Name=%q want watt", un.Name)
	}
	if un.Symbol != "W" {
		t.Fatalf("Symbol=%q want W", un.Symbol)
	}
}

func TestParseUnitNameKilowattSymbol(t *testing.T) {
	r := mustReg(t)
	un, err := r.ParseUnitName("kilowatt")
	if err != nil {
		t.Fatal(err)
	}
	if un.Name != "kilowatt" {
		t.Fatalf("Name=%q want kilowatt", un.Name)
	}
	if un.Symbol != "kW" {
		t.Fatalf("Symbol=%q want kW", un.Symbol)
	}
}

func TestParseUnitNameDegC(t *testing.T) {
	r := mustReg(t)
	un, err := r.ParseUnitName("degC")
	if err != nil {
		t.Fatal(err)
	}
	if un.Name != "degree_Celsius" {
		t.Fatalf("Name=%q want degree_Celsius", un.Name)
	}
}

func TestParseUnitNamePrefersCatalogCompound(t *testing.T) {
	r := mustReg(t)
	cases := []struct{ in, want string }{
		{"mph", "mile_per_hour"},
		{"mile per hour", "mile_per_hour"},
		{"mile / hour", "mile_per_hour"},
		{"kph", "kilometer_per_hour"},
		{"kilometer / hour", "kilometer_per_hour"},
		{"meter / second", "meter_per_second"},
		{"joule / second", "watt"},
		{"centimeter per hour", "centimeter / hour"},
		{"cm / hour", "centimeter / hour"},
	}
	for _, tc := range cases {
		un, err := r.ParseUnitName(tc.in)
		if err != nil {
			t.Errorf("%q: %v", tc.in, err)
			continue
		}
		if un.Name != tc.want {
			t.Errorf("%q Name=%q want %q", tc.in, un.Name, tc.want)
		}
	}

	for _, q := range []string{"mph", "mile per hour", "mile / hour"} {
		got := r.Complete(q, 20)
		if got.Parsed == nil || got.Parsed.Name != "mile_per_hour" {
			t.Errorf("Complete(%q) Parsed=%v want mile_per_hour", q, got.Parsed)
		}
		for _, s := range got.Suggestions {
			if s.Text == q || s.Text == "mile / hour" {
				t.Errorf("Complete(%q) duplicate suggestion %q", q, s.Text)
			}
		}
	}
}

func TestCompleteWatt(t *testing.T) {
	r := mustReg(t)
	got := r.Complete("watt", 20)
	if got.Parsed == nil || got.Parsed.Name != "watt" {
		t.Fatalf("Parsed=%v want watt", got.Parsed)
	}
	if !completeHasText(got, "watt_hour") {
		t.Fatalf("suggestions %v missing watt_hour", suggestionTexts(got))
	}
}

func TestCompleteDegC(t *testing.T) {
	r := mustReg(t)
	got := r.Complete("degC", 20)
	if got.Parsed == nil || got.Parsed.Name != "degree_Celsius" {
		t.Fatalf("Parsed=%v want degree_Celsius", got.Parsed)
	}
}

func TestCompleteWattSlash(t *testing.T) {
	r := mustReg(t)
	got := r.Complete("watt /", 20)
	if got.Parsed != nil {
		t.Fatalf("Parsed=%v want nil", got.Parsed)
	}
	if len(got.Suggestions) != 0 {
		t.Fatalf("suggestions %v want none until the next unit is started", suggestionTexts(got))
	}
}

func TestCompleteWattSlashH(t *testing.T) {
	r := mustReg(t)
	got := r.Complete("watt / h", 20)
	if !completeHasText(got, "watt / hour") {
		t.Fatalf("suggestions %v missing watt / hour", suggestionTexts(got))
	}
}

func TestCompletePrefixedPartial(t *testing.T) {
	r := mustReg(t)
	got := r.Complete("kilopa", 20)
	if got.Parsed != nil {
		t.Fatalf("Parsed=%v want nil", got.Parsed)
	}
	if !completeHasText(got, "kilopascal") {
		t.Fatalf("suggestions %v missing kilopascal", suggestionTexts(got))
	}

	got = r.Complete("millim", 20)
	if !completeHasText(got, "millimeter") {
		t.Fatalf("suggestions %v missing millimeter", suggestionTexts(got))
	}

	got = r.Complete("centim", 20)
	if !completeHasText(got, "centimeter") {
		t.Fatalf("suggestions %v missing centimeter", suggestionTexts(got))
	}

	got = r.Complete("kPa", 20)
	if !completeHasText(got, "kilopascal") {
		t.Fatalf("suggestions %v missing kilopascal", suggestionTexts(got))
	}
}

func TestCompleteKilowaSuggestionsParse(t *testing.T) {
	r := mustReg(t)
	got := r.Complete("kilowa", 20)
	if !completeHasText(got, "kilowatt") {
		t.Fatalf("suggestions %v missing kilowatt", suggestionTexts(got))
	}
	for _, s := range got.Suggestions {
		if _, err := r.ParseUnitName(s.Text); err != nil {
			t.Errorf("suggestion %q does not parse: %v", s.Text, err)
		}
	}
}

func TestCompleteKilowaKeepsSymbolAfterParse(t *testing.T) {
	r := mustReg(t)
	before := suggestionByText(r.Complete("kilowa", 20), "kilowatt")
	if before == nil {
		t.Fatal("kilowatt missing before parse")
	}
	if before.Symbol != "kW" {
		t.Fatalf("before parse Symbol=%q want kW", before.Symbol)
	}
	if _, err := r.ParseUnitName("kilowatt"); err != nil {
		t.Fatal(err)
	}
	after := suggestionByText(r.Complete("kilowa", 20), "kilowatt")
	if after == nil {
		t.Fatal("kilowatt missing after parse")
	}
	if after.Symbol != "kW" {
		t.Fatalf("after parse Symbol=%q want kW", after.Symbol)
	}
}

func TestCompleteBarePrefixDoesNotCompose(t *testing.T) {
	r := mustReg(t)
	for _, q := range []string{"milli", "centi", "kilo", "mega", "micro", "nano", "pico"} {
		got := r.Complete(q, 20)
		for _, s := range got.Suggestions {
			if strings.HasPrefix(s.Text, "pebi") || strings.HasPrefix(s.Text, "kibi") {
				t.Fatalf("%q suggested %q from a shorter prefix spelling", q, s.Text)
			}
			if !completeCatalogHasPrefix(r, s.Text, q) {
				t.Fatalf("%q composed %q; a bare prefix only catalog-matches (minPrefixUnitPartial)", q, s.Text)
			}
		}
	}
}

func completeCatalogHasPrefix(r *Registry, name, frag string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fragLower := strings.ToLower(frag)
	for key, def := range r.units {
		if def.name == name && strings.HasPrefix(strings.ToLower(key), fragLower) {
			return true
		}
	}
	return false
}

func TestCompleteTrailingPerWaitsForUnit(t *testing.T) {
	r := mustReg(t)
	got := r.Complete("meter per", 20)
	for _, s := range got.Suggestions {
		if s.Text == "meter * percent" || strings.Contains(s.Text, "percent") {
			t.Fatalf("per treated as unit prefix: %v", suggestionTexts(got))
		}
	}
	if len(got.Suggestions) != 0 {
		t.Fatalf("suggestions %v want none until the next unit is started", suggestionTexts(got))
	}
}

func TestCompleteCanonicalizesCompound(t *testing.T) {
	r := mustReg(t)
	got := r.Complete("meter  per se", 20)
	if got.Parsed != nil {
		t.Fatalf("Parsed=%v want nil", got.Parsed)
	}
	if !completeHasText(got, "meter_per_second") {
		t.Fatalf("suggestions %v missing meter_per_second", suggestionTexts(got))
	}
	for _, s := range got.Suggestions {
		if strings.Contains(s.Text, " per ") {
			t.Fatalf("suggestion %q still uses per", s.Text)
		}
		if s.Text == "meter_per_second" && s.Canonical != "meter_per_second" {
			t.Fatalf("Canonical=%q want meter_per_second", s.Canonical)
		}
	}
}

func TestCompleteDropsSameParsedExpression(t *testing.T) {
	r := mustReg(t)
	for _, q := range []string{"cm per hour", "cm / hour"} {
		got := r.Complete(q, 20)
		if got.Parsed == nil || got.Parsed.Name != "centimeter / hour" {
			t.Fatalf("%q Parsed=%v want centimeter / hour", q, got.Parsed)
		}
		for _, s := range got.Suggestions {
			if s.Text == q || s.Canonical == "hour" {
				t.Fatalf("%q duplicate suggestion %v; parsed already covers this unit", q, s)
			}
			un, err := r.ParseUnitName(s.Text)
			if err == nil && un.Name == got.Parsed.Name {
				t.Fatalf("%q suggestion %q parses to the same unit as Parsed", q, s.Text)
			}
		}
	}
}

func TestCompleteDoesNotSynthesizePrefixes(t *testing.T) {
	r := mustReg(t)
	r.mu.RLock()
	before := make(map[string]struct{}, len(r.unitByName))
	for k := range r.unitByName {
		before[k] = struct{}{}
	}
	r.mu.RUnlock()

	_ = r.Complete("milli", 20)
	_ = r.Complete("xyzzy", 20)
	_ = r.Complete("kilopa", 20)

	r.mu.RLock()
	defer r.mu.RUnlock()
	for k := range r.unitByName {
		if _, ok := before[k]; !ok {
			t.Fatalf("Complete added unitByName key %q", k)
		}
	}
}

func TestCompleteDefaultLimit(t *testing.T) {
	r := mustReg(t)
	got := r.Complete("watt /", 0)
	if got.Limit != DefaultCompleteLimit {
		t.Fatalf("Limit=%d want %d", got.Limit, DefaultCompleteLimit)
	}
}

func completeHasText(got CompleteResult, text string) bool {
	if got.Parsed != nil && got.Parsed.Name == text {
		return true
	}
	return suggestionByText(got, text) != nil
}

func suggestionByText(got CompleteResult, text string) *Completion {
	for i := range got.Suggestions {
		if got.Suggestions[i].Text == text {
			return &got.Suggestions[i]
		}
	}
	return nil
}

func suggestionTexts(got CompleteResult) []string {
	out := make([]string, len(got.Suggestions))
	for i, s := range got.Suggestions {
		out[i] = s.Text
	}
	return out
}

func ExampleRegistry_ParseUnitName() {
	ureg, err := NewRegistry()
	if err != nil {
		panic(err)
	}
	un, err := ureg.ParseUnitName("degC")
	if err != nil {
		panic(err)
	}
	fmt.Println(un.Name)
	// Output: degree_Celsius
}

func ExampleRegistry_Complete() {
	ureg, err := NewRegistry()
	if err != nil {
		panic(err)
	}
	got := ureg.Complete("watt / h", 0)
	fmt.Println(got.Parsed.Name)
	fmt.Println(got.Limit)
	// Output:
	// watt / hour
	// 20
}
