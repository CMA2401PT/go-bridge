// Package pyapi plans Python-facing names and signatures before any source is emitted.
package pyapi

import (
	"fmt"
	"go/types"
	"regexp"
	"strings"
	"unicode"

	"go-bridge/internal/gomodel"
)

type Type struct {
	Go   *gomodel.Type
	Name string
}

type Parameter struct {
	Name string
	Type *Type
}
type Function struct {
	Go      *gomodel.Function
	Name    string
	Params  []Parameter
	Results []*Type
}
type Alias struct {
	Name string
	Type *Type
}
type Package struct {
	Go        *gomodel.Package
	Name      string
	Types     []*Type
	Functions []*Function
	Aliases   []Alias
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9]*$`)
var keywords = strings.Fields("False None True and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield")

func keyword(s string) bool {
	for _, word := range keywords {
		if word == s {
			return true
		}
	}
	return false
}

// Snake handles acronym boundaries: HTTPServer -> http_server. Digits stay
// with their word, so Uint16 and Value2 do not acquire an extra separator.
func Snake(name string) string {
	runes := []rune(name)
	var out strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 && runes[i-1] != '_' {
			previousLower := unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1])
			endAcronym := unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if previousLower || endAcronym {
				out.WriteByte('_')
			}
		}
		out.WriteRune(unicode.ToLower(r))
	}
	result := out.String()
	if keyword(result) {
		result += "_"
	}
	return result
}

// Plan currently accepts the scalar backend's subset. A broader Go model is
// never silently reduced to the declarations this backend happens to support.
func Plan(g *gomodel.Package, name string) (*Package, error) {
	if name == "" {
		name = "go_" + Snake(g.Name)
	}
	if !identifier.MatchString(name) || keyword(name) {
		return nil, fmt.Errorf("invalid Python package name %q", name)
	}
	p := &Package{Go: g, Name: name}
	mapped := make(map[*gomodel.Type]*Type)
	for _, t := range g.Types {
		if t.Kind != gomodel.Basic {
			return nil, fmt.Errorf("%s: binding generation for %s is not implemented yet (current backend supports basic values only)", g.Path, t)
		}
		basic := t.Go.(*types.Basic)
		var python string
		switch {
		case basic.Info()&types.IsBoolean != 0:
			python = "bool"
		case basic.Info()&types.IsInteger != 0:
			python = "int"
		case basic.Info()&types.IsFloat != 0:
			python = "float"
		case basic.Info()&types.IsString != 0:
			python = "str"
		default:
			return nil, fmt.Errorf("no Python mapping for %s", t)
		}
		pt := &Type{Go: t, Name: python}
		p.Types = append(p.Types, pt)
		mapped[t] = pt
	}
	names := map[string]string{"GoPanic": "bridge exception", "_native": "native module", "__all__": "package exports"}
	claim := func(name, origin string) error {
		if previous, exists := names[name]; exists {
			return fmt.Errorf("Python name %q collides: %s and %s", name, previous, origin)
		}
		names[name] = origin
		return nil
	}
	for _, d := range g.Declarations {
		name := d.Name
		if keyword(name) {
			name += "_"
		}
		if err := claim(name, g.Path+"."+d.Name); err != nil {
			return nil, err
		}
		p.Aliases = append(p.Aliases, Alias{Name: name, Type: mapped[d.Type]})
	}
	for _, f := range g.Functions {
		pf := &Function{Go: f, Name: Snake(f.Name)}
		if err := claim(pf.Name, g.Path+"."+f.Name); err != nil {
			return nil, err
		}
		params := make(map[string]string)
		for i, param := range f.Params {
			name := Snake(param.Name)
			if name == "" || name == "_" {
				name = fmt.Sprintf("arg%d", i+1)
			}
			if previous, exists := params[name]; exists {
				return nil, fmt.Errorf("%s.%s: Python parameter %q collides: %s and %s", g.Path, f.Name, name, previous, param.Name)
			}
			params[name] = param.Name
			pf.Params = append(pf.Params, Parameter{Name: name, Type: mapped[param.Type]})
		}
		for _, r := range f.Results {
			pf.Results = append(pf.Results, mapped[r.Type])
		}
		p.Functions = append(p.Functions, pf)
	}
	return p, nil
}
