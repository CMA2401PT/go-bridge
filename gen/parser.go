package gen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

func ParseGoFile(filename string) (*PackageIR, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return nil, err
	}

	pkg := &PackageIR{
		TargetPackage: file.Name.Name,
		TargetImport:  fmt.Sprintf("path/to/%s", file.Name.Name),
		BridgePackage: fmt.Sprintf("bridge_%s", file.Name.Name),
	}

	typeAliases := map[string]string{}

	// first pass: find type aliases
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts := spec.(*ast.TypeSpec)
			if ident, ok := ts.Type.(*ast.Ident); ok {
				typeAliases[ts.Name.Name] = ident.Name
			}
		}
	}

	// second pass: vars
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}

		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)

			for i, name := range vs.Names {
				var typ ast.Expr
				if vs.Type != nil {
					typ = vs.Type
				} else if len(vs.Values) > i {
					typ = inferType(vs.Values[i])
				}

				if typ == nil {
					continue
				}

				ir, ok := typeToIR(name.Name, typ, typeAliases)
				if ok {
					pkg.Vars = append(pkg.Vars, ir)
				}
			}
		}
	}

	return pkg, nil
}

func inferType(expr ast.Expr) ast.Expr {
	switch v := expr.(type) {
	case *ast.BasicLit:
		return &ast.Ident{Name: v.Kind.String()}
	default:
		return nil
	}
}
