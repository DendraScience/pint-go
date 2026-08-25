package pint

import "testing"

func TestGroupSimple(t *testing.T) {
	r := NewEmptyRegistry()
	if err := r.Load(`
@group mygroup
meter = [length]
second = [time]
@end
`); err != nil {
		t.Fatal(err)
	}
	g, err := r.Group("mygroup")
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]bool{}
	for _, n := range g.Members() {
		m[n] = true
	}
	if !m["meter"] || !m["second"] {
		t.Fatalf("%v", g.Members())
	}
}

func TestGroupUsing(t *testing.T) {
	r := NewEmptyRegistry()
	g1 := r.NewGroup("group1")
	g1.AddUnits("second", "inch")
	g2 := r.NewGroup("group2")
	g2.AddUnits("meter")
	if err := g2.AddGroups(r, "group1"); err != nil {
		t.Fatal(err)
	}
	exp := g2.Expanded(r)
	if _, ok := exp["inch"]; !ok {
		t.Fatalf("%v", exp)
	}
}

func TestGroupCyclic(t *testing.T) {
	r := NewEmptyRegistry()
	g2 := r.NewGroup("g2")
	g3 := r.NewGroup("g3")
	if err := g2.AddGroups(r, "g3"); err != nil {
		t.Fatal(err)
	}
	if err := g3.AddGroups(r, "g2"); err == nil {
		t.Fatal("expected cycle")
	}
}

func TestSystemParse(t *testing.T) {
	r := NewEmptyRegistry()
	if err := r.Load(`
meter = [length]
@system mks
meter
@end
`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.System("mks"); err != nil {
		t.Fatal(err)
	}
}
