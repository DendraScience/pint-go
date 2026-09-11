package pint

import (
	"slices"
	"strings"
	"testing"
)

const salDefs = `
# comment-only lines must not enter the catalog
# @context fake
# fake_unit = [fake]

PSU = [psu]
@context(dS_A=0) sal
    [psu] -> [mass] / [mass]: value / PSU * gram / kilogram + dS_A
    [mass] / [mass] -> [psu]: (value - dS_A) / (gram / kilogram) * PSU
@end
`

func loadSal(t *testing.T, r *Registry) {
	t.Helper()
	if err := r.Load(salDefs); err != nil {
		t.Fatal(err)
	}
}

func chemMW(mag float64, unit string) Scope {
	return Scope{{Name: "chem", Params: map[string]Param{"mw": {Magnitude: mag, Unit: unit}}}}
}

func salDSA(mag float64, unit string) Scope {
	return Scope{{Name: "sal", Params: map[string]Param{"dS_A": {Magnitude: mag, Unit: unit}}}}
}

func TestConverterContextRequiresScope(t *testing.T) {
	r := mustReg(t)
	_, err := r.Converter("nm", "THz")
	if err == nil {
		t.Fatal("nm -> THz without scope or enable must fail")
	}
	if _, ok := err.(*DimensionalityError); !ok {
		t.Fatalf("got %T %v", err, err)
	}
}

func TestConverterScopeSpectroscopy(t *testing.T) {
	r := mustReg(t)
	if err := r.EnableContext("sp", nil); err != nil {
		t.Fatal(err)
	}
	q, err := r.Quantity(532, "nm")
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.To("terahertz")
	if err != nil {
		t.Fatal(err)
	}
	r.DisableContext()

	op, err := r.Converter("nm", "THz", Scope{{Name: "sp"}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := op.Convert(532)
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, got.Magnitude(), 1e-9)
}

func TestConverterScopeSalinity(t *testing.T) {
	r := mustReg(t)
	loadSal(t, r)
	_, err := r.Converter("PSU", "g/kg")
	if err == nil {
		t.Fatal("PSU -> g/kg without sal must fail")
	}
	scope := salDSA(0, "gram/kilogram")
	v, err := r.Convert(35, "PSU", "g/kg", scope)
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, 35, 1e-9)
	shifted, err := r.Convert(35, "PSU", "g/kg", salDSA(2, "gram/kilogram"))
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, shifted, 37, 1e-9)
}

func TestCompatibleAgreesWithConverter(t *testing.T) {
	r := mustReg(t)
	ok, err := r.Compatible("nm", "THz")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("nm/THz without scope must be incompatible")
	}
	_, err = r.Converter("nm", "THz")
	if err == nil {
		t.Fatal("expected Converter error")
	}

	sp := Scope{{Name: "sp"}}
	ok, err = r.Compatible("nm", "THz", sp)
	if err != nil || !ok {
		t.Fatalf("nm/THz with sp: %v %v", ok, err)
	}
	if _, err := r.Converter("nm", "THz", sp); err != nil {
		t.Fatal(err)
	}
}

func TestConverterOpsDoNotShareParams(t *testing.T) {
	r := mustReg(t)
	op18, err := r.Converter("gram", "mole", chemMW(18, "g/mol"))
	if err != nil {
		t.Fatal(err)
	}
	op32, err := r.Converter("gram", "mole", chemMW(32, "g/mol"))
	if err != nil {
		t.Fatal(err)
	}
	v18, err := op18.Convert(18)
	if err != nil {
		t.Fatal(err)
	}
	v32, err := op32.Convert(32)
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v18, 1, 1e-9)
	assertClose(t, v32, 1, 1e-9)
	s18 := chemMW(18, "g/mol")
	s32 := chemMW(32, "g/mol")
	if s18.Fingerprint() == s32.Fingerprint() {
		t.Fatal("fingerprints for different mw must differ")
	}
}

