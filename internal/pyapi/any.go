package pyapi

// AnyConversion names the explicit boxing and assertion API of one concrete
// type. Builtins use module functions; other types use class members.
type AnyConversion struct {
	Type                   *Type
	Box, Assert, TryAssert string
}

// planAny enables GoAny only when the closure contains the empty interface,
// then plans conversions for every other mapped type.
func planAny(p *Package, names namespace) error {
	for _, t := range p.Types {
		if t.Shape == AnyObject {
			p.Any = t
			break
		}
	}
	if p.Any == nil {
		return nil
	}
	for _, t := range p.Types {
		if t == p.Any {
			continue
		}
		c := &AnyConversion{Type: t, Box: "as_any", Assert: "from_any", TryAssert: "try_from_any"}
		if t.Shape == Builtin {
			stem := t.Go.Name
			c.Box, c.Assert, c.TryAssert = stem+"_as_any", "any_as_"+stem, "try_any_as_"+stem
			for _, name := range []string{c.Box, c.Assert, c.TryAssert} {
				if err := names.claim("name", name, "any conversion for "+t.Go.String()); err != nil {
					return err
				}
			}
		}
		p.AnyConversions = append(p.AnyConversions, c)
	}
	return nil
}
