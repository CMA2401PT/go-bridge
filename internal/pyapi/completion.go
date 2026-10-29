package pyapi

import (
	"go/types"

	"go-bridge/internal/gomodel"
)

// complete adds only policy-required bridge types. Struct and array wrappers represent
// addressable variables, so addr needs *T. Other ptr methods are exposed only
// when their result type is already present; pointer chains stay finite.
func complete(g *gomodel.Package) error {
	for i := 0; i < len(g.Types); i++ {
		t := g.Types[i]
		if _, ok := t.Go.Underlying().(*types.Interface); ok {
			if _, err := g.Ensure(types.NewInterfaceType(nil, nil).Complete(), "bridge interface conversions"); err != nil {
				return err
			}
		}
		if t.Kind == gomodel.Channel && t.Direction == types.SendRecv {
			for _, direction := range []types.ChanDir{types.SendOnly, types.RecvOnly} {
				if _, err := g.Ensure(types.NewChan(direction, t.Elem.Go), "bridge channel direction of "+t.String()); err != nil {
					return err
				}
			}
		}
		switch t.Go.Underlying().(type) {
		case *types.Struct, *types.Array:
		default:
			continue
		}
		if _, err := g.Ensure(types.NewPointer(t.Go), "bridge addr of "+t.String()); err != nil {
			return err
		}
	}
	return nil
}
