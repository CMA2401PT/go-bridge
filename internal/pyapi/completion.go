package pyapi

import (
	"go/types"

	"go-bridge/internal/gomodel"
)

// complete adds only policy-required bridge types. Struct wrappers represent
// addressable variables, so addr needs *T. Other ptr methods are exposed only
// when their result type is already present; pointer chains stay finite.
func complete(g *gomodel.Package) error {
	for i := 0; i < len(g.Types); i++ {
		t := g.Types[i]
		if t.Kind != gomodel.Named {
			continue
		}
		if _, ok := t.Go.Underlying().(*types.Struct); !ok {
			continue
		}
		if _, err := g.Ensure(types.NewPointer(t.Go), "bridge addr of "+t.String()); err != nil {
			return err
		}
	}
	return nil
}