func TestConverterScopeIgnoresEnableContext(t *testing.T) {
	r := mustReg(t)
	op, err := r.Converter("nm", "THz", Scope{{Name: "sp"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.EnableContext("chem", map[string]Param{"mw": {Magnitude: 18, Unit: "g/mol"}}); err != nil {
		t.Fatal(err)
	}
	v, err := op.Convert(532)
	if err != nil {
		t.Fatal(err)
	}
	if v < 500 || v > 630 {
		t.Fatalf("532 nm -> THz = %g after EnableContext(chem)", v)
	}
}

func TestConverterExplicitEmptyScopeIgnoresActive(t *testing.T) {
	r := mustReg(t)
	if err := r.EnableContext("sp", nil); err != nil {
		t.Fatal(err)
	}
	_, err := r.Converter("nm", "THz", Scope{})
	if err == nil {
		t.Fatal("explicit empty Scope must not use r.active")
	}
	if _, err := r.Converter("nm", "THz"); err != nil {
		t.Fatal(err)
	}
}

func TestConverterScopeMultiHop(t *testing.T) {
	r := mustReg(t)
	scope := Scope{
		{Name: "Gau"},
		{Name: "ESU"},
		{Name: "sp"},
		{Name: "energy"},
		{Name: "boltzmann"},
	}
	v, err := r.Convert(540, "nm", "kcal/mol", scope)
	if err != nil {
		t.Fatal(err)
	}
	if v < 52 || v > 54 {
		t.Fatalf("540 nm -> kcal/mol = %g", v)
	}
}

func TestConverterChemDimensionlessMWFails(t *testing.T) {
	r := mustReg(t)
	_, err := r.Converter("gram", "mole", chemMW(18, ""))
	if err == nil {
		t.Fatal("chem mw without units must not compile")
	}
	if _, ok := err.(*DimensionalityError); !ok {
		t.Fatalf("got %T %v, want DimensionalityError", err, err)
	}
}

func TestConverterChemMWUnits(t *testing.T) {
	r := mustReg(t)
	scope := chemMW(18, "g/mol")
	v, err := r.Convert(2, "mole", "kilogram", scope)
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, v, 0.036, 1e-12)

	kg, err := r.Convert(2, "mole", "kilogram", chemMW(0.018, "kg/mol"))
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, kg, 0.036, 1e-12)

	g, err := r.Convert(2, "mole", "gram", scope)
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, g, 36, 1e-12)

	if chemMW(18, "g/mol").Fingerprint() == chemMW(0.018, "kg/mol").Fingerprint() {
		t.Fatal("fingerprints must include param units")
	}
}

func TestConverterChemEnableContextWithUnits(t *testing.T) {
	r := mustReg(t)
	if err := r.EnableContext("chem", map[string]Param{"mw": {Magnitude: 18, Unit: "g/mol"}}); err != nil {
		t.Fatal(err)
	}
	q, err := r.Quantity(2, "mole")
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.To("kilogram")
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, got.Magnitude(), 0.036, 1e-12)
}

func TestConverterUnknownParam(t *testing.T) {
	r := mustReg(t)
	_, err := r.Converter("gram", "mole", Scope{{
		Name:   "chem",
		Params: map[string]Param{"nope": {Magnitude: 1, Unit: "g/mol"}},
	}})
	if err == nil {
		t.Fatal("expected unknown parameter")
	}
	if !strings.Contains(err.Error(), "unknown context parameter") {
		t.Fatalf("got %v", err)
	}
}

func TestConverterChemZeroMWFailsClosed(t *testing.T) {
	r := mustReg(t)
	_, err := r.Converter("gram", "mole", Scope{{Name: "chem"}})
	if err == nil {
		t.Fatal("chem with header mw=0 must not compile")
	}

	_, err = r.Converter("gram", "mole", chemMW(0, "g/mol"))
	if err == nil {
		t.Fatal("chem mw=0 g/mol must not compile")
	}
	if _, ok := err.(*NonFiniteConversionError); !ok {
		t.Fatalf("got %T %v, want NonFiniteConversionError", err, err)
	}
	ok, err := r.Compatible("gram", "mole", chemMW(0, "g/mol"))
	if err == nil {
		t.Fatal("Compatible must fail closed with chem mw=0 g/mol")
	}
	if ok {
		t.Fatal("Compatible must not report true on a non-finite hop")
	}
}

func TestCompleteUnchangedByScope(t *testing.T) {
	r := mustReg(t)
	before := r.Complete("kilopa", 20)
	if _, err := r.Converter("nm", "THz", Scope{{Name: "sp"}}); err != nil {
		t.Fatal(err)
	}
	after := r.Complete("kilopa", 20)
	if before.TotalCount != after.TotalCount {
		t.Fatalf("Complete total %d vs %d", before.TotalCount, after.TotalCount)
	}
	if !completeHasText(after, "kilopascal") {
		t.Fatalf("suggestions %v missing kilopascal", suggestionTexts(after))
	}
}

