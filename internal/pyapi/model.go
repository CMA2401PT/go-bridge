// Package pyapi plans Python-facing names and signatures before any source is
// emitted. Every user-visible Python name is decided (and checked for
// collisions) here, so emitters only print what the plan contains.
package pyapi

import (
	"fmt"
	"go/types"
	"regexp"
	"strings"
	"unicode"

	"go-bridge/internal/gomodel"
)

// Type is the Python representation of one Go model type.
type Type struct {
	Go   *gomodel.Type
	Name string // Python type name, e.g. "int" or "str"
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

// Alias re-exports a Python type under an exported Go type name.
type Alias struct {
	Name string
	Type *Type
}

type Package struct {
	Go        *gomodel.Package
	Name      string // Python package name
	Types     []*Type
	Functions []*Function
	Aliases   []Alias
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9]*$`)

var keywords = func() map[string]bool {
	set := make(map[string]bool)
	for _, word := range strings.Fields("False None True and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield") {
		set[word] = true
	}
	return set
}()

// avoidKeyword appends "_" to Python keywords, following PEP 8.
func avoidKeyword(name string) string {
	if keywords[name] {
		return name + "_"
	}
	return name
}

// Snake handles acronym boundaries: HTTPServer -> http_server. Digits stay
// with their word, so Uint16 and Value2 do not acquire an extra separator.
// Python keywords receive a trailing underscore.
func Snake(name string) string {
	runes := []rune(name)
	var out strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 && runes[i-1] != '_' {
			afterLower := unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1])
			endsAcronym := unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if afterLower || endsAcronym {
				out.WriteByte('_')
			}
		}
		out.WriteRune(unicode.ToLower(r))
	}
	return avoidKeyword(out.String())
}

// reserved are package-level names the generated package itself defines.
var reserved = map[string]string{
	"GoPanic": "bridge exception",
	"_native": "native module",
	"__all__": "package exports",
}

// namespace records the origin of each claimed name, so a collision can
// report both declarations instead of silently renaming one of them.
type namespace map[string]string

// claim reserves name for origin. kind describes the name in diagnostics,
// such as "name" or "parameter".
func (n namespace) claim(kind, name, origin string) error {
	if previous, exists := n[name]; exists {
		return fmt.Errorf("Python %s %q collides: %s and %s", kind, name, previous, origin)
	}
	n[name] = origin
	return nil
}

// Plan currently accepts the scalar backend's subset. A broader Go model is
// never silently reduced to the declarations this backend happens to support.
func Plan(g *gomodel.Package, name string) (*Package, error) {
	if name == "" {
		name = "go_" + Snake(g.Name)
	}
	if !identifier.MatchString(name) || keywords[name] {
		return nil, fmt.Errorf("invalid Python package name %q", name)
	}
	p := &Package{Go: g, Name: name}
	mapped := make(map[*gomodel.Type]*Type)
	for _, t := range g.Types {
		python, err := basicName(t)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", g.Path, err)
		}
		pt := &Type{Go: t, Name: python}
		p.Types = append(p.Types, pt)
		mapped[t] = pt
	}

	names := make(namespace)
	for name, origin := range reserved {
		names[name] = origin
	}
	for _, d := range g.Declarations {
		name := avoidKeyword(d.Name)
		if err := names.claim("name", name, g.Path+"."+d.Name); err != nil {
			return nil, err
		}
		p.Aliases = append(p.Aliases, Alias{Name: name, Type: mapped[d.Type]})
	}
	for _, f := range g.Functions {
		pf, err := planFunction(f, mapped, g.Path+"."+f.Name)
		if err != nil {
			return nil, err
		}
		if err := names.claim("name", pf.Name, g.Path+"."+f.Name); err != nil {
			return nil, err
		}
		p.Functions = append(p.Functions, pf)
	}
	return p, nil
}

// basicName maps a basic Go type to its Python builtin. Other kinds are
// rejected until the backend can bind them.
func basicName(t *gomodel.Type) (string, error) {
	if t.Kind != gomodel.Basic {
		return "", fmt.Errorf("binding generation for %s is not implemented yet (current backend supports basic values only)", t)
	}
	info := t.Go.(*types.Basic).Info()
	switch {
	case info&types.IsBoolean != 0:
		return "bool", nil
	case info&types.IsInteger != 0:
		return "int", nil
	case info&types.IsFloat != 0:
		return "float", nil
	case info&types.IsString != 0:
		return "str", nil
	}
	return "", fmt.Errorf("no Python mapping for %s", t)
}

// planFunction names a function and its parameters. Unnamed and "_"
// parameters become positional names arg1, arg2, ... by their position.
func planFunction(f *gomodel.Function, mapped map[*gomodel.Type]*Type, where string) (*Function, error) {
	pf := &Function{Go: f, Name: Snake(f.Name)}
	params := make(namespace)
	for i, param := range f.Params {
		name := Snake(param.Name)
		if name == "" || name == "_" {
			name = fmt.Sprintf("arg%d", i+1)
		}
		if err := params.claim("parameter", name, param.Name); err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		pf.Params = append(pf.Params, Parameter{Name: name, Type: mapped[param.Type]})
	}
	for _, r := range f.Results {
		pf.Results = append(pf.Results, mapped[r.Type])
	}
	return pf, nil
}
