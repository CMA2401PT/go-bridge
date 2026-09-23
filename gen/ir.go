package gen

import "go/ast"

type GoTypeKind int

const (
	TypeString GoTypeKind = iota
	TypeInt
	TypeUint
	TypeFloat64
	TypeAliasInt // e.g. Kind
)

type VarIR struct {
	GoName    string // V1
	GoType    string // string / int / Kind
	TypeKind  GoTypeKind
	AliasBase string // only for alias, e.g. "int"
}

type PackageIR struct {
	TargetPackage string // vars
	TargetImport  string // "path/to/vars"
	BridgePackage string // vars_bridge
	Vars          []VarIR
}

func typeToIR(name string, typ ast.Expr, aliases map[string]string) (VarIR, bool) {
	id, ok := typ.(*ast.Ident)
	if !ok {
		return VarIR{}, false
	}

	switch id.Name {
	case "string":
		return VarIR{name, "string", TypeString, ""}, true
	case "int":
		return VarIR{name, "int", TypeInt, ""}, true
	case "uint":
		return VarIR{name, "uint", TypeUint, ""}, true
	case "float64":
		return VarIR{name, "float64", TypeFloat64, ""}, true
	default:
		if base, ok := aliases[id.Name]; ok && base == "int" {
			return VarIR{name, id.Name, TypeAliasInt, base}, true
		}
	}

	return VarIR{}, false
}