func TestContextsListsCanonicalNames(t *testing.T) {
	r := mustReg(t)
	got := r.Contexts()
	if len(got) == 0 {
		t.Fatal("expected built-in contexts")
	}
	names := make([]string, 0, len(got))
	var chem ContextInfo
	for _, info := range got {
		names = append(names, info.Name)
		if info.Name == "chemistry" {
			chem = info
		}
	}
	if slices.Contains(names, "sp") || slices.Contains(names, "chem") || slices.Contains(names, "Gau") || slices.Contains(names, "esu") {
		t.Fatalf("aliases listed: %v", names)
	}
	if !slices.Contains(names, "chemistry") || !slices.Contains(names, "spectroscopy") {
		t.Fatalf("missing canonical names: %v", names)
	}
	if !slices.Contains(chem.Params, "mw") || !slices.Contains(chem.Params, "volume") || !slices.Contains(chem.Params, "solvent_mass") {
		t.Fatalf("chemistry params %v", chem.Params)
	}
	if !slices.IsSorted(names) {
		t.Fatalf("Contexts not sorted: %v", names)
	}

	loadSal(t, r)
	after := r.Contexts()
	var sawSal bool
	for _, info := range after {
		if info.Name == "sal" {
			sawSal = true
			if !slices.Equal(info.Params, []string{"dS_A"}) {
				t.Fatalf("sal params %v", info.Params)
			}
		}
		if info.Name == "fake" {
			t.Fatal("comment @context fake must not load")
		}
	}
	if !sawSal {
		t.Fatal("Load sal must appear in Contexts")
	}
}

func TestLoadSkipsComments(t *testing.T) {
	r := mustReg(t)
	nUnits := len(r.unitByName)
	nCtx := len(r.Contexts())
	nDim := len(r.dimensions)
	if err := r.Load(`
# only comments
# @context fake
# fake_unit = [fake]
`); err != nil {
		t.Fatal(err)
	}
	if len(r.unitByName) != nUnits || len(r.Contexts()) != nCtx || len(r.dimensions) != nDim {
		t.Fatal("comment-only Load grew the catalog")
	}

	loadSal(t, r)
	if !r.HasUnit("PSU") {
		t.Fatal("PSU missing")
	}
	if r.HasUnit("fake_unit") {
		t.Fatal("comment unit must not load")
	}
	if _, err := r.ContextByName("sal"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ContextByName("fake"); err == nil {
		t.Fatal("comment context must not load")
	}
	if len(r.Contexts()) != nCtx+1 {
		t.Fatalf("Contexts grew by %d, want 1", len(r.Contexts())-nCtx)
	}
}

func TestCompatibleUnitsUsesScope(t *testing.T) {
	r := mustReg(t)
	loadSal(t, r)
	ok, err := r.Compatible("PSU", "g/kg")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("PSU/g/kg without sal must be incompatible")
	}
	scope := salDSA(35, "gram/kilogram")
	ok, err = r.Compatible("PSU", "g/kg", scope)
	if err != nil || !ok {
		t.Fatalf("PSU/g/kg with sal: %v %v", ok, err)
	}
	plain, err := r.CompatibleUnits("PSU", "")
	if err != nil {
		t.Fatal(err)
	}
	scoped, err := r.CompatibleUnits("PSU", "", scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) <= len(plain) {
		t.Fatalf("sal should add compatible units: plain %d scoped %d", len(plain), len(scoped))
	}
}

func TestConverterUnknownContext(t *testing.T) {
	r := mustReg(t)
	_, err := r.Converter("nm", "THz", Scope{{Name: "nope"}})
	if err == nil {
		t.Fatal("expected unknown context")
	}
	if !strings.Contains(err.Error(), "unknown context") {
		t.Fatalf("got %v", err)
	}
}

func TestNonFiniteConversionErrorMessage(t *testing.T) {
	err := &NonFiniteConversionError{From: "gram", To: "mole"}
	got := err.Error()
	want := "conversion gram -> mole is not finite for the bound context parameters"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
