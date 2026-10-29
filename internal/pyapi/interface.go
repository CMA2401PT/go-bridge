package pyapi

import "go/types"

// planInterfaces keeps static Go assignment separate from dynamic assertions.
// Builtin Python values must be explicitly boxed to choose their Go type.
func planInterfaces(p *Package) {
	for _, target := range p.Types {
		if target.Shape != InterfaceObject {
			continue
		}
		for _, source := range p.Types {
			if source.Shape != Builtin && types.AssignableTo(source.Go.Go, target.Go.Go) {
				target.Implementors = append(target.Implementors, source)
			}
		}
	}
}
