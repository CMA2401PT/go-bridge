package pyapi

import (
	"strings"
	"testing"
)

func TestAnyConversionPlan(t *testing.T) {
	plan, err := planSource(t, `
type Value = any
type Count int
type Box struct { Data any; AsAny int }
func (v Count) FromAny() int { return int(v) }
func Echo(value interface{}, pointer *any, values []any) any { return value }
`)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Any == nil || plan.Any.Shape != AnyObject || plan.Any.Name != "GoAny" || len(plan.AnyConversions) != len(plan.Types)-1 {
		t.Fatal("missing any type or concrete conversion plans")
	}
	if plan.Functions[0].Params[0].Type != plan.Any || plan.Functions[0].Results[0] != plan.Any {
		t.Fatal("any signature does not use GoAny")
	}
	for _, conversion := range plan.AnyConversions {
		if conversion.Type == plan.Any {
			t.Fatal("GoAny received concrete conversions")
		}
		if conversion.Type.Shape == Builtin {
			stem := conversion.Type.Go.Name
			if conversion.Box != stem+"_as_any" || conversion.Assert != "any_as_"+stem || conversion.TryAssert != "try_any_as_"+stem {
				t.Fatal("scalar conversion names do not identify Go types")
			}
		} else if conversion.Box != "as_any" || conversion.Assert != "from_any" || conversion.TryAssert != "try_from_any" {
			t.Fatal("incorrect concrete conversion members")
		}
	}
	for _, typ := range plan.Types {
		if typ.Name == "Box" {
			for _, field := range typ.Fields {
				if field.Go.Name == "AsAny" && field.Name != "as_any_" {
					t.Fatal("conversion member did not reserve the field spelling")
				}
			}
		}
		if typ.Name == "Count" && typ.Methods[0].Name != "from_any_" {
			t.Fatal("conversion member did not reserve the method spelling")
		}
	}
}

func TestAnyConversionConflicts(t *testing.T) {
	for _, source := range []string{
		`type GoAny int; func Echo(v any) any { return v }`,
		`func IntAsAny() {}; func Echo(v any) any { return v }`,
		`func AnyAsString() {}; func Echo(v any) any { return v }`,
		`func TryAnyAsFloat32() {}; func Echo(v any) any { return v }`,
	} {
		plan, err := planSource(t, source)
		if err == nil || plan != nil || !strings.Contains(err.Error(), "collides") {
			t.Fatalf("missing conversion conflict for %s: %v", source, err)
		}
	}
	plan, err := planSource(t, `type GoAny int; func IntAsAny() {}`)
	if err != nil || plan.Any != nil || len(plan.AnyConversions) != 0 {
		t.Fatalf("any bridge enabled without an empty interface: %v", err)
	}
}
