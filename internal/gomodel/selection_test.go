package gomodel

import (
	"go/types"
	"strings"
	"testing"
)

func TestSelectedFieldsFollowGoVisibility(t *testing.T) {
	p, err := analyzeSource(t, `
type hidden struct { X []int; N int }
type Other struct { N string }
type Outer struct { hidden; Other; Own bool }
type Shadow struct { hidden; X string }
type Pointed struct { *hidden }
type Recursive struct { *Recursive; Value int }
type methodBase struct{}
func (methodBase) N() {}
type MethodShadow struct { hidden; methodBase }
type deep struct { hidden }
type shallower struct { X string }
type DepthWinner struct { deep; shallower }
type Mid struct { *hidden }
type Top struct { Mid }
type Holder struct { *Other }
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		fields map[string]bool
	}{
		{"Outer", map[string]bool{"X": false, "Other": true, "Own": true}},
		{"Shadow", map[string]bool{"X": true, "N": false}},
		{"Pointed", map[string]bool{"X": false, "N": false}},
		{"Recursive", map[string]bool{"Recursive": true, "Value": true}},
		{"MethodShadow", map[string]bool{"X": false}},
		{"DepthWinner", map[string]bool{"X": false, "N": false}},
		{"Mid", map[string]bool{"X": false, "N": false}},
		{"Top", map[string]bool{"X": false, "N": false, "Mid": true}},
		{"Holder", map[string]bool{"Other": true, "N": false}},
	} {
		got := named(p, tc.name).Selectable
		if len(got) != len(tc.fields) {
			t.Fatalf("%s: %+v", tc.name, got)
		}
		for _, f := range got {
			direct, ok := tc.fields[f.Name]
			if !ok || f.Direct != direct {
				t.Errorf("%s: unexpected %+v", tc.name, f)
			}
			if (tc.name == "Pointed" || tc.name == "Mid" || (tc.name == "Top" && f.Name != "Mid") || (tc.name == "Holder" && f.Name == "N")) && !f.Indirect {
				t.Errorf("embedded pointer not recorded: %+v", f)
			}
		}
	}
	for _, typ := range p.Types {
		for _, direct := range typ.Fields {
			found := false
			for _, selected := range typ.Selectable {
				if selected.Name == direct.Name && selected.Direct {
					found = true
				}
			}
			if !found {
				t.Errorf("direct field missing from selections: %s.%s", typ, direct.Name)
			}
		}
	}
	for _, f := range named(p, "DepthWinner").Selectable {
		if f.Name == "X" && (f.Type.Go != types.Typ[types.String] || len(f.Path) != 2) {
			t.Errorf("wrong depth winner: %+v", f)
		}
	}
	for _, f := range named(p, "Holder").Selectable {
		if f.Name == "Other" && f.Indirect {
			t.Errorf("selecting pointer field itself must not dereference: %+v", f)
		}
	}
	if p.Find(types.NewSlice(types.Typ[types.Int])) == nil {
		t.Fatal("hidden promoted field dependency omitted")
	}
	if named(p, "hidden") != nil {
		t.Fatal("hidden embedding unnecessarily exposed")
	}
}

func TestEnsureSharesTypeIdentity(t *testing.T) {
	p, err := analyzeSource(t, `type Box struct { N int }`)
	if err != nil {
		t.Fatal(err)
	}
	box := named(p, "Box")
	pointer, err := p.Ensure(types.NewPointer(box.Go), "bridge addr")
	if err != nil {
		t.Fatal(err)
	}
	again, err := p.Ensure(types.NewPointer(box.Go), "bridge addr")
	if err != nil || again != pointer || pointer.Elem != box || pointer.Origin != "bridge addr" {
		t.Fatalf("identity lost: %v", err)
	}
}

func TestUnsupportedPromotedFieldHasContext(t *testing.T) {
	for _, source := range []string{
		`type hidden struct { M complex128 }; type Outer struct { hidden }`,
		`type inner int; type hidden struct { M inner }; type Outer struct { hidden }`,
	} {
		_, err := analyzeSource(t, source)
		if err == nil || !strings.Contains(err.Error(), "selected field M") {
			t.Fatalf("missing promoted field context: %v", err)
		}
	}
}
